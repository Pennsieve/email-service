package notifications

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// SubscriptionStore manages the subscriptions table: which topics a user has
// opted into.
type SubscriptionStore interface {
	CreateSubscription(ctx context.Context, sub Subscription) (Subscription, error)
	GetSubscription(ctx context.Context, subscriptionID int64) (Subscription, error)
	ListSubscriptionsByUser(ctx context.Context, userID int64) ([]Subscription, error)
	ListSubscriptionsByTopic(ctx context.Context, topicID int64) ([]Subscription, error)
	DeleteSubscription(ctx context.Context, subscriptionID int64) error
}

func (s *PostgresStore) CreateSubscription(ctx context.Context, sub Subscription) (Subscription, error) {
	contextArg, err := jsonArg(sub.Context)
	if err != nil {
		return Subscription{}, err
	}
	row := s.pool.QueryRow(ctx,
		`INSERT INTO subscriptions (user_id, topic_id, context) VALUES ($1, $2, $3::jsonb)
		 RETURNING subscription_id, user_id, topic_id, context, created_at`,
		sub.UserID, sub.TopicID, contextArg)
	return scanSubscription(row)
}

func (s *PostgresStore) GetSubscription(ctx context.Context, subscriptionID int64) (Subscription, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT subscription_id, user_id, topic_id, context, created_at
		 FROM subscriptions WHERE subscription_id = $1`,
		subscriptionID)
	return scanSubscription(row)
}

func (s *PostgresStore) ListSubscriptionsByUser(ctx context.Context, userID int64) ([]Subscription, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT subscription_id, user_id, topic_id, context, created_at
		 FROM subscriptions WHERE user_id = $1 ORDER BY subscription_id`,
		userID)
	if err != nil {
		return nil, fmt.Errorf("notifications: error listing subscriptions for user %d: %w", userID, err)
	}
	defer rows.Close()
	return scanSubscriptions(rows)
}

func (s *PostgresStore) ListSubscriptionsByTopic(ctx context.Context, topicID int64) ([]Subscription, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT subscription_id, user_id, topic_id, context, created_at
		 FROM subscriptions WHERE topic_id = $1 ORDER BY subscription_id`,
		topicID)
	if err != nil {
		return nil, fmt.Errorf("notifications: error listing subscriptions for topic %d: %w", topicID, err)
	}
	defer rows.Close()
	return scanSubscriptions(rows)
}

func (s *PostgresStore) DeleteSubscription(ctx context.Context, subscriptionID int64) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM subscriptions WHERE subscription_id = $1`, subscriptionID)
	if err != nil {
		return fmt.Errorf("notifications: error deleting subscription %d: %w", subscriptionID, err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func scanSubscriptions(rows pgx.Rows) ([]Subscription, error) {
	var subs []Subscription
	for rows.Next() {
		sub, err := scanSubscription(rows)
		if err != nil {
			return nil, err
		}
		subs = append(subs, sub)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("notifications: error iterating subscriptions: %w", err)
	}
	return subs, nil
}

func scanSubscription(row pgx.Row) (Subscription, error) {
	var sub Subscription
	var rawContext []byte
	if err := row.Scan(&sub.SubscriptionID, &sub.UserID, &sub.TopicID, &rawContext, &sub.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Subscription{}, ErrNotFound
		}
		return Subscription{}, fmt.Errorf("notifications: error scanning subscription: %w", err)
	}
	contextData, err := scanJSON(rawContext)
	if err != nil {
		return Subscription{}, err
	}
	sub.Context = contextData
	return sub, nil
}
