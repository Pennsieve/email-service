package handler

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	emailconfig "github.com/pennsieve/email-service/internal/config"
	"github.com/pennsieve/email-service/internal/journal"
	"github.com/pennsieve/email-service/internal/mailer"
	"github.com/pennsieve/email-service/internal/templates"
)

// --- mocks ---------------------------------------------------------------

type mockTemplateStore struct {
	mapping *templates.Mapping
	err     error
}

func (m *mockTemplateStore) GetMapping(_ context.Context, _ string) (*templates.Mapping, error) {
	return m.mapping, m.err
}

type mockBodyStore struct {
	body []byte
	err  error
	// captured arguments from the last call
	orgId  int64
	hasOrg bool
	file   string
}

func (m *mockBodyStore) FetchTemplate(_ context.Context, orgId int64, hasOrg bool, file string) ([]byte, error) {
	m.orgId, m.hasOrg, m.file = orgId, hasOrg, file
	return m.body, m.err
}

type mockMailer struct {
	sent []mailer.Email
	err  error
}

func (m *mockMailer) Send(_ context.Context, email mailer.Email) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	m.sent = append(m.sent, email)
	return "ses-msg-id", nil
}

// mockJournal records claim/mark calls. claimErr (if set) is returned by Claim
// to simulate either a duplicate (journal.ErrAlreadyClaimed) or an error.
type mockJournal struct {
	claimed  []journal.Entry
	sent     []string
	failed   []string
	claimErr error
}

func (m *mockJournal) Claim(_ context.Context, entry journal.Entry) error {
	if m.claimErr != nil {
		return m.claimErr
	}
	m.claimed = append(m.claimed, entry)
	return nil
}

func (m *mockJournal) MarkSent(_ context.Context, id, _ /*ses*/, _ /*sent*/ string) error {
	m.sent = append(m.sent, id)
	return nil
}

func (m *mockJournal) MarkFailed(_ context.Context, id, _ string) error {
	m.failed = append(m.failed, id)
	return nil
}

// statefulJournal models the real DynamoDB Claim condition: a claim succeeds
// when there is no row OR the existing row is FAILED; a QUEUED/SENT row blocks
// it with ErrAlreadyClaimed. Used to exercise the redelivery/retry contract.
type statefulJournal struct {
	status map[string]journal.Status
	claims int
	sends  int
}

func newStatefulJournal() *statefulJournal {
	return &statefulJournal{status: map[string]journal.Status{}}
}

func (m *statefulJournal) Claim(_ context.Context, entry journal.Entry) error {
	if s, ok := m.status[entry.Id]; ok && s != journal.StatusFailed {
		return journal.ErrAlreadyClaimed
	}
	m.status[entry.Id] = journal.StatusQueued
	m.claims++
	return nil
}

func (m *statefulJournal) MarkSent(_ context.Context, id, _, _ string) error {
	m.status[id] = journal.StatusSent
	m.sends++
	return nil
}

func (m *statefulJournal) MarkFailed(_ context.Context, id, _ string) error {
	m.status[id] = journal.StatusFailed
	return nil
}

func newTestConfig(ts templates.Store, bs *mockBodyStore, ml mailer.Mailer, jr journal.Journal) *emailconfig.Config {
	cfg := emailconfig.NewConfig(aws.Config{}, emailconfig.Env{JournalTTLDays: 90})
	cfg.SetTemplateStore(ts)
	cfg.SetBodyStore(bs)
	cfg.SetMailer(ml)
	cfg.SetJournal(jr)
	return cfg
}

func sqsEvent(bodies ...string) events.SQSEvent {
	var records []events.SQSMessage
	for i, b := range bodies {
		records = append(records, events.SQSMessage{MessageId: string(rune('A' + i)), Body: b})
	}
	return events.SQSEvent{Records: records}
}

func init() {
	// Deterministic timestamp for assertions.
	now = func() time.Time { return time.Unix(1700000000, 0).UTC() }
}

// --- tests ---------------------------------------------------------------

func TestEmptyBatch(t *testing.T) {
	resp := ProcessEvent(context.Background(), newTestConfig(
		&mockTemplateStore{}, &mockBodyStore{}, &mockMailer{}, &mockJournal{}),
		events.SQSEvent{Records: nil})
	assert.Empty(t, resp.BatchItemFailures)
}

func TestSendToMultipleRecipients(t *testing.T) {
	ts := &mockTemplateStore{mapping: &templates.Mapping{MessageId: "welcome", TemplateFile: "welcome.html", Subject: "Welcome"}}
	bs := &mockBodyStore{body: []byte("Hi {{.userName}} at {{.organiztionName}}")}
	ml := &mockMailer{}
	jr := &mockJournal{}

	body := `{
		"messageId": "welcome",
		"recipients": [{"name":"A","email":"a@example.com"},{"name":"B","email":"b@example.com"}],
		"context": {"organizationId": 367, "userName": "Alice", "organiztionName": "SPARC"}
	}`

	resp := ProcessEvent(context.Background(), newTestConfig(ts, bs, ml, jr), sqsEvent(body))

	assert.Empty(t, resp.BatchItemFailures, "no failures expected")
	require.Len(t, ml.sent, 2)
	// branding: org present, so custom lookup is attempted with org 367
	assert.Equal(t, int64(367), bs.orgId)
	assert.True(t, bs.hasOrg)
	assert.Equal(t, "welcome.html", bs.file)
	// rendered + escaped body
	assert.Equal(t, "Hi Alice at SPARC", ml.sent[0].HTMLBody)
	assert.Equal(t, "Welcome", ml.sent[0].Subject)
	// claim + mark-sent per recipient, with TTL set 90 days out
	require.Len(t, jr.claimed, 2)
	require.Len(t, jr.sent, 2)
	assert.Empty(t, jr.failed)
	assert.Equal(t, int64(1700000000), jr.claimed[0].Timestamp)
	assert.Equal(t, journal.SentAtKey(time.Unix(1700000000, 0).UTC()), jr.claimed[0].SentAtKey)
	assert.Equal(t, time.Unix(1700000000, 0).UTC().AddDate(0, 0, 90).Unix(), jr.claimed[0].ExpiresAt)
	// distinct dedupe keys per recipient
	assert.NotEqual(t, jr.claimed[0].Id, jr.claimed[1].Id)
}

func TestDuplicateIsSkipped(t *testing.T) {
	ts := &mockTemplateStore{mapping: &templates.Mapping{TemplateFile: "f.html"}}
	bs := &mockBodyStore{body: []byte("body")}
	ml := &mockMailer{}
	jr := &mockJournal{claimErr: journal.ErrAlreadyClaimed}
	body := `{"messageId":"m","recipients":[{"email":"a@x.com"}],"context":{}}`

	resp := ProcessEvent(context.Background(), newTestConfig(ts, bs, ml, jr), sqsEvent(body))

	assert.Empty(t, resp.BatchItemFailures, "duplicate is success, not failure")
	assert.Empty(t, ml.sent, "must not send on duplicate")
	assert.Empty(t, jr.sent)
}

func TestSubjectOverride(t *testing.T) {
	ts := &mockTemplateStore{mapping: &templates.Mapping{TemplateFile: "f.html", Subject: "Default"}}
	bs := &mockBodyStore{body: []byte("body")}
	ml := &mockMailer{}
	body := `{"messageId":"m","recipients":[{"email":"a@x.com"}],"context":{"subject":"Custom"}}`

	ProcessEvent(context.Background(), newTestConfig(ts, bs, ml, &mockJournal{}), sqsEvent(body))

	require.Len(t, ml.sent, 1)
	assert.Equal(t, "Custom", ml.sent[0].Subject)
}

func TestDefaultSubjectIsInterpolated(t *testing.T) {
	// A {{.var}} in the table's default subject is rendered against context.
	ts := &mockTemplateStore{mapping: &templates.Mapping{TemplateFile: "f.html", Subject: "Proposal submitted to {{.WorkspaceName}}"}}
	bs := &mockBodyStore{body: []byte("body")}
	ml := &mockMailer{}
	body := `{"messageId":"m","recipients":[{"email":"a@x.com"}],"context":{"WorkspaceName":"SPARC"}}`

	ProcessEvent(context.Background(), newTestConfig(ts, bs, ml, &mockJournal{}), sqsEvent(body))

	require.Len(t, ml.sent, 1)
	assert.Equal(t, "Proposal submitted to SPARC", ml.sent[0].Subject)
}

func TestNoOrgUsesDefaultOnly(t *testing.T) {
	ts := &mockTemplateStore{mapping: &templates.Mapping{TemplateFile: "f.html"}}
	bs := &mockBodyStore{body: []byte("body")}
	body := `{"messageId":"m","recipients":[{"email":"a@x.com"}],"context":{}}`

	ProcessEvent(context.Background(), newTestConfig(ts, bs, &mockMailer{}, &mockJournal{}), sqsEvent(body))

	assert.False(t, bs.hasOrg, "no organizationId in context")
	assert.Equal(t, int64(0), bs.orgId)
}

func TestMalformedBodyIsBatchFailure(t *testing.T) {
	resp := ProcessEvent(context.Background(), newTestConfig(
		&mockTemplateStore{}, &mockBodyStore{}, &mockMailer{}, &mockJournal{}),
		sqsEvent("not json"))
	require.Len(t, resp.BatchItemFailures, 1)
	assert.Equal(t, "A", resp.BatchItemFailures[0].ItemIdentifier)
}

func TestPartialBatchFailure(t *testing.T) {
	ts := &mockTemplateStore{mapping: &templates.Mapping{TemplateFile: "f.html", Subject: "S"}}
	bs := &mockBodyStore{body: []byte("body")}
	good := `{"messageId":"m","recipients":[{"email":"a@x.com"}],"context":{}}`
	bad := `{"messageId":"m","recipients":[],"context":{}}` // no recipients -> error

	resp := ProcessEvent(context.Background(), newTestConfig(ts, bs, &mockMailer{}, &mockJournal{}),
		sqsEvent(good, bad))

	require.Len(t, resp.BatchItemFailures, 1)
	assert.Equal(t, "B", resp.BatchItemFailures[0].ItemIdentifier, "only the bad record fails")
}

func TestMailerErrorMarksFailedAndIsBatchFailure(t *testing.T) {
	ts := &mockTemplateStore{mapping: &templates.Mapping{TemplateFile: "f.html"}}
	bs := &mockBodyStore{body: []byte("body")}
	ml := &mockMailer{err: errors.New("ses down")}
	jr := &mockJournal{}
	body := `{"messageId":"m","recipients":[{"email":"a@x.com"}],"context":{}}`

	resp := ProcessEvent(context.Background(), newTestConfig(ts, bs, ml, jr), sqsEvent(body))

	require.Len(t, resp.BatchItemFailures, 1)
	require.Len(t, jr.claimed, 1, "claimed before send")
	require.Len(t, jr.failed, 1, "marked failed after send error")
	assert.Empty(t, jr.sent)
}

// flakyMailer fails for the first failTimes calls, then succeeds.
type flakyMailer struct {
	failTimes int
	calls     int
	sent      []mailer.Email
}

func (m *flakyMailer) Send(_ context.Context, email mailer.Email) (string, error) {
	m.calls++
	if m.calls <= m.failTimes {
		return "", errors.New("transient ses error")
	}
	m.sent = append(m.sent, email)
	return "ses-id", nil
}

// A transiently-failed send must be retried on SQS redelivery (the journal
// claim accepts a FAILED row), and must end up delivered exactly once.
func TestTransientFailureRetriesOnRedelivery(t *testing.T) {
	ts := &mockTemplateStore{mapping: &templates.Mapping{TemplateFile: "f.html", Subject: "S"}}
	bs := &mockBodyStore{body: []byte("body")}
	ml := &flakyMailer{failTimes: 1} // first delivery fails, second succeeds
	jr := newStatefulJournal()
	body := `{"messageId":"m","recipients":[{"email":"a@x.com"}],"context":{}}`
	cfg := newTestConfig(ts, bs, ml, jr)

	// First delivery: send fails -> batch failure, row left FAILED.
	resp1 := ProcessEvent(context.Background(), cfg, sqsEvent(body))
	require.Len(t, resp1.BatchItemFailures, 1, "first delivery fails")
	assert.Empty(t, ml.sent)
	assert.Equal(t, 0, jr.sends, "nothing marked sent yet")

	// Redelivery: claim re-succeeds on the FAILED row, send now succeeds.
	resp2 := ProcessEvent(context.Background(), cfg, sqsEvent(body))
	assert.Empty(t, resp2.BatchItemFailures, "redelivery succeeds")
	require.Len(t, ml.sent, 1, "delivered exactly once")
	assert.Equal(t, 1, jr.sends)
	assert.Equal(t, 2, jr.claims, "claimed on both deliveries (FAILED row allows re-claim)")
}
