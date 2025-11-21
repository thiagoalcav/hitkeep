package database

import (
	"database/sql"
	"fmt"
	"log/slog"

	_ "github.com/duckdb/duckdb-go/v2"
)

type Store struct {
	db   *sql.DB
	path string
}

func NewStore(path string) *Store {
	return &Store{
		path: path,
	}
}

func (s *Store) Connect() error {
	slog.Info("Connecting to database...", "path", s.path)
	db, err := sql.Open("duckdb", s.path)
	if err != nil {
		return fmt.Errorf("could not open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return fmt.Errorf("could not connect to database: %w", err)
	}

	s.db = db
	slog.Debug("Database connection established successfully.")
	return nil
}

func (s *Store) Close() error {
	slog.Debug("Closing database connection...")
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

func (s *Store) DB() *sql.DB {
	return s.db
}
