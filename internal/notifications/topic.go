package notifications

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// TopicStore manages the topics table.
type TopicStore interface {
	CreateTopic(ctx context.Context, t Topic) (Topic, error)
	GetTopic(ctx context.Context, topicID int64) (Topic, error)
	ListTopics(ctx context.Context) ([]Topic, error)
}

func (s *PostgresStore) CreateTopic(ctx context.Context, t Topic) (Topic, error) {
	row := s.pool.QueryRow(ctx,
		`INSERT INTO topics (name, description) VALUES ($1, $2)
		 RETURNING topic_id, name, description, created_at`,
		t.Name, t.Description)
	return scanTopic(row)
}

func (s *PostgresStore) GetTopic(ctx context.Context, topicID int64) (Topic, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT topic_id, name, description, created_at FROM topics WHERE topic_id = $1`,
		topicID)
	return scanTopic(row)
}

func (s *PostgresStore) ListTopics(ctx context.Context) ([]Topic, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT topic_id, name, description, created_at FROM topics ORDER BY topic_id`)
	if err != nil {
		return nil, fmt.Errorf("notifications: error listing topics: %w", err)
	}
	defer rows.Close()

	var topics []Topic
	for rows.Next() {
		t, err := scanTopic(rows)
		if err != nil {
			return nil, err
		}
		topics = append(topics, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("notifications: error iterating topics: %w", err)
	}
	return topics, nil
}

func scanTopic(row pgx.Row) (Topic, error) {
	var t Topic
	if err := row.Scan(&t.TopicID, &t.Name, &t.Description, &t.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Topic{}, ErrNotFound
		}
		return Topic{}, fmt.Errorf("notifications: error scanning topic: %w", err)
	}
	return t, nil
}
