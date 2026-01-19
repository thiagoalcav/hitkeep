package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"hitkeep/internal/database"
)

type RollupBackfillWorker struct {
	store *database.Store
}

func NewRollupBackfillWorker(store *database.Store) *RollupBackfillWorker {
	return &RollupBackfillWorker{
		store: store,
	}
}

func (w *RollupBackfillWorker) Start(ctx context.Context) {
	go func() {
		time.Sleep(10 * time.Second)
		if err := w.Run(ctx); err != nil {
			slog.Error("Initial rollup backfill failed", "error", err)
		}
	}()

	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := w.Run(ctx); err != nil {
				slog.Error("Rollup backfill failed", "error", err)
			}
		}
	}
}

func (w *RollupBackfillWorker) Run(ctx context.Context) error {
	rows, err := w.store.DB().QueryContext(ctx, "SELECT id FROM sites")
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var siteID uuid.UUID
		if err := rows.Scan(&siteID); err != nil {
			slog.Warn("Failed to scan site for rollup backfill", "error", err)
			continue
		}
		if err := w.store.BackfillRollups(ctx, siteID); err != nil {
			slog.Warn("Rollup backfill failed for site", "error", err, "site_id", siteID)
		}
	}

	return rows.Err()
}
