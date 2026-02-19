package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	_ "github.com/duckdb/duckdb-go/v2"
)

const (
	walAutoCheckpointSize         = "64MB"
	maintenanceCheckpointInterval = 15 * time.Minute
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
	if err := s.loadExcelExtension(); err != nil {
		slog.Warn("DuckDB excel extension unavailable; XLSX exports may fail", "error", err)
	}
	if _, err := s.db.Exec(fmt.Sprintf("PRAGMA wal_autocheckpoint='%s';", walAutoCheckpointSize)); err != nil {
		slog.Warn("Failed to set wal_autocheckpoint", "size", walAutoCheckpointSize, "error", err)
	}
	slog.Debug("Database connection established successfully.")
	return nil
}

func (s *Store) loadExcelExtension() error {
	if _, err := s.db.Exec("LOAD excel;"); err == nil {
		return nil
	} else {
		loadErr := err
		if _, err := s.db.Exec("INSTALL excel;"); err != nil {
			return fmt.Errorf("load excel extension: %w; install excel extension: %v", loadErr, err)
		}
		if _, err := s.db.Exec("LOAD excel;"); err != nil {
			return fmt.Errorf("load excel extension after install: %w", err)
		}
	}
	return nil
}

func (s *Store) StartMaintenance(ctx context.Context) {
	if s.db == nil {
		slog.Warn("Skipping database maintenance loop because database is not connected")
		return
	}

	ticker := time.NewTicker(maintenanceCheckpointInterval)
	go func() {
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				slog.Debug("Running database checkpoint...")
				if _, err := s.db.ExecContext(ctx, "CHECKPOINT;"); err != nil && !errors.Is(err, context.Canceled) {
					slog.Error("Checkpoint failed", "error", err)
				}
			}
		}
	}()
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
