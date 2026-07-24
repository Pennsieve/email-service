-- Reverse of 0001_create_notification_schema.up.sql, in dependency order.

DROP TABLE IF EXISTS notification_audits;
DROP TABLE IF EXISTS notification_preferences;
DROP TABLE IF EXISTS user_notifications;
DROP TABLE IF EXISTS messages;
DROP TABLE IF EXISTS notifications;
DROP TABLE IF EXISTS subscriptions;
DROP TABLE IF EXISTS topics;
DROP TABLE IF EXISTS users;

DROP TYPE IF EXISTS notification_read_status;
