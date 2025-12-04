package database

import (
	"context"
	"database/sql"
	"errors"
)

// QueryRowOrNil executes a query expecting a single row.
// It accepts a slice of pointers (dest) to scan into.
// Returns nil if no rows are found (suppressing sql.ErrNoRows).
func (s *Store) QueryRowOrNil(ctx context.Context, query string, dest []any, args ...any) error {
	err := s.db.QueryRowContext(ctx, query, args...).Scan(dest...)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	return err
}

// QueryList executes a query and iterates over rows, calling scanFn for each.
// It handles row closing and error checking automatically.
func (s *Store) QueryList(ctx context.Context, query string, scanFn func(*sql.Rows) error, args ...any) error {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		if err := scanFn(rows); err != nil {
			return err
		}
	}
	return rows.Err()
}
