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

// SESMailer sends email through Amazon SES, mirroring the conventions used by
// the rehydration-service emailer (HTML body, UTF-8, support@{domain} sender).
type SESMailer struct {
	client *ses.Client
	sender string
}

// NewSESMailer constructs a mailer that sends from support@{pennsieveDomain}.
func NewSESMailer(client *ses.Client, pennsieveDomain string) *SESMailer {
	return &SESMailer{
		client: client,
		sender: fmt.Sprintf("support@%s", pennsieveDomain),
	}
}

func (m *SESMailer) Send(ctx context.Context, email Email) (string, error) {
	out, err := m.client.SendEmail(ctx, &ses.SendEmailInput{
		Source: aws.String(m.sender),
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
