package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"hitkeep/internal/api"

	"github.com/google/uuid"
)

func (s *Store) GetUserCount(ctx context.Context) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("could not query user count: %w", err)
	}
	return count, nil
}

func (s *Store) GetUserByEmail(ctx context.Context, email string) (*api.User, error) {
	var user api.User
	err := s.db.QueryRowContext(ctx,
		"SELECT id, email, password, created_at FROM users WHERE email = ?",
		email,
	).Scan(&user.ID, &user.Email, &user.Password, &user.CreatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("could not query user: %w", err)
	}
	return &user, nil
}

func (s *Store) CreateUser(ctx context.Context, email string, hashedPassword string) (uuid.UUID, error) {
	id := uuid.New()
	_, err := s.db.ExecContext(ctx,
		"INSERT INTO users (id, email, password, created_at) VALUES (?, ?, ?, ?)",
		id, email, hashedPassword, time.Now(),
	)
	if err != nil {
		return uuid.Nil, fmt.Errorf("could not create user: %w", err)
	}
	return id, nil
}
