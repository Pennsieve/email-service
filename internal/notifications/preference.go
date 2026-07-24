package notifications

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// NotificationPreferenceStore manages the notification_preferences table: one
// row per user, upserted on first write.
type NotificationPreferenceStore interface {
	GetNotificationPreference(ctx context.Context, userID int64) (NotificationPreference, error)
	// UpsertNotificationPreference creates or replaces the row for pref.UserID.
	UpsertNotificationPreference(ctx context.Context, pref NotificationPreference) (NotificationPreference, error)
}

func (s *PostgresStore) GetNotificationPreference(ctx context.Context, userID int64) (NotificationPreference, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT user_id, email_enabled, sms_enabled, push_enabled
		 FROM notification_preferences WHERE user_id = $1`,
		userID)
	return scanNotificationPreference(row)
}

func (s *PostgresStore) UpsertNotificationPreference(ctx context.Context, pref NotificationPreference) (NotificationPreference, error) {
	row := s.pool.QueryRow(ctx,
		`INSERT INTO notification_preferences (user_id, email_enabled, sms_enabled, push_enabled)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (user_id) DO UPDATE SET
		   email_enabled = EXCLUDED.email_enabled,
		   sms_enabled   = EXCLUDED.sms_enabled,
		   push_enabled  = EXCLUDED.push_enabled
		 RETURNING user_id, email_enabled, sms_enabled, push_enabled`,
		pref.UserID, pref.EmailEnabled, pref.SMSEnabled, pref.PushEnabled)
	return scanNotificationPreference(row)
}

func scanNotificationPreference(row pgx.Row) (NotificationPreference, error) {
	var p NotificationPreference
	if err := row.Scan(&p.UserID, &p.EmailEnabled, &p.SMSEnabled, &p.PushEnabled); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return NotificationPreference{}, ErrNotFound
		}
		return NotificationPreference{}, fmt.Errorf("notifications: error scanning notification_preference: %w", err)
	}
	return p, nil
}
