// Package ratelimit is the send-rate safeguard. It caps how many emails the
// service will hand to SES in a rolling window, protecting the account's SES
// reputation/quota from a producer stuck in a loop. Over-limit sends are made
// log-only by the caller (not delivered), never reaching SES.
//
// It is a fixed-window counter backed by a DynamoDB table: each window has a
// row keyed by a bucket string; a send does an atomic ADD and compares the
// post-increment count to the limit. Rows carry a TTL so old windows expire.
package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// Limiter reports whether a send under a named counter is within its limit.
type Limiter interface {
	// Allow atomically counts one send against key for the current window and
	// returns whether the count is still within limit. A non-nil error means
	// the check could not be performed — the caller decides fail-open/closed
	// (this service fails closed: treat an error as not-allowed).
	Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error)
}

// DynamoLimiter is a Limiter backed by a DynamoDB counter table.
type DynamoLimiter struct {
	client *dynamodb.Client
	table  string
	now    func() time.Time // seam for tests
}

func NewDynamoLimiter(client *dynamodb.Client, table string) *DynamoLimiter {
	return &DynamoLimiter{client: client, table: table, now: func() time.Time { return time.Now().UTC() }}
}

// Allow increments the counter for key's current window and returns count <= limit.
// A limit <= 0 disables the check (always allowed).
func (l *DynamoLimiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
	if limit <= 0 {
		return true, nil
	}
	now := l.now()
	// Fixed-window bucket: the same key+window collapses to one counter row that
	// resets each window boundary.
	bucket := now.Truncate(window).Unix()
	pk := fmt.Sprintf("%s#%d", key, bucket)
	// Expire the row one window after it ends, so stale counters don't accrue.
	expiresAt := now.Add(2 * window).Unix()

	out, err := l.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:        aws.String(l.table),
		Key:              map[string]types.AttributeValue{"Bucket": &types.AttributeValueMemberS{Value: pk}},
		UpdateExpression: aws.String("ADD #c :one SET ExpiresAt = if_not_exists(ExpiresAt, :exp)"),
		ExpressionAttributeNames: map[string]string{
			"#c": "Count",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":one": &types.AttributeValueMemberN{Value: "1"},
			":exp": &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", expiresAt)},
		},
		ReturnValues: types.ReturnValueUpdatedNew,
	})
	if err != nil {
		return false, fmt.Errorf("error updating rate counter %q: %w", key, err)
	}

	countAttr, ok := out.Attributes["Count"].(*types.AttributeValueMemberN)
	if !ok {
		return false, fmt.Errorf("rate counter %q returned no Count", key)
	}
	var count int
	if _, err := fmt.Sscanf(countAttr.Value, "%d", &count); err != nil {
		return false, fmt.Errorf("rate counter %q count %q not an int: %w", key, countAttr.Value, err)
	}
	return count <= limit, nil
}
