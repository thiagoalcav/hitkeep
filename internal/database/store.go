package database

import (
	"database/sql"
	"fmt"
	"log/slog"
	"sync"

	_ "github.com/duckdb/duckdb-go/v2"
)

type Store struct {
	db                  *sql.DB
	path                string
	analyticsMu         sync.Mutex
	analyticsStatements *analyticsStatements
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
	s.analyticsMu.Lock()
	if s.analyticsStatements != nil {
		_ = s.analyticsStatements.close()
		s.analyticsStatements = nil
	}
	s.analyticsMu.Unlock()
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

func (s *Store) DB() *sql.DB {
	return s.db
}
