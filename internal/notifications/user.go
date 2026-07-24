package notifications

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// UserStore manages the users table.
type UserStore interface {
	CreateUser(ctx context.Context, u User) (User, error)
	GetUser(ctx context.Context, userID int64) (User, error)
	GetUserByEmail(ctx context.Context, email string) (User, error)
}

func (s *PostgresStore) CreateUser(ctx context.Context, u User) (User, error) {
	row := s.pool.QueryRow(ctx,
		`INSERT INTO users (name, email) VALUES ($1, $2)
		 RETURNING user_id, name, email, created_at`,
		u.Name, u.Email)
	return scanUser(row)
}

func (s *PostgresStore) GetUser(ctx context.Context, userID int64) (User, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT user_id, name, email, created_at FROM users WHERE user_id = $1`,
		userID)
	return scanUser(row)
}

func (s *PostgresStore) GetUserByEmail(ctx context.Context, email string) (User, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT user_id, name, email, created_at FROM users WHERE email = $1`,
		email)
	return scanUser(row)
}

func scanUser(row pgx.Row) (User, error) {
	var u User
	if err := row.Scan(&u.UserID, &u.Name, &u.Email, &u.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, ErrNotFound
		}
		return User{}, fmt.Errorf("notifications: error scanning user: %w", err)
	}
	return u, nil
}
