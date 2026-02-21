package mailables

import (
	"fmt"
	"time"

	"hitkeep/internal/api"
	"hitkeep/internal/mailer"
)

// ReportStats holds the aggregated KPIs for one analytics period.
type ReportStats struct {
	Pageviews          int
	Visitors           int
	BounceRate         float64
	AvgSessionDuration float64
	TopPages           []api.MetricStat
	TopReferrers       []api.MetricStat
	Goals              []api.GoalStats
}

// SiteAnalyticsReport implements mailer.Mailable for per-site reports.
type SiteAnalyticsReport struct {
	SiteDomain     string
	PeriodLabel    string // e.g. "Feb 13 – Feb 19, 2026"
	FreqLabel      string // e.g. "Weekly"
	DashURL        string
	SettingsURL    string
	Current        ReportStats
	Previous       ReportStats
	DailyPageviews []int // one value per day in the current period, for the sparkline
}

func NewSiteAnalyticsReport(
	domain, periodLabel, freqLabel, dashURL, settingsURL string,
	current, previous ReportStats,
	dailyPageviews []int,
) mailer.Mailable {
	return &SiteAnalyticsReport{
		SiteDomain:     domain,
		PeriodLabel:    periodLabel,
		FreqLabel:      freqLabel,
		DashURL:        dashURL,
		SettingsURL:    settingsURL,
		Current:        current,
		Previous:       previous,
		DailyPageviews: dailyPageviews,
	}
}

func (m *SiteAnalyticsReport) Subject() string {
	return fmt.Sprintf("%s Report for %s – %s", m.FreqLabel, m.SiteDomain, m.PeriodLabel)
}

func (m *SiteAnalyticsReport) Template() string { return "site_report.mjml" }

func (m *SiteAnalyticsReport) Data() any { return m }

// DigestSiteEntry is one row in the consolidated digest email.
type DigestSiteEntry struct {
	Domain        string
	DashURL       string
	Pageviews     int
	PrevPageviews int
	Visitors      int
	PrevVisitors  int
}

// AnalyticsDigest implements mailer.Mailable for consolidated multi-site digests.
type AnalyticsDigest struct {
	PeriodLabel string
	FreqLabel   string
	DashURL     string
	SettingsURL string
	Sites       []DigestSiteEntry
}

func NewAnalyticsDigest(periodLabel, freqLabel, dashURL, settingsURL string, sites []DigestSiteEntry) mailer.Mailable {
	return &AnalyticsDigest{
		PeriodLabel: periodLabel,
		FreqLabel:   freqLabel,
		DashURL:     dashURL,
		SettingsURL: settingsURL,
		Sites:       sites,
	}
}

func (m *AnalyticsDigest) Subject() string {
	return fmt.Sprintf("Your %s Analytics Digest – %s", m.FreqLabel, m.PeriodLabel)
}

func (m *AnalyticsDigest) Template() string { return "analytics_digest.mjml" }

func (m *AnalyticsDigest) Data() any { return m }

// FormatPeriodLabel formats a human-readable period label for the given start/end times.
func FormatPeriodLabel(start, end time.Time) string {
	if start.Month() == end.Month() && start.Year() == end.Year() {
		return fmt.Sprintf("%s %d–%d, %d", start.Month().String()[:3], start.Day(), end.Day(), start.Year())
	}
	return fmt.Sprintf("%s %d – %s %d, %d",
		start.Month().String()[:3], start.Day(),
		end.Month().String()[:3], end.Day(),
		end.Year(),
	)
}
