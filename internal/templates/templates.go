package templates

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// ErrMappingNotFound is returned by Store.GetMapping when no row exists for the
// given messageId. This is a caller error (an unknown messageId) rather than an
// infrastructure failure, so callers can distinguish it from a DynamoDB error.
var ErrMappingNotFound = errors.New("template mapping not found")

// Mapping is a row in the email-message-templates table. It maps a slug-style
// MessageId to the name of a template file in S3 and the default subject line
// for that email.
type Mapping struct {
	MessageId    string `dynamodbav:"MessageId"`
	TemplateFile string `dynamodbav:"TemplateFile"`
	Subject      string `dynamodbav:"Subject"`
	// SendDisabled, when true, suppresses actual delivery of this message type:
	// requests are journaled (LoggedOnly) but not sent via SES. Absent (false)
	// means sending is enabled, so existing rows need no change.
	SendDisabled bool `dynamodbav:"SendDisabled,omitempty"`
}

// Store looks up the template mapping for a messageId.
type Store interface {
	GetMapping(ctx context.Context, messageId string) (*Mapping, error)
}

// DynamoStore is a Store backed by the email-message-templates DynamoDB table.
type DynamoStore struct {
	client *dynamodb.Client
	table  string
}

func NewDynamoStore(client *dynamodb.Client, table string) *DynamoStore {
	return &DynamoStore{client: client, table: table}
}

// GetMapping fetches the mapping for messageId. It returns ErrMappingNotFound
// (wrapped) when no row exists.
func (s *DynamoStore) GetMapping(ctx context.Context, messageId string) (*Mapping, error) {
	out, err := s.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(s.table),
		Key: map[string]types.AttributeValue{
			"MessageId": &types.AttributeValueMemberS{Value: messageId},
		},
		ConsistentRead: aws.Bool(true),
	})
	if err != nil {
		return nil, fmt.Errorf("error getting template mapping for messageId %s: %w", messageId, err)
	}
	if len(out.Item) == 0 {
		return nil, fmt.Errorf("messageId %s: %w", messageId, ErrMappingNotFound)
	}

	var mapping Mapping
	if err := attributevalue.UnmarshalMap(out.Item, &mapping); err != nil {
		return nil, fmt.Errorf("error unmarshalling template mapping for messageId %s: %w", messageId, err)
	}
	return &mapping, nil
}
