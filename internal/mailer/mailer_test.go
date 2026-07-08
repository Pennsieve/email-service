package mailer

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ses"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeSES struct {
	in *ses.SendEmailInput
}

func (f *fakeSES) SendEmail(_ context.Context, in *ses.SendEmailInput, _ ...func(*ses.Options)) (*ses.SendEmailOutput, error) {
	f.in = in
	return &ses.SendEmailOutput{MessageId: aws.String("ses-msg-id")}, nil
}

func TestSendSetsFromDisplayNameAndReplyTo(t *testing.T) {
	fake := &fakeSES{}
	m := NewSESMailer(fake, "pennsieve.io")

	id, err := m.Send(context.Background(), Email{
		Recipient: "alice@example.com",
		Subject:   "Hi",
		HTMLBody:  "<p>hi</p>",
	})
	require.NoError(t, err)
	assert.Equal(t, "ses-msg-id", id)
	require.NotNil(t, fake.in)

	// From carries the display name; Reply-To is the bare support address.
	assert.Equal(t, "Pennsieve <support@pennsieve.io>", aws.ToString(fake.in.Source))
	assert.Equal(t, []string{"support@pennsieve.io"}, fake.in.ReplyToAddresses)
	assert.Equal(t, []string{"alice@example.com"}, fake.in.Destination.ToAddresses)
}
