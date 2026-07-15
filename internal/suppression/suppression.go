// Package suppression is the address-level send control: a list of email
// addresses that must not receive emails. A suppressed address is not delivered
// to — the request is still journaled (LoggedOnly). Backed by a DynamoDB table
// keyed by the email address, so it can be edited without a deploy.
package suppression

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// Record is a row in the suppression table: an address that must not be sent to,
// with why and when it was added.
type Record struct {
	Email     string `dynamodbav:"Email"`
	Reason    string `dynamodbav:"Reason,omitempty"`
	CreatedAt string `dynamodbav:"CreatedAt,omitempty"`
}

// Store reports whether an email address is suppressed, and can add addresses
// to the suppression list.
type Store interface {
	IsSuppressed(ctx context.Context, email string) (bool, error)
	// Suppress adds (or overwrites) a suppression record for an address.
	Suppress(ctx context.Context, record Record) error
}

// DynamoStore is a Store backed by the email-suppression DynamoDB table.
type DynamoStore struct {
	client *dynamodb.Client
	table  string
}

func NewDynamoStore(client *dynamodb.Client, table string) *DynamoStore {
	return &DynamoStore{client: client, table: table}
}

// IsSuppressed returns true when a suppression record exists for the address.
func (s *DynamoStore) IsSuppressed(ctx context.Context, email string) (bool, error) {
	out, err := s.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(s.table),
		Key: map[string]types.AttributeValue{
			"Email": &types.AttributeValueMemberS{Value: email},
		},
		ConsistentRead: aws.Bool(true),
	})
	if err != nil {
		return false, fmt.Errorf("error checking suppression for %s: %w", email, err)
	}
	return len(out.Item) > 0, nil
}

// Suppress writes a suppression record (PutItem, so re-suppressing an address
// refreshes its reason/timestamp rather than failing).
func (s *DynamoStore) Suppress(ctx context.Context, record Record) error {
	if record.Email == "" {
		return fmt.Errorf("cannot suppress an empty email address")
	}
	item, err := attributevalue.MarshalMap(record)
	if err != nil {
		return fmt.Errorf("error marshalling suppression record for %s: %w", record.Email, err)
	}
	_, err = s.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(s.table),
		Item:      item,
	})
	if err != nil {
		return fmt.Errorf("error writing suppression record for %s: %w", record.Email, err)
	}
	return nil
}
