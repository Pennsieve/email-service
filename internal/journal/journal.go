package journal

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// Status is the delivery state of a single (message, recipient) send.
type Status string

const (
	// StatusQueued is written when the send is claimed, before handing off to
	// SES. Its presence is also the idempotency guard against redelivery.
	StatusQueued Status = "QUEUED"
	// StatusSent is written after SES accepts the message.
	StatusSent Status = "SENT"
	// StatusFailed is written when sending fails. The Error field carries detail.
	StatusFailed Status = "FAILED"
)

// ErrAlreadyClaimed is returned by Claim when an entry for the same dedupe key
// already exists, i.e. this (message, recipient) was already processed. The
// caller should treat the send as a duplicate and skip it.
var ErrAlreadyClaimed = errors.New("entry already claimed")

// Entry is a row in the email-message-log table. It is both the audit record
// of a sent email and the operational journal used to answer "I never got the
// email": one row per (message, recipient), carrying delivery Status, the SES
// MessageId, any error, and a TTL (ExpiresAt) for configurable expiration.
//
// Id is the dedupe key (see client.EmailRequest.DedupeKey): using it as the
// partition key lets Claim use a conditional PutItem as the dedupe guard.
type Entry struct {
	Id           string         `dynamodbav:"Id"`
	MessageId    string         `dynamodbav:"MessageId"`
	Recipient    string         `dynamodbav:"Recipient"`
	Status       Status         `dynamodbav:"Status"`
	Timestamp    int64          `dynamodbav:"Timestamp"`
	SentAtKey    string         `dynamodbav:"SentAtKey"`
	MessageSent  string         `dynamodbav:"MessageSent,omitempty"`
	SesMessageId string         `dynamodbav:"SesMessageId,omitempty"`
	Error        string         `dynamodbav:"Error,omitempty"`
	Context      map[string]any `dynamodbav:"Context,omitempty"`
	ExpiresAt    int64          `dynamodbav:"ExpiresAt"`
}

// SentAtKey returns the GSI (RecipientSentAtIndex) sort key for a timestamp: the
// Unix epoch seconds, zero-padded to a fixed width so lexicographic string
// ordering matches chronological ordering. Querying the index with
// ScanIndexForward=false then returns a recipient's most recent emails first.
// Width 20 covers int64 epoch values comfortably (max ~9.2e18 is 19 digits).
func SentAtKey(t time.Time) string {
	return fmt.Sprintf("%020d", t.Unix())
}

// Journal records and updates email send entries.
type Journal interface {
	// Claim atomically records a QUEUED entry. It returns ErrAlreadyClaimed if
	// an entry with the same Id already exists (the dedupe guard).
	Claim(ctx context.Context, entry Entry) error
	// MarkSent updates the entry to SENT with the SES message id.
	MarkSent(ctx context.Context, id string, sesMessageId, messageSent string) error
	// MarkFailed updates the entry to FAILED with the error detail.
	MarkFailed(ctx context.Context, id string, sendErr string) error
}

// DynamoJournal is a Journal backed by the email-message-log DynamoDB table.
type DynamoJournal struct {
	client *dynamodb.Client
	table  string
}

func NewDynamoJournal(client *dynamodb.Client, table string) *DynamoJournal {
	return &DynamoJournal{client: client, table: table}
}

func (j *DynamoJournal) Claim(ctx context.Context, entry Entry) error {
	item, err := attributevalue.MarshalMap(entry)
	if err != nil {
		return fmt.Errorf("error marshalling journal entry for messageId %s recipient %s: %w", entry.MessageId, entry.Recipient, err)
	}
	// Claim succeeds when there is no existing row OR the existing row is FAILED.
	// The FAILED case lets an SQS redelivery retry a send that failed
	// transiently (the row is overwritten back to QUEUED). A QUEUED or SENT row
	// blocks the claim (ErrAlreadyClaimed), so a successful send is never
	// repeated. The small double-send window — process dies after SES accepts
	// but before MarkSent leaves the row QUEUED, so redelivery is blocked, not
	// retried — favors at-most-once for the already-sent case while still
	// retrying genuine failures.
	_, err = j.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName:           aws.String(j.table),
		Item:                item,
		ConditionExpression: aws.String("attribute_not_exists(Id) OR #s = :failed"),
		ExpressionAttributeNames: map[string]string{
			"#s": "Status",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":failed": &types.AttributeValueMemberS{Value: string(StatusFailed)},
		},
	})
	if err != nil {
		var conditionFailed *types.ConditionalCheckFailedException
		if errors.As(err, &conditionFailed) {
			return fmt.Errorf("messageId %s recipient %s: %w", entry.MessageId, entry.Recipient, ErrAlreadyClaimed)
		}
		return fmt.Errorf("error claiming journal entry for messageId %s recipient %s: %w", entry.MessageId, entry.Recipient, err)
	}
	return nil
}

func (j *DynamoJournal) MarkSent(ctx context.Context, id, sesMessageId, messageSent string) error {
	_, err := j.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:        aws.String(j.table),
		Key:              key(id),
		UpdateExpression: aws.String("SET #s = :status, SesMessageId = :ses, MessageSent = :sent"),
		ExpressionAttributeNames: map[string]string{
			"#s": "Status",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":status": &types.AttributeValueMemberS{Value: string(StatusSent)},
			":ses":    &types.AttributeValueMemberS{Value: sesMessageId},
			":sent":   &types.AttributeValueMemberS{Value: messageSent},
		},
	})
	if err != nil {
		return fmt.Errorf("error marking journal entry %s sent: %w", id, err)
	}
	return nil
}

func (j *DynamoJournal) MarkFailed(ctx context.Context, id, sendErr string) error {
	_, err := j.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:        aws.String(j.table),
		Key:              key(id),
		UpdateExpression: aws.String("SET #s = :status, #e = :err"),
		ExpressionAttributeNames: map[string]string{
			"#s": "Status",
			"#e": "Error",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":status": &types.AttributeValueMemberS{Value: string(StatusFailed)},
			":err":    &types.AttributeValueMemberS{Value: sendErr},
		},
	})
	if err != nil {
		return fmt.Errorf("error marking journal entry %s failed: %w", id, err)
	}
	return nil
}

func key(id string) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{
		"Id": &types.AttributeValueMemberS{Value: id},
	}
}
