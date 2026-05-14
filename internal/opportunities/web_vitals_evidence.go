package opportunities

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"hitkeep/internal/api"
	"hitkeep/internal/database"
)

type WebVitalsEvidenceSnapshot struct {
	SiteID  uuid.UUID
	From    time.Time
	To      time.Time
	Summary []api.WebVitalSummaryMetric
	Pages   map[api.WebVitalMetric][]api.WebVitalPageRow
}

func buildWebVitalsEvidenceSnapshot(ctx context.Context, store *database.Store, siteID uuid.UUID, from, to time.Time) (*WebVitalsEvidenceSnapshot, error) {
	if err := validateWebVitalsEvidenceInput(store, siteID); err != nil {
		return nil, err
	}

	summary, err := store.GetWebVitalsSummary(ctx, api.WebVitalsParams{
		SiteID: siteID,
		Start:  from,
		End:    to,
	})
	if err != nil {
		return nil, fmt.Errorf("load web vitals summary: %w", err)
	}
	pages, err := loadWebVitalsEvidencePages(ctx, store, siteID, from, to, summary)
	if err != nil {
		return nil, err
	}

	return &WebVitalsEvidenceSnapshot{
		SiteID:  siteID,
		From:    from,
		To:      to,
		Summary: append([]api.WebVitalSummaryMetric(nil), summary...),
		Pages:   pages,
	}, nil
}

func validateWebVitalsEvidenceInput(store *database.Store, siteID uuid.UUID) error {
	if store == nil {
		return fmt.Errorf("analytics store is required")
	}
	if siteID == uuid.Nil {
		return fmt.Errorf("site id is required")
	}
	return nil
}

func loadWebVitalsEvidencePages(ctx context.Context, store *database.Store, siteID uuid.UUID, from, to time.Time, summary []api.WebVitalSummaryMetric) (map[api.WebVitalMetric][]api.WebVitalPageRow, error) {
	pages := make(map[api.WebVitalMetric][]api.WebVitalPageRow, len(summary))
	for _, metric := range summary {
		rows, err := store.GetWebVitalsPages(ctx, api.WebVitalsParams{
			SiteID: siteID,
			Start:  from,
			End:    to,
			Metric: metric.Metric,
			Limit:  5,
		})
		if err != nil {
			return nil, fmt.Errorf("load web vitals pages for %s: %w", metric.Metric, err)
		}
		pages[metric.Metric] = rows
	}
	return pages, nil
}
