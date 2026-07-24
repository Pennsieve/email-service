-- Notification schema: lets producers publish events on a Topic, users
-- subscribe to topics they care about, and the service track per-user
-- delivery/read status plus a user-to-user message thread and an audit trail.
--
-- Table names are pluralized snake_case versions of the entities in the design
-- doc ("User" -> users, "NotificationPreference" -> notification_preferences,
-- etc.) so no identifier collides with a reserved word (notably "user").

CREATE TYPE notification_read_status AS ENUM ('UNREAD', 'READ');

CREATE TABLE users (
    user_id    BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name       TEXT        NOT NULL,
    email      TEXT        NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE topics (
    topic_id    BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name        TEXT        NOT NULL UNIQUE,
    description TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- One row per (user, topic): a user subscribes to a topic at most once.
-- context holds subscription-scoped metadata (e.g. filters) as free-form JSON.
CREATE TABLE subscriptions (
    subscription_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id         BIGINT      NOT NULL REFERENCES users (user_id) ON DELETE CASCADE,
    topic_id        BIGINT      NOT NULL REFERENCES topics (topic_id) ON DELETE CASCADE,
    context         JSONB,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, topic_id)
);

CREATE INDEX subscriptions_topic_id_idx ON subscriptions (topic_id);

-- An event that occurred on a topic. sender_id is who/what triggered it.
CREATE TABLE notifications (
    notification_id BIGINT      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    topic_id         BIGINT      NOT NULL REFERENCES topics (topic_id) ON DELETE CASCADE,
    sender_id        BIGINT      NOT NULL REFERENCES users (user_id) ON DELETE RESTRICT,
    title            TEXT        NOT NULL,
    message          TEXT        NOT NULL,
    metadata         JSONB,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX notifications_topic_id_idx ON notifications (topic_id);
CREATE INDEX notifications_sender_id_idx ON notifications (sender_id);

-- User-to-user communication attached to a notification (e.g. a reply/comment
-- thread on the event). from_user/to_user aren't marked (FK) in the design doc
-- but are clearly user references, so they're constrained here too.
CREATE TABLE messages (
    message_id      BIGINT      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    from_user       BIGINT      NOT NULL REFERENCES users (user_id) ON DELETE RESTRICT,
    to_user         BIGINT      NOT NULL REFERENCES users (user_id) ON DELETE RESTRICT,
    notification_id BIGINT      NOT NULL REFERENCES notifications (notification_id) ON DELETE CASCADE,
    content         TEXT        NOT NULL,
    "timestamp"     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX messages_notification_id_idx ON messages (notification_id);
CREATE INDEX messages_from_user_idx ON messages (from_user);
CREATE INDEX messages_to_user_idx ON messages (to_user);

-- Per-user delivery/read status of a notification. One row per (user,
-- notification); read_at is set only once the user reads it.
CREATE TABLE user_notifications (
    user_notification_id BIGINT                   GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id              BIGINT                   NOT NULL REFERENCES users (user_id) ON DELETE CASCADE,
    notification_id      BIGINT                   NOT NULL REFERENCES notifications (notification_id) ON DELETE CASCADE,
    status               notification_read_status NOT NULL DEFAULT 'UNREAD',
    delivered_at         TIMESTAMPTZ               NOT NULL DEFAULT now(),
    read_at              TIMESTAMPTZ,
    UNIQUE (user_id, notification_id)
);

CREATE INDEX user_notifications_notification_id_idx ON user_notifications (notification_id);
-- Serves "a user's unread notifications" lookups.
CREATE INDEX user_notifications_user_status_idx ON user_notifications (user_id, status);

-- One row per user; created lazily (upserted) the first time preferences are set.
CREATE TABLE notification_preferences (
    user_id       BIGINT  PRIMARY KEY REFERENCES users (user_id) ON DELETE CASCADE,
    email_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    sms_enabled   BOOLEAN NOT NULL DEFAULT FALSE,
    push_enabled  BOOLEAN NOT NULL DEFAULT TRUE
);

-- Append-only audit trail of lifecycle events for a notification (e.g.
-- created, delivery attempted, delivery failed).
CREATE TABLE notification_audits (
    audit_id        BIGINT      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    notification_id BIGINT      NOT NULL REFERENCES notifications (notification_id) ON DELETE CASCADE,
    event_type      TEXT        NOT NULL,
    "timestamp"     TIMESTAMPTZ NOT NULL DEFAULT now(),
    details         JSONB
);

CREATE INDEX notification_audits_notification_id_idx ON notification_audits (notification_id);
