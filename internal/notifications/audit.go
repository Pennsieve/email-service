package notifications

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// NotificationAuditStore manages the notification_audits table: an
// append-only trail of lifecycle events for a notification.
type NotificationAuditStore interface {
	CreateNotificationAudit(ctx context.Context, a NotificationAudit) (NotificationAudit, error)
	ListNotificationAuditsByNotification(ctx context.Context, notificationID int64) ([]NotificationAudit, error)
}

func (s *PostgresStore) CreateNotificationAudit(ctx context.Context, a NotificationAudit) (NotificationAudit, error) {
	details, err := jsonArg(a.Details)
	if err != nil {
		return NotificationAudit{}, err
	}
	row := s.pool.QueryRow(ctx,
		`INSERT INTO notification_audits (notification_id, event_type, details)
		 VALUES ($1, $2, $3::jsonb)
		 RETURNING audit_id, notification_id, event_type, "timestamp", details`,
		a.NotificationID, a.EventType, details)
	return scanNotificationAudit(row)
}

func (s *PostgresStore) ListNotificationAuditsByNotification(ctx context.Context, notificationID int64) ([]NotificationAudit, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT audit_id, notification_id, event_type, "timestamp", details
		 FROM notification_audits WHERE notification_id = $1 ORDER BY "timestamp"`,
		notificationID)
	if err != nil {
		return nil, fmt.Errorf("notifications: error listing audits for notification %d: %w", notificationID, err)
	}
	defer rows.Close()

	var out []NotificationAudit
	for rows.Next() {
		a, err := scanNotificationAudit(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("notifications: error iterating audits: %w", err)
	}
	return out, nil
}

func scanNotificationAudit(row pgx.Row) (NotificationAudit, error) {
	var a NotificationAudit
	var rawDetails []byte
	if err := row.Scan(&a.AuditID, &a.NotificationID, &a.EventType, &a.Timestamp, &rawDetails); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return NotificationAudit{}, ErrNotFound
		}
		return NotificationAudit{}, fmt.Errorf("notifications: error scanning notification_audit: %w", err)
	}
	details, err := scanJSON(rawDetails)
	if err != nil {
		return NotificationAudit{}, err
	}
	a.Details = details
	return a, nil
}
