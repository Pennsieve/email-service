// Package notifications is the data layer for the notification schema: a
// Topic-based event system where Users subscribe to Topics, a Notification
// records an event on a Topic, and UserNotification tracks per-user
// delivery/read state. Message carries user-to-user communication attached to
// a Notification; NotificationPreference and NotificationAudit hold delivery
// channel preferences and an audit trail, respectively.
//
// See internal/notifications/migrations for the backing Postgres schema.
package notifications

import "time"

// ReadStatus is the delivery/read state of a UserNotification row.
type ReadStatus string

const (
	StatusUnread ReadStatus = "UNREAD"
	StatusRead   ReadStatus = "READ"
)

// User is a person who can subscribe to topics and receive notifications.
type User struct {
	UserID    int64     `db:"user_id"`
	Name      string    `db:"name"`
	Email     string    `db:"email"`
	CreatedAt time.Time `db:"created_at"`
}

// Topic is an event category users can subscribe to.
type Topic struct {
	TopicID     int64     `db:"topic_id"`
	Name        string    `db:"name"`
	Description string    `db:"description"`
	CreatedAt   time.Time `db:"created_at"`
}

// Subscription records that a user is interested in a topic. Context holds
// arbitrary subscription-scoped metadata (e.g. filters).
type Subscription struct {
	SubscriptionID int64          `db:"subscription_id"`
	UserID         int64          `db:"user_id"`
	TopicID        int64          `db:"topic_id"`
	Context        map[string]any `db:"context"`
	CreatedAt      time.Time      `db:"created_at"`
}

// Notification is an event that occurred on a topic, e.g. "dataset published".
type Notification struct {
	NotificationID int64          `db:"notification_id"`
	TopicID        int64          `db:"topic_id"`
	SenderID       int64          `db:"sender_id"`
	Title          string         `db:"title"`
	Message        string         `db:"message"`
	Metadata       map[string]any `db:"metadata"`
	CreatedAt      time.Time      `db:"created_at"`
}

// Message is user-to-user communication attached to a notification (e.g. a
// reply or comment on the event it describes).
type Message struct {
	MessageID      int64     `db:"message_id"`
	FromUser       int64     `db:"from_user"`
	ToUser         int64     `db:"to_user"`
	NotificationID int64     `db:"notification_id"`
	Content        string    `db:"content"`
	Timestamp      time.Time `db:"timestamp"`
}

// UserNotification is the per-user delivery/read status of a notification.
// ReadAt is nil until the user reads it.
type UserNotification struct {
	UserNotificationID int64      `db:"user_notification_id"`
	UserID             int64      `db:"user_id"`
	NotificationID     int64      `db:"notification_id"`
	Status             ReadStatus `db:"status"`
	DeliveredAt        time.Time  `db:"delivered_at"`
	ReadAt             *time.Time `db:"read_at"`
}

// NotificationPreference holds a user's opt-in/out per delivery channel. One
// row per user, keyed by UserID (no separate surrogate id).
type NotificationPreference struct {
	UserID       int64 `db:"user_id"`
	EmailEnabled bool  `db:"email_enabled"`
	SMSEnabled   bool  `db:"sms_enabled"`
	PushEnabled  bool  `db:"push_enabled"`
}

// NotificationAudit is one entry in the append-only audit trail for a
// notification's lifecycle (e.g. "created", "delivery_failed").
type NotificationAudit struct {
	AuditID        int64          `db:"audit_id"`
	NotificationID int64          `db:"notification_id"`
	EventType      string         `db:"event_type"`
	Timestamp      time.Time      `db:"timestamp"`
	Details        map[string]any `db:"details"`
}
