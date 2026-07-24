package notifications

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// MessageStore manages the messages table: user-to-user communication
// attached to a notification.
type MessageStore interface {
	CreateMessage(ctx context.Context, m Message) (Message, error)
	GetMessage(ctx context.Context, messageID int64) (Message, error)
	ListMessagesByNotification(ctx context.Context, notificationID int64) ([]Message, error)
}

func (s *PostgresStore) CreateMessage(ctx context.Context, m Message) (Message, error) {
	row := s.pool.QueryRow(ctx,
		`INSERT INTO messages (from_user, to_user, notification_id, content)
		 VALUES ($1, $2, $3, $4)
		 RETURNING message_id, from_user, to_user, notification_id, content, "timestamp"`,
		m.FromUser, m.ToUser, m.NotificationID, m.Content)
	return scanMessage(row)
}

func (s *PostgresStore) GetMessage(ctx context.Context, messageID int64) (Message, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT message_id, from_user, to_user, notification_id, content, "timestamp"
		 FROM messages WHERE message_id = $1`,
		messageID)
	return scanMessage(row)
}

func (s *PostgresStore) ListMessagesByNotification(ctx context.Context, notificationID int64) ([]Message, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT message_id, from_user, to_user, notification_id, content, "timestamp"
		 FROM messages WHERE notification_id = $1 ORDER BY "timestamp"`,
		notificationID)
	if err != nil {
		return nil, fmt.Errorf("notifications: error listing messages for notification %d: %w", notificationID, err)
	}
	defer rows.Close()

	var out []Message
	for rows.Next() {
		m, err := scanMessage(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("notifications: error iterating messages: %w", err)
	}
	return out, nil
}

func scanMessage(row pgx.Row) (Message, error) {
	var m Message
	if err := row.Scan(&m.MessageID, &m.FromUser, &m.ToUser, &m.NotificationID, &m.Content, &m.Timestamp); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Message{}, ErrNotFound
		}
		return Message{}, fmt.Errorf("notifications: error scanning message: %w", err)
	}
	return m, nil
}
