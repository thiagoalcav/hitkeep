package worker

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"

	"hitkeep/internal/database"
)

type RetentionWorker struct {
	store       *database.Store
	path        string
	defaultDays int
}

func NewRetentionWorker(store *database.Store, archivePath string, defaultDays int) *RetentionWorker {
	return &RetentionWorker{
		store:       store,
		path:        archivePath,
		defaultDays: defaultDays,
	}
}

func (w *RetentionWorker) Start(ctx context.Context) {
	// Run once on startup after a short delay to let DB settle
	go func() {
		time.Sleep(10 * time.Second)
		if err := w.Run(ctx); err != nil {
			slog.Error("Initial retention run failed", "error", err)
		}
	}()

	// Run daily
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := w.Run(ctx); err != nil {
				slog.Error("Retention worker failed", "error", err)
			}
		}
	}
}

func (w *RetentionWorker) Run(ctx context.Context) error {
	slog.Debug("Checking for data retention cleanup...")

	// 1. Ensure archive directory exists
	if err := os.MkdirAll(w.path, 0755); err != nil {
		return fmt.Errorf("failed to create archive directory: %w", err)
	}

	// 2. Get all sites with retention policy
	rows, err := w.store.DB().QueryContext(ctx, "SELECT id, data_retention_days FROM sites WHERE data_retention_days IS NOT NULL AND data_retention_days > 0")
	if err != nil {
		return fmt.Errorf("failed to query sites: %w", err)
	}
	defer rows.Close()

	type sitePolicy struct {
		ID   uuid.UUID
		Days int
	}
	var policies []sitePolicy

	for rows.Next() {
		var p sitePolicy
		if err := rows.Scan(&p.ID, &p.Days); err != nil {
			slog.Error("Failed to scan site policy", "error", err)
			continue
		}
		policies = append(policies, p)
	}
	rows.Close()

	for _, p := range policies {
		cutoff := time.Now().AddDate(0, 0, -p.Days)

		var hitCount, eventCount int64
		err := w.store.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM hits WHERE site_id = ? AND timestamp < ?", p.ID, cutoff).Scan(&hitCount)
		if err != nil {
			slog.Error("Failed to count hits for retention", "error", err, "site_id", p.ID)
			continue
		}
		err = w.store.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM events WHERE site_id = ? AND timestamp < ?", p.ID, cutoff).Scan(&eventCount)
		if err != nil {
			slog.Error("Failed to count events for retention", "error", err, "site_id", p.ID)
			continue
		}

		if hitCount == 0 && eventCount == 0 {
			continue
		}

		slog.Info("Archiving old data", "site_id", p.ID, "hits", hitCount, "events", eventCount, "cutoff", cutoff.Format(time.DateOnly))

		filename := filepath.Join(w.path, fmt.Sprintf("site_%s_%d.parquet", p.ID, time.Now().Unix()))

		exportQuery := fmt.Sprintf(`
			COPY (
				SELECT * FROM hits WHERE site_id = '%s' AND timestamp < '%s'
				UNION BY NAME
				SELECT * FROM events WHERE site_id = '%s' AND timestamp < '%s'
			) TO '%s' (FORMAT PARQUET, COMPRESSION 'SNAPPY');
		`, p.ID, cutoff.Format(time.RFC3339), p.ID, cutoff.Format(time.RFC3339), filename)

		if _, err := w.store.DB().ExecContext(ctx, exportQuery); err != nil {
			slog.Error("Failed to export data to parquet", "error", err, "site_id", p.ID)
			continue
		}

		tx, err := w.store.DB().BeginTx(ctx, nil)
		if err != nil {
			slog.Error("Failed to start transaction for deletion", "error", err)
			continue
		}

		if hitCount > 0 {
			if _, err := tx.ExecContext(ctx, "DELETE FROM hits WHERE site_id = ? AND timestamp < ?", p.ID, cutoff); err != nil {
				slog.Error("Failed to prune hits", "error", err, "site_id", p.ID)
				defer func() { _ = tx.Rollback() }()
				continue
			}
		}

		if eventCount > 0 {
			if _, err := tx.ExecContext(ctx, "DELETE FROM events WHERE site_id = ? AND timestamp < ?", p.ID, cutoff); err != nil {
				slog.Error("Failed to prune events", "error", err, "site_id", p.ID)
				defer func() { _ = tx.Rollback() }()
				continue
			}
		}

		if _, err := tx.ExecContext(ctx, "DELETE FROM hit_rollups_hourly WHERE site_id = ? AND bucket < ?", p.ID, cutoff); err != nil {
			slog.Error("Failed to prune hourly rollups", "error", err, "site_id", p.ID)
			defer func() { _ = tx.Rollback() }()
			continue
		}

		if _, err := tx.ExecContext(ctx, "DELETE FROM hit_rollups_daily WHERE site_id = ? AND bucket < ?", p.ID, cutoff); err != nil {
			slog.Error("Failed to prune daily rollups", "error", err, "site_id", p.ID)
			defer func() { _ = tx.Rollback() }()
			continue
		}

		if _, err := tx.ExecContext(ctx, "DELETE FROM hit_rollups_monthly WHERE site_id = ? AND bucket < ?", p.ID, cutoff); err != nil {
			slog.Error("Failed to prune monthly rollups", "error", err, "site_id", p.ID)
			defer func() { _ = tx.Rollback() }()
			continue
		}

		if _, err := tx.ExecContext(ctx, "DELETE FROM goal_rollups_hourly WHERE site_id = ? AND bucket < ?", p.ID, cutoff); err != nil {
			slog.Error("Failed to prune hourly goal rollups", "error", err, "site_id", p.ID)
			defer func() { _ = tx.Rollback() }()
			continue
		}

		if _, err := tx.ExecContext(ctx, "DELETE FROM goal_rollups_daily WHERE site_id = ? AND bucket < ?", p.ID, cutoff); err != nil {
			slog.Error("Failed to prune daily goal rollups", "error", err, "site_id", p.ID)
			defer func() { _ = tx.Rollback() }()
			continue
		}

		if _, err := tx.ExecContext(ctx, "DELETE FROM goal_rollups_monthly WHERE site_id = ? AND bucket < ?", p.ID, cutoff); err != nil {
			slog.Error("Failed to prune monthly goal rollups", "error", err, "site_id", p.ID)
			defer func() { _ = tx.Rollback() }()
			continue
		}

		if _, err := tx.ExecContext(ctx, "DELETE FROM funnel_rollups_hourly WHERE site_id = ? AND bucket < ?", p.ID, cutoff); err != nil {
			slog.Error("Failed to prune hourly funnel rollups", "error", err, "site_id", p.ID)
			defer func() { _ = tx.Rollback() }()
			continue
		}

		if _, err := tx.ExecContext(ctx, "DELETE FROM funnel_rollups_daily WHERE site_id = ? AND bucket < ?", p.ID, cutoff); err != nil {
			slog.Error("Failed to prune daily funnel rollups", "error", err, "site_id", p.ID)
			defer func() { _ = tx.Rollback() }()
			continue
		}

		if _, err := tx.ExecContext(ctx, "DELETE FROM funnel_rollups_monthly WHERE site_id = ? AND bucket < ?", p.ID, cutoff); err != nil {
			slog.Error("Failed to prune monthly funnel rollups", "error", err, "site_id", p.ID)
			defer func() { _ = tx.Rollback() }()
			continue
		}

		if _, err := tx.ExecContext(ctx, "DELETE FROM session_rollups_hourly WHERE site_id = ? AND bucket < ?", p.ID, cutoff); err != nil {
			slog.Error("Failed to prune hourly session rollups", "error", err, "site_id", p.ID)
			defer func() { _ = tx.Rollback() }()
			continue
		}

		if _, err := tx.ExecContext(ctx, "DELETE FROM session_rollups_daily WHERE site_id = ? AND bucket < ?", p.ID, cutoff); err != nil {
			slog.Error("Failed to prune daily session rollups", "error", err, "site_id", p.ID)
			defer func() { _ = tx.Rollback() }()
			continue
		}

		if _, err := tx.ExecContext(ctx, "DELETE FROM session_rollups_monthly WHERE site_id = ? AND bucket < ?", p.ID, cutoff); err != nil {
			slog.Error("Failed to prune monthly session rollups", "error", err, "site_id", p.ID)
			defer func() { _ = tx.Rollback() }()
			continue
		}

		if err := tx.Commit(); err != nil {
			slog.Error("Failed to commit deletion", "error", err, "site_id", p.ID)
		} else {
			slog.Info("Retention process completed", "site_id", p.ID, "archive", filename)
		}
	}

	return nil
}
