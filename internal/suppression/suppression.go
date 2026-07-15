// Package suppression is the address-level send control: a list of email
// addresses that must not receive emails. A suppressed address is not delivered
// to — the request is still journaled (LoggedOnly). Backed by a DynamoDB table
// keyed by the email address, so it can be edited without a deploy.
package suppression

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
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

// Store reports whether an email address is suppressed.
type Store interface {
	IsSuppressed(ctx context.Context, email string) (bool, error)
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
