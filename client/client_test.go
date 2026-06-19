package client

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeSQS struct {
	gotQueueURL string
	gotBody     string
	err         error
	calls       int
}

func (f *fakeSQS) SendMessage(_ context.Context, in *sqs.SendMessageInput, _ ...func(*sqs.Options)) (*sqs.SendMessageOutput, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	f.gotQueueURL = aws.ToString(in.QueueUrl)
	f.gotBody = aws.ToString(in.MessageBody)
	return &sqs.SendMessageOutput{MessageId: aws.String("sqs-id")}, nil
}

func TestSendMarshalsAndEnqueues(t *testing.T) {
	fake := &fakeSQS{}
	c := New(fake, "https://sqs/queue")

	req := DatasetPublicationAccepted(
		To{Name: "Alice", Email: "alice@example.com"},
		DatasetPublicationAcceptedArgs{DatasetName: "My Dataset", ReviewerName: "Bob", Date: "2026-06-18"},
	).WithOrganization(367)

	require.NoError(t, c.Send(context.Background(), req))
	assert.Equal(t, "https://sqs/queue", fake.gotQueueURL)

	// Body is the JSON wire contract.
	var got EmailRequest
	require.NoError(t, json.Unmarshal([]byte(fake.gotBody), &got))
	assert.Equal(t, "dataset-publication-accepted", got.MessageId)
	require.Len(t, got.Recipients, 1)
	assert.Equal(t, "alice@example.com", got.Recipients[0].Email)
	assert.Equal(t, "My Dataset", got.Context["datasetName"])
	assert.Equal(t, "Bob", got.Context["reviewerName"])
	// org id round-trips (as a JSON number -> float64).
	id, ok := got.OrganizationId()
	assert.True(t, ok)
	assert.Equal(t, int64(367), id)
}

func TestSendValidates(t *testing.T) {
	fake := &fakeSQS{}
	c := New(fake, "q")

	// missing messageId
	err := c.Send(context.Background(), EmailRequest{Recipients: []Recipient{{Email: "a@x.com"}}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "messageId")

	// no recipients
	err = c.Send(context.Background(), EmailRequest{MessageId: "m"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "recipients")

	// recipient with no email
	err = c.Send(context.Background(), EmailRequest{MessageId: "m", Recipients: []Recipient{{Name: "x"}}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "email address")

	assert.Equal(t, 0, fake.calls, "validation fails before hitting SQS")
}

func TestBuildersSetMessageIdAndKeys(t *testing.T) {
	r := RehydrationComplete(To{Email: "a@x.com"}, RehydrationCompleteArgs{
		DatasetID: "5", DatasetVersionID: "2", RehydrationLocation: "s3://b/k", AWSRegion: "us-east-1",
	})
	assert.Equal(t, "rehydration-complete", r.MessageId)
	assert.Equal(t, "5", r.Context["DatasetID"])
	assert.Equal(t, "us-east-1", r.Context["AWSRegion"])

	m := Message("custom-thing", To{Email: "a@x.com"}, nil)
	assert.Equal(t, "custom-thing", m.MessageId)
	assert.NotNil(t, m.Context, "nil context is normalized to empty map")
}

func TestWithDedupeIdAndSubject(t *testing.T) {
	r := AddedToTeam(To{Email: "a@x.com"}, AddedToTeamArgs{TeamName: "T"}).
		WithDedupeId("evt-123").
		WithSubject("Custom subject")
	assert.Equal(t, "evt-123", r.DedupeId)
	assert.Equal(t, "evt-123:a@x.com", r.DedupeKey("a@x.com"))
	assert.Equal(t, "Custom subject", r.Subject("default"))
}
