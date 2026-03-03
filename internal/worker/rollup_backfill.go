package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"hitkeep/internal/database"
)

type RollupBackfillWorker struct {
	tenantMgr *database.TenantStoreManager
}

func NewRollupBackfillWorker(tenantMgr *database.TenantStoreManager) *RollupBackfillWorker {
	return &RollupBackfillWorker{
		tenantMgr: tenantMgr,
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
	shared := w.tenantMgr.Shared()

	rows, err := shared.DB().QueryContext(ctx, "SELECT id FROM sites")
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

		tenantStore, _, err := w.tenantMgr.ResolveSiteStore(ctx, siteID)
		if err != nil {
			slog.Warn("Failed to resolve tenant store for rollup backfill", "error", err, "site_id", siteID)
			continue
		}

		if err := tenantStore.BackfillRollups(ctx, siteID); err != nil {
			slog.Warn("Rollup backfill failed for site", "error", err, "site_id", siteID)
		}
	}

	return rows.Err()
}
