package mailer

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ses"
	sestypes "github.com/aws/aws-sdk-go-v2/service/ses/types"
)

const charSet = "UTF-8"

// Email is a single rendered message ready to send.
type Email struct {
	Recipient string
	Subject   string
	HTMLBody  string
}

// Mailer sends a rendered HTML email and returns the provider message id.
type Mailer interface {
	Send(ctx context.Context, email Email) (messageId string, err error)
}

// SESAPI is the subset of the SES client this package uses. *ses.Client
// satisfies it; tests provide a fake.
type SESAPI interface {
	SendEmail(ctx context.Context, params *ses.SendEmailInput, optFns ...func(*ses.Options)) (*ses.SendEmailOutput, error)
}

// senderDisplayName is the friendly name shown on the From address, e.g.
// "Pennsieve <support@pennsieve.io>".
const senderDisplayName = "Pennsieve"

// SESMailer sends email through Amazon SES. It sends from
// "Pennsieve <support@{domain}>" and sets Reply-To to support@{domain}.
type SESMailer struct {
	client SESAPI
	// sender is the bare address (support@{domain}) — used for Reply-To and in
	// error messages. from is the display-name form used as the SES Source.
	sender string
	from   string
}

// NewSESMailer constructs a mailer that sends from
// "Pennsieve <support@{pennsieveDomain}>" with Reply-To support@{pennsieveDomain}.
func NewSESMailer(client SESAPI, pennsieveDomain string) *SESMailer {
	sender := fmt.Sprintf("support@%s", pennsieveDomain)
	return &SESMailer{
		client: client,
		sender: sender,
		from:   fmt.Sprintf("%s <%s>", senderDisplayName, sender),
	}
}

func (m *SESMailer) Send(ctx context.Context, email Email) (string, error) {
	out, err := m.client.SendEmail(ctx, &ses.SendEmailInput{
		Source:           aws.String(m.from),
		ReplyToAddresses: []string{m.sender},
		Destination: &sestypes.Destination{
			ToAddresses: []string{email.Recipient},
		},
		Message: &sestypes.Message{
			Subject: &sestypes.Content{
				Data:    aws.String(email.Subject),
				Charset: aws.String(charSet),
			},
			Body: &sestypes.Body{
				Html: &sestypes.Content{
					Data:    aws.String(email.HTMLBody),
					Charset: aws.String(charSet),
				},
			},
		},
	})
	if err != nil {
		return "", fmt.Errorf("error sending email from %s to %s: %w", m.sender, email.Recipient, err)
	}
	return aws.ToString(out.MessageId), nil
}
