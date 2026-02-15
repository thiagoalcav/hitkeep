package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"

	"hitkeep/internal/api"
)

// GetUserCount returns the total number of users.
func (s *Store) GetUserCount(ctx context.Context) (int, error) {
	var count int
	// Helper handles ErrNoRows (though COUNT always returns a row)
	err := s.QueryRowOrNil(ctx, "SELECT COUNT(*) FROM users", []any{&count})
	if err != nil {
		return 0, fmt.Errorf("could not query user count: %w", err)
	}
	return count, nil
}

// GetUserByEmail finds a user by email address. Returns nil if not found.
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
	// QueryRowOrNil returns nil error on no rows, so check if ID was populated
	if user.ID == uuid.Nil {
		return nil, nil
	}
	return &user, nil
}

// GetUserByID finds a user by UUID. Returns nil if not found.
func (s *Store) GetUserByID(ctx context.Context, id uuid.UUID) (*api.User, error) {
	var user api.User
	err := s.QueryRowOrNil(ctx,
		"SELECT id, email, password, created_at FROM users WHERE id = ?",
		[]any{&user.ID, &user.Email, &user.Password, &user.CreatedAt},
		id,
	)

	if err != nil {
		return nil, fmt.Errorf("could not query user by id: %w", err)
	}
	if user.ID == uuid.Nil {
		return nil, nil
	}
	return &user, nil
}

// ListUsers returns all users ordered by creation date.
func (s *Store) ListUsers(ctx context.Context) ([]api.User, error) {
	var users []api.User

	err := s.QueryList(ctx,
		"SELECT id, email, created_at FROM users ORDER BY created_at DESC",
		func(rows *sql.Rows) error {
			var u api.User
			// Note: password is not selected for listing
			if err := rows.Scan(&u.ID, &u.Email, &u.CreatedAt); err != nil {
				return fmt.Errorf("could not scan user: %w", err)
			}
			users = append(users, u)
			return nil
		},
	)

	if err != nil {
		return nil, fmt.Errorf("could not list users: %w", err)
	}
	return users, nil
}

// CreateUser creates a new user and assigns the 'owner' role if they are the first user.
// This uses a transaction, so we do not use the helpers here.
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

	// Check if this is the first user
	var count int
	err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&count)
	if err != nil {
		return uuid.Nil, fmt.Errorf("could not count users: %w", err)
	}

	// If first user, make them instance owner
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

// DeleteUser removes a user by ID.
func (s *Store) DeleteUser(ctx context.Context, id uuid.UUID) error {
	siteIDs, err := s.listUserSiteIDs(ctx, id)
	if err != nil {
		return err
	}

	for _, siteID := range siteIDs {
		if err := s.DeleteSite(ctx, siteID); err != nil {
			return fmt.Errorf("could not delete site %s: %w", siteID, err)
		}
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("could not begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := deleteUserRows(ctx, tx, id); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("could not commit transaction: %w", err)
	}
	return nil
}

func (s *Store) listUserSiteIDs(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id FROM sites WHERE user_id = ?", userID)
	if err != nil {
		return nil, fmt.Errorf("could not list user sites: %w", err)
	}
	defer rows.Close()

	var siteIDs []uuid.UUID
	for rows.Next() {
		var siteID uuid.UUID
		if err := rows.Scan(&siteID); err != nil {
			return nil, fmt.Errorf("could not scan site id: %w", err)
		}
		siteIDs = append(siteIDs, siteID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("could not read site ids: %w", err)
	}
	return siteIDs, nil
}

func deleteUserRows(ctx context.Context, tx *sql.Tx, userID uuid.UUID) error {
	if _, err := tx.ExecContext(ctx, "UPDATE share_links SET created_by = NULL WHERE created_by = ?", userID); err != nil {
		return fmt.Errorf("could not null share link owner: %w", err)
	}
	if _, err := tx.ExecContext(ctx, "UPDATE site_members SET added_by = NULL WHERE added_by = ?", userID); err != nil {
		return fmt.Errorf("could not null site member added_by: %w", err)
	}
	if _, err := tx.ExecContext(ctx, "UPDATE instance_roles SET granted_by = NULL WHERE granted_by = ?", userID); err != nil {
		return fmt.Errorf("could not null instance role granted_by: %w", err)
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM site_members WHERE user_id = ?", userID); err != nil {
		return fmt.Errorf("could not delete user site memberships: %w", err)
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM api_client_site_roles WHERE api_client_id IN (SELECT id FROM api_clients WHERE user_id = ?)", userID); err != nil {
		return fmt.Errorf("could not delete user api client site roles: %w", err)
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM api_clients WHERE user_id = ?", userID); err != nil {
		return fmt.Errorf("could not delete user api clients: %w", err)
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM remember_me_tokens WHERE user_id = ?", userID); err != nil {
		return fmt.Errorf("could not delete remember tokens: %w", err)
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM user_passkey_challenges WHERE user_id = ?", userID); err != nil {
		return fmt.Errorf("could not delete user passkey challenges: %w", err)
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM user_passkeys WHERE user_id = ?", userID); err != nil {
		return fmt.Errorf("could not delete user passkeys: %w", err)
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM user_totp_pending_setup WHERE user_id = ?", userID); err != nil {
		return fmt.Errorf("could not delete pending totp setup: %w", err)
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM user_totp_factors WHERE user_id = ?", userID); err != nil {
		return fmt.Errorf("could not delete user totp factors: %w", err)
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM instance_roles WHERE user_id = ?", userID); err != nil {
		return fmt.Errorf("could not delete instance role: %w", err)
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM users WHERE id = ?", userID); err != nil {
		return fmt.Errorf("could not delete user: %w", err)
	}
	return nil
}
