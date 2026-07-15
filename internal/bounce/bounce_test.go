package bounce

import (
	"context"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/pennsieve/email-service/internal/suppression"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockStore struct {
	suppressed []suppression.Record
}

func (m *mockStore) IsSuppressed(_ context.Context, _ string) (bool, error) { return false, nil }
func (m *mockStore) Suppress(_ context.Context, r suppression.Record) error {
	m.suppressed = append(m.suppressed, r)
	return nil
}

func snsEvent(message string) events.SNSEvent {
	return events.SNSEvent{Records: []events.SNSEventRecord{
		{SNS: events.SNSEntity{Message: message}},
	}}
}

func handle(t *testing.T, message string) *mockStore {
	t.Helper()
	store := &mockStore{}
	h := NewHandler(store, func() string { return "2026-07-15T00:00:00Z" })
	require.NoError(t, h.Handle(context.Background(), snsEvent(message)))
	return store
}

func TestPermanentBounceSuppresses(t *testing.T) {
	store := handle(t, `{
		"notificationType": "Bounce",
		"bounce": { "bounceType": "Permanent",
			"bouncedRecipients": [{"emailAddress": "bad@example.com"}] }
	}`)
	require.Len(t, store.suppressed, 1)
	assert.Equal(t, "bad@example.com", store.suppressed[0].Email)
	assert.Equal(t, "bounce", store.suppressed[0].Reason)
	assert.Equal(t, "2026-07-15T00:00:00Z", store.suppressed[0].CreatedAt)
}

func TestTransientBounceDoesNotSuppress(t *testing.T) {
	store := handle(t, `{
		"notificationType": "Bounce",
		"bounce": { "bounceType": "Transient",
			"bouncedRecipients": [{"emailAddress": "mailbox-full@example.com"}] }
	}`)
	assert.Empty(t, store.suppressed, "transient bounces must not permanently suppress")
}

func TestComplaintSuppresses(t *testing.T) {
	store := handle(t, `{
		"notificationType": "Complaint",
		"complaint": { "complainedRecipients": [{"emailAddress": "angry@example.com"}] }
	}`)
	require.Len(t, store.suppressed, 1)
	assert.Equal(t, "angry@example.com", store.suppressed[0].Email)
	assert.Equal(t, "complaint", store.suppressed[0].Reason)
}

func TestMultipleRecipientsAllSuppressed(t *testing.T) {
	store := handle(t, `{
		"notificationType": "Bounce",
		"bounce": { "bounceType": "Permanent",
			"bouncedRecipients": [{"emailAddress": "a@x.com"}, {"emailAddress": "b@x.com"}] }
	}`)
	require.Len(t, store.suppressed, 2)
}

func TestUnparseableMessageIsSkipped(t *testing.T) {
	// Not JSON — should be logged and skipped, not error the whole event.
	store := handle(t, "this is not json")
	assert.Empty(t, store.suppressed)
}

func TestDeliveryNotificationIgnored(t *testing.T) {
	// A non-bounce/complaint notification (e.g. Delivery) suppresses nothing.
	store := handle(t, `{"notificationType": "Delivery"}`)
	assert.Empty(t, store.suppressed)
}
