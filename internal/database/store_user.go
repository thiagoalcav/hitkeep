package database

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"hitkeep/internal/api"
)

func (s *Store) GetUserCount(ctx context.Context) (int, error) {
	var count int
	// Use slice []any{&count} for the destination
	err := s.QueryRowOrNil(ctx, "SELECT COUNT(*) FROM users", []any{&count})
	if err != nil {
		return 0, fmt.Errorf("could not query user count: %w", err)
	}
	return count, nil
}

func (s *Store) GetUserByEmail(ctx context.Context, email string) (*api.User, error) {
	var user api.User
	err := s.QueryRowOrNil(ctx,
		"SELECT id, email, password, created_at FROM users WHERE email = ?",
		[]any{&user.ID, &user.Email, &user.Password, &user.CreatedAt},
		email,
	)

	if err != nil {
		return nil, fmt.Errorf("could not query user: %w", err)
	}
	if user.ID == uuid.Nil {
		return nil, nil
	}
	return &user, nil
}

func (s *Store) CreateUser(ctx context.Context, email string, hashedPassword string) (uuid.UUID, error) {
	id := uuid.New()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return uuid.Nil, fmt.Errorf("could not begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.ExecContext(ctx,
		"INSERT INTO users (id, email, password, created_at) VALUES (?, ?, ?, ?)",
		id, email, hashedPassword, time.Now(),
	)
	if err != nil {
		return uuid.Nil, fmt.Errorf("could not create user: %w", err)
	}

	var count int
	err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&count)
	if err != nil {
		return uuid.Nil, fmt.Errorf("could not count users: %w", err)
	}

	if count == 1 {
		_, err = tx.ExecContext(ctx,
			"INSERT INTO instance_roles (user_id, role) VALUES (?, 'owner')",
			id,
		)
		if err != nil {
			return uuid.Nil, fmt.Errorf("could not assign owner role: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return uuid.Nil, fmt.Errorf("could not commit transaction: %w", err)
	}

	return id, nil
}

func (s *Store) DeleteUser(ctx context.Context, id uuid.UUID) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM users WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("could not delete user: %w", err)
	}
	return nil
}
