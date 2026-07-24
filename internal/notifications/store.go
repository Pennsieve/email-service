package notifications

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound is returned by a Get method when no row matches the given key.
var ErrNotFound = errors.New("notifications: not found")

// Store is the repository interface over the notification schema. It is
// intentionally one interface per entity's CRUD/list needs rather than a
// single monolithic method set, so callers can depend on just the entities
// they use.
type Store interface {
	UserStore
	TopicStore
	SubscriptionStore
	NotificationStore
	MessageStore
	UserNotificationStore
	NotificationPreferenceStore
	NotificationAuditStore
}

// PostgresStore is a Store backed by a Postgres database via pgx.
type PostgresStore struct {
	pool *pgxpool.Pool
}

// NewPostgresStore returns a Store that runs queries against pool. Callers own
// the pool's lifecycle (e.g. via pgxpool.New and pool.Close).
func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

var _ Store = (*PostgresStore)(nil)

// jsonArg marshals m for binding to a jsonb column parameter (cast with
// "::jsonb" in the query text). A nil map binds SQL NULL rather than the
// empty-string literal, which "::jsonb" would otherwise reject.
func jsonArg(m map[string]any) (*string, error) {
	if m == nil {
		return nil, nil
	}
	b, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("notifications: error marshalling json: %w", err)
	}
	s := string(b)
	return &s, nil
}

// scanJSON unmarshals the raw bytes of a jsonb column. A NULL column scans as
// a nil raw slice, which returns a nil map.
func scanJSON(raw []byte) (map[string]any, error) {
	if raw == nil {
		return nil, nil
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("notifications: error unmarshalling json: %w", err)
	}
	return m, nil
}
