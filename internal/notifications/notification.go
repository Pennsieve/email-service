package notifications

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// NotificationStore manages the notifications table: events that occurred on
// a topic.
type NotificationStore interface {
	CreateNotification(ctx context.Context, n Notification) (Notification, error)
	GetNotification(ctx context.Context, notificationID int64) (Notification, error)
	ListNotificationsByTopic(ctx context.Context, topicID int64) ([]Notification, error)
}

func (s *PostgresStore) CreateNotification(ctx context.Context, n Notification) (Notification, error) {
	metadata, err := jsonArg(n.Metadata)
	if err != nil {
		return Notification{}, err
	}
	row := s.pool.QueryRow(ctx,
		`INSERT INTO notifications (topic_id, sender_id, title, message, metadata)
		 VALUES ($1, $2, $3, $4, $5::jsonb)
		 RETURNING notification_id, topic_id, sender_id, title, message, metadata, created_at`,
		n.TopicID, n.SenderID, n.Title, n.Message, metadata)
	return scanNotification(row)
}

func (s *PostgresStore) GetNotification(ctx context.Context, notificationID int64) (Notification, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT notification_id, topic_id, sender_id, title, message, metadata, created_at
		 FROM notifications WHERE notification_id = $1`,
		notificationID)
	return scanNotification(row)
}

func (s *PostgresStore) ListNotificationsByTopic(ctx context.Context, topicID int64) ([]Notification, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT notification_id, topic_id, sender_id, title, message, metadata, created_at
		 FROM notifications WHERE topic_id = $1 ORDER BY notification_id`,
		topicID)
	if err != nil {
		return nil, fmt.Errorf("notifications: error listing notifications for topic %d: %w", topicID, err)
	}
	defer rows.Close()

	var out []Notification
	for rows.Next() {
		n, err := scanNotification(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("notifications: error iterating notifications: %w", err)
	}
	return out, nil
}

func scanNotification(row pgx.Row) (Notification, error) {
	var n Notification
	var rawMetadata []byte
	if err := row.Scan(&n.NotificationID, &n.TopicID, &n.SenderID, &n.Title, &n.Message, &rawMetadata, &n.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Notification{}, ErrNotFound
		}
		return Notification{}, fmt.Errorf("notifications: error scanning notification: %w", err)
	}
	metadata, err := scanJSON(rawMetadata)
	if err != nil {
		return Notification{}, err
	}
	n.Metadata = metadata
	return n, nil
}
