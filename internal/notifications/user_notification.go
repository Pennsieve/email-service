package notifications

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// UserNotificationStore manages the user_notifications table: per-user
// delivery/read status of a notification.
type UserNotificationStore interface {
	CreateUserNotification(ctx context.Context, un UserNotification) (UserNotification, error)
	GetUserNotification(ctx context.Context, userNotificationID int64) (UserNotification, error)
	// ListUserNotifications returns a user's notifications, optionally filtered
	// by status. Pass "" to return all statuses.
	ListUserNotifications(ctx context.Context, userID int64, status ReadStatus) ([]UserNotification, error)
	// MarkRead sets status to READ and read_at to now() for the given row.
	MarkRead(ctx context.Context, userNotificationID int64) error
}

func (s *PostgresStore) CreateUserNotification(ctx context.Context, un UserNotification) (UserNotification, error) {
	row := s.pool.QueryRow(ctx,
		`INSERT INTO user_notifications (user_id, notification_id, status)
		 VALUES ($1, $2, $3)
		 RETURNING user_notification_id, user_id, notification_id, status, delivered_at, read_at`,
		un.UserID, un.NotificationID, statusOrDefault(un.Status))
	return scanUserNotification(row)
}

func (s *PostgresStore) GetUserNotification(ctx context.Context, userNotificationID int64) (UserNotification, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT user_notification_id, user_id, notification_id, status, delivered_at, read_at
		 FROM user_notifications WHERE user_notification_id = $1`,
		userNotificationID)
	return scanUserNotification(row)
}

func (s *PostgresStore) ListUserNotifications(ctx context.Context, userID int64, status ReadStatus) ([]UserNotification, error) {
	var rows pgx.Rows
	var err error
	if status == "" {
		rows, err = s.pool.Query(ctx,
			`SELECT user_notification_id, user_id, notification_id, status, delivered_at, read_at
			 FROM user_notifications WHERE user_id = $1 ORDER BY delivered_at DESC`,
			userID)
	} else {
		rows, err = s.pool.Query(ctx,
			`SELECT user_notification_id, user_id, notification_id, status, delivered_at, read_at
			 FROM user_notifications WHERE user_id = $1 AND status = $2 ORDER BY delivered_at DESC`,
			userID, status)
	}
	if err != nil {
		return nil, fmt.Errorf("notifications: error listing user_notifications for user %d: %w", userID, err)
	}
	defer rows.Close()

	var out []UserNotification
	for rows.Next() {
		un, err := scanUserNotification(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, un)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("notifications: error iterating user_notifications: %w", err)
	}
	return out, nil
}

func (s *PostgresStore) MarkRead(ctx context.Context, userNotificationID int64) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE user_notifications SET status = $1, read_at = now() WHERE user_notification_id = $2`,
		StatusRead, userNotificationID)
	if err != nil {
		return fmt.Errorf("notifications: error marking user_notification %d read: %w", userNotificationID, err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func statusOrDefault(status ReadStatus) ReadStatus {
	if status == "" {
		return StatusUnread
	}
	return status
}

func scanUserNotification(row pgx.Row) (UserNotification, error) {
	var un UserNotification
	if err := row.Scan(&un.UserNotificationID, &un.UserID, &un.NotificationID, &un.Status, &un.DeliveredAt, &un.ReadAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return UserNotification{}, ErrNotFound
		}
		return UserNotification{}, fmt.Errorf("notifications: error scanning user_notification: %w", err)
	}
	return un, nil
}
