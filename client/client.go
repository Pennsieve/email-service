package client

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

// SQSAPI is the subset of the SQS client this package uses. *sqs.Client
// satisfies it; tests provide a fake.
type SQSAPI interface {
	SendMessage(ctx context.Context, params *sqs.SendMessageInput, optFns ...func(*sqs.Options)) (*sqs.SendMessageOutput, error)
}

// Client enqueues email requests on the email-service SQS queue.
type Client struct {
	sqs      SQSAPI
	queueURL string
}

// New constructs a Client. queueURL is the URL of the email-service send queue
// for the target environment (e.g. from the queue's Terraform output).
func New(sqsClient SQSAPI, queueURL string) *Client {
	return &Client{sqs: sqsClient, queueURL: queueURL}
}

// Send validates and enqueues an email request. The email-service consumer then
// renders the template and delivers via SES. Send returns once the message is
// on the queue — delivery is asynchronous.
func (c *Client) Send(ctx context.Context, req EmailRequest) error {
	if err := req.validate(); err != nil {
		return err
	}
	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("error marshalling email request %q: %w", req.MessageId, err)
	}
	_, err = c.sqs.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:    aws.String(c.queueURL),
		MessageBody: aws.String(string(body)),
	})
	if err != nil {
		return fmt.Errorf("error enqueuing email request %q: %w", req.MessageId, err)
	}
	return nil
}

// validate checks the request is well-formed before it hits the queue, so
// producers get an immediate error instead of a silently-dropped message.
func (r EmailRequest) validate() error {
	if r.MessageId == "" {
		return fmt.Errorf("email request is missing messageId")
	}
	if len(r.Recipients) == 0 {
		return fmt.Errorf("email request %q has no recipients", r.MessageId)
	}
	for i, rcpt := range r.Recipients {
		if rcpt.Email == "" {
			return fmt.Errorf("email request %q recipient %d has no email address", r.MessageId, i)
		}
	}
	return nil
}
