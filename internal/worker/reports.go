package worker

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"hitkeep/internal/api"
	"hitkeep/internal/database"
	"hitkeep/internal/mailables"
	"hitkeep/internal/mailer"
)

// ReportWorker sends scheduled analytics emails (daily / weekly / monthly).
type ReportWorker struct {
	store  *database.Store
	mailer *mailer.Mailer
	pubURL string
}

// NewReportWorker creates a ReportWorker. pubURL is used to build dashboard deep-links.
func NewReportWorker(store *database.Store, m *mailer.Mailer, pubURL string) *ReportWorker {
	return &ReportWorker{
		store:  store,
		mailer: m,
		pubURL: pubURL,
	}
}

// Start waits until 08:00 UTC, then ticks every 24 hours.
func (w *ReportWorker) Start(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("ReportWorker panicked", "error", r)
		}
	}()

	now := time.Now().UTC()
	next := time.Date(now.Year(), now.Month(), now.Day(), 8, 0, 0, 0, time.UTC)
	if !next.After(now) {
		next = next.Add(24 * time.Hour)
	}

	timer := time.NewTimer(time.Until(next))
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return
	case <-timer.C:
	}

	w.Run(ctx)

	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.Run(ctx)
		}
	}
}

// Run sends any reports that are due today.
func (w *ReportWorker) Run(ctx context.Context) {
	if w.mailer == nil {
		slog.Debug("ReportWorker: mailer not configured, skipping scheduled reports")
		return
	}

	now := time.Now().UTC()

	w.processSiteReports(ctx, api.ReportFrequencyDaily, now)
	w.processDigests(ctx, api.ReportFrequencyDaily, now)

	if now.Weekday() == time.Monday {
		w.processSiteReports(ctx, api.ReportFrequencyWeekly, now)
		w.processDigests(ctx, api.ReportFrequencyWeekly, now)
	}

	if now.Day() == 1 {
		w.processSiteReports(ctx, api.ReportFrequencyMonthly, now)
		w.processDigests(ctx, api.ReportFrequencyMonthly, now)
	}
}

func (w *ReportWorker) processSiteReports(ctx context.Context, freq api.ReportFrequency, now time.Time) {
	pending, err := w.store.GetPendingSiteReports(ctx, freq)
	if err != nil {
		slog.Error("ReportWorker: failed to load pending site reports", "freq", freq, "error", err)
		return
	}

	start, end, prevStart, prevEnd := periodBounds(freq, now)
	freqLabel := freqLabelFor(freq)
	periodLabel := mailables.FormatPeriodLabel(start, end.Add(-time.Second))

	for _, p := range pending {
		if ctx.Err() != nil {
			slog.Warn("ReportWorker: context cancelled, halting site reports")
			return
		}

		curParams := api.AnalyticsParams{
			SiteID: p.SiteID,
			UserID: p.UserID,
			Start:  start,
			End:    end,
		}
		prevParams := api.AnalyticsParams{
			SiteID: p.SiteID,
			UserID: p.UserID,
			Start:  prevStart,
			End:    prevEnd,
		}

		curStats, err := w.store.GetSiteStats(ctx, curParams)
		if err != nil {
			slog.Error("ReportWorker: failed to get current site stats", "site_id", p.SiteID, "error", err)
			continue
		}

		prevStats, err := w.store.GetSiteStats(ctx, prevParams)
		if err != nil {
			slog.Error("ReportWorker: failed to get previous site stats", "site_id", p.SiteID, "error", err)
			continue
		}

		topPages := curStats.TopPages
		if len(topPages) > 5 {
			topPages = topPages[:5]
		}
		topRefs := curStats.TopReferrers
		if len(topRefs) > 5 {
			topRefs = topRefs[:5]
		}

		cur := mailables.ReportStats{
			Pageviews:          curStats.TotalPageviews,
			Visitors:           curStats.UniqueSessions,
			BounceRate:         curStats.BounceRate,
			AvgSessionDuration: curStats.AvgSessionDuration,
			TopPages:           topPages,
			TopReferrers:       topRefs,
			Goals:              curStats.Goals,
		}
		prev := mailables.ReportStats{
			Pageviews: prevStats.TotalPageviews,
			Visitors:  prevStats.UniqueSessions,
		}

		dailyPVs, err := w.store.GetDailyPageviewsForPeriod(ctx, p.SiteID, start, end)
		if err != nil {
			slog.Warn("ReportWorker: could not fetch daily pageviews for chart", "site_id", p.SiteID, "error", err)
			dailyPVs = nil
		}

		dashURL := fmt.Sprintf("%s/dashboard", w.pubURL)
		settingsURL := fmt.Sprintf("%s/settings", w.pubURL)
		report := mailables.NewSiteAnalyticsReport(p.Domain, periodLabel, freqLabel, dashURL, settingsURL, cur, prev, dailyPVs)

		if err := w.mailer.Send(p.UserEmail, report); err != nil {
			slog.Error("ReportWorker: failed to send site report", "email", p.UserEmail, "site", p.Domain, "error", err)
		} else {
			slog.Info("ReportWorker: sent site report", "email", p.UserEmail, "site", p.Domain, "freq", freq)
		}
	}
}

func (w *ReportWorker) processDigests(ctx context.Context, freq api.ReportFrequency, now time.Time) {
	pending, err := w.store.GetPendingDigests(ctx, freq)
	if err != nil {
		slog.Error("ReportWorker: failed to load pending digests", "freq", freq, "error", err)
		return
	}

	start, end, prevStart, prevEnd := periodBounds(freq, now)
	freqLabel := freqLabelFor(freq)
	periodLabel := mailables.FormatPeriodLabel(start, end.Add(-time.Second))

	for _, p := range pending {
		if ctx.Err() != nil {
			slog.Warn("ReportWorker: context cancelled, halting site reports")
			return
		}

		var entries []mailables.DigestSiteEntry

		for _, site := range p.Sites {
			curParams := api.AnalyticsParams{
				SiteID: site.SiteID,
				UserID: p.UserID,
				Start:  start,
				End:    end,
			}
			prevParams := api.AnalyticsParams{
				SiteID: site.SiteID,
				UserID: p.UserID,
				Start:  prevStart,
				End:    prevEnd,
			}

			curStats, err := w.store.GetSiteStats(ctx, curParams)
			if err != nil {
				slog.Error("ReportWorker: failed to get digest site stats", "site_id", site.SiteID, "error", err)
				continue
			}

			prevStats, err := w.store.GetSiteStats(ctx, prevParams)
			if err != nil {
				slog.Error("ReportWorker: failed to get digest prev site stats", "site_id", site.SiteID, "error", err)
				continue
			}

			entries = append(entries, mailables.DigestSiteEntry{
				Domain:        site.Domain,
				DashURL:       fmt.Sprintf("%s/dashboard", w.pubURL),
				Pageviews:     curStats.TotalPageviews,
				PrevPageviews: prevStats.TotalPageviews,
				Visitors:      curStats.UniqueSessions,
				PrevVisitors:  prevStats.UniqueSessions,
			})
		}

		if len(entries) == 0 {
			continue
		}

		dashURL := fmt.Sprintf("%s/dashboard", w.pubURL)
		settingsURL := fmt.Sprintf("%s/settings", w.pubURL)
		digest := mailables.NewAnalyticsDigest(periodLabel, freqLabel, dashURL, settingsURL, entries)

		if err := w.mailer.Send(p.UserEmail, digest); err != nil {
			slog.Error("ReportWorker: failed to send digest", "email", p.UserEmail, "error", err)
		} else {
			slog.Info("ReportWorker: sent digest", "email", p.UserEmail, "sites", len(entries), "freq", freq)
		}
	}
}

// periodBounds returns [start, end) for the current period and [prevStart, prevEnd) for the previous period.
// 'now' is assumed to be 08:00 UTC on the dispatch day.
func periodBounds(freq api.ReportFrequency, now time.Time) (start, end, prevStart, prevEnd time.Time) {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	switch freq {
	case api.ReportFrequencyDaily:
		// Yesterday 00:00 – yesterday 23:59:59
		end = today
		start = today.AddDate(0, 0, -1)
		prevEnd = start
		prevStart = start.AddDate(0, 0, -1)

	case api.ReportFrequencyWeekly:
		// Last week Mon 00:00 – Sun 23:59:59 (today is Monday)
		end = today
		start = today.AddDate(0, 0, -7)
		prevEnd = start
		prevStart = start.AddDate(0, 0, -7)

	case api.ReportFrequencyMonthly:
		// Previous calendar month; today is the 1st.
		end = today
		start = time.Date(today.Year(), today.Month()-1, 1, 0, 0, 0, 0, time.UTC)
		if today.Month() == time.January {
			start = time.Date(today.Year()-1, time.December, 1, 0, 0, 0, 0, time.UTC)
		}
		prevEnd = start
		prevStart = time.Date(start.Year(), start.Month()-1, 1, 0, 0, 0, 0, time.UTC)
		if start.Month() == time.January {
			prevStart = time.Date(start.Year()-1, time.December, 1, 0, 0, 0, 0, time.UTC)
		}
	}
	return
}

func freqLabelFor(freq api.ReportFrequency) string {
	switch freq {
	case api.ReportFrequencyDaily:
		return "Daily"
	case api.ReportFrequencyWeekly:
		return "Weekly"
	case api.ReportFrequencyMonthly:
		return "Monthly"
	default:
		return string(freq)
	}
}
