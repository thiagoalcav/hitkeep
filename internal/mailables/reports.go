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
	LocaleCode     string
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
	locale, domain, periodLabel, freqLabel, dashURL, settingsURL string,
	current, previous ReportStats,
	dailyPageviews []int,
) mailer.Mailable {
	return &SiteAnalyticsReport{
		LocaleCode:     locale,
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
	return mailer.Translatef(m.LocaleCode, "subject.site_report", m.FreqLabel, m.SiteDomain, m.PeriodLabel)
}

func (m *SiteAnalyticsReport) Template() string { return "site_report.mjml" }

func (m *SiteAnalyticsReport) Data() any { return m }

func (m *SiteAnalyticsReport) Locale() string { return m.LocaleCode }

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
	LocaleCode       string
	PeriodLabel      string
	FreqLabel        string
	SubjectFreqLabel string
	DashURL          string
	SettingsURL      string
	Sites            []DigestSiteEntry
}

func NewAnalyticsDigest(locale, periodLabel, freqLabel, dashURL, settingsURL string, sites []DigestSiteEntry) mailer.Mailable {
	return NewAnalyticsDigestWithSubjectLabel(locale, periodLabel, freqLabel, freqLabel, dashURL, settingsURL, sites)
}

func NewAnalyticsDigestWithSubjectLabel(locale, periodLabel, freqLabel, subjectFreqLabel, dashURL, settingsURL string, sites []DigestSiteEntry) mailer.Mailable {
	return &AnalyticsDigest{
		LocaleCode:       locale,
		PeriodLabel:      periodLabel,
		FreqLabel:        freqLabel,
		SubjectFreqLabel: subjectFreqLabel,
		DashURL:          dashURL,
		SettingsURL:      settingsURL,
		Sites:            sites,
	}
}

func (m *AnalyticsDigest) Subject() string {
	freqLabel := m.SubjectFreqLabel
	if freqLabel == "" {
		freqLabel = m.FreqLabel
	}
	return mailer.Translatef(m.LocaleCode, "subject.analytics_digest", freqLabel, m.PeriodLabel)
}

func (m *AnalyticsDigest) Template() string { return "analytics_digest.mjml" }

func (m *AnalyticsDigest) Data() any { return m }

func (m *AnalyticsDigest) Locale() string { return m.LocaleCode }

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

func LocalizedFrequencyLabel(locale string, freq string) string {
	return mailer.Translate(locale, "freq."+freq)
}

func LocalizedReportFrequencyLabel(locale string, freq string) string {
	return mailer.Translate(locale, "freq.report."+freq)
}

func LocalizedDigestFrequencyLabel(locale string, freq string) string {
	return mailer.Translate(locale, "freq.digest."+freq)
}

func LocalizedDigestSubjectFrequencyLabel(locale string, freq string) string {
	return mailer.Translate(locale, "freq.digest_subject."+freq)
}

func FormatPeriodLabelForLocale(locale string, start, end time.Time) string {
	locale = mailer.NormalizeLocale(locale)
	if start.Month() == end.Month() && start.Year() == end.Year() {
		switch locale {
		case "de":
			return fmt.Sprintf("%d.-%d. %s %d", start.Day(), end.Day(), mailer.MonthName(locale, start.Month(), false), start.Year())
		case "fr", "it":
			return fmt.Sprintf("%d-%d %s %d", start.Day(), end.Day(), mailer.MonthName(locale, start.Month(), true), start.Year())
		case "es":
			return fmt.Sprintf("%d-%d %s %d", start.Day(), end.Day(), mailer.MonthName(locale, start.Month(), true), start.Year())
		default:
			return fmt.Sprintf("%s %d–%d, %d", mailer.MonthName(locale, start.Month(), true), start.Day(), end.Day(), start.Year())
		}
	}

	switch locale {
	case "de":
		return fmt.Sprintf("%d. %s - %d. %s %d", start.Day(), mailer.MonthName(locale, start.Month(), false), end.Day(), mailer.MonthName(locale, end.Month(), false), end.Year())
	case "fr", "it":
		return fmt.Sprintf("%d %s - %d %s %d", start.Day(), mailer.MonthName(locale, start.Month(), true), end.Day(), mailer.MonthName(locale, end.Month(), true), end.Year())
	case "es":
		return fmt.Sprintf("%d %s - %d %s %d", start.Day(), mailer.MonthName(locale, start.Month(), true), end.Day(), mailer.MonthName(locale, end.Month(), true), end.Year())
	default:
		return fmt.Sprintf("%s %d – %s %d, %d",
			mailer.MonthName(locale, start.Month(), true), start.Day(),
			mailer.MonthName(locale, end.Month(), true), end.Day(),
			end.Year(),
		)
	}
}

func FormatSingleDayLabel(locale string, day time.Time) string {
	locale = mailer.NormalizeLocale(locale)
	switch locale {
	case "de":
		return fmt.Sprintf("%d. %s %d", day.Day(), mailer.MonthName(locale, day.Month(), false), day.Year())
	case "fr", "it":
		return fmt.Sprintf("%d %s %d", day.Day(), mailer.MonthName(locale, day.Month(), false), day.Year())
	case "es":
		return fmt.Sprintf("%d %s %d", day.Day(), mailer.MonthName(locale, day.Month(), true), day.Year())
	default:
		return fmt.Sprintf("%s %d, %d", mailer.MonthName(locale, day.Month(), true), day.Day(), day.Year())
	}
}

func FormatMonthYearLabel(locale string, day time.Time) string {
	locale = mailer.NormalizeLocale(locale)
	return fmt.Sprintf("%s %d", mailer.MonthName(locale, day.Month(), false), day.Year())
}
