package worker

import (
	"testing"
	"time"

	"hitkeep/internal/api"
)

func TestReportPeriodLabelDailyUsesSingleDay(t *testing.T) {
	start := time.Date(2026, time.March, 30, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, time.March, 31, 0, 0, 0, 0, time.UTC)

	if got := reportPeriodLabel("en", api.ReportFrequencyDaily, start, end); got != "Mar 30, 2026" {
		t.Fatalf("expected daily label Mar 30, 2026, got %q", got)
	}
}

func TestReportPeriodLabelWeeklyUsesRange(t *testing.T) {
	start := time.Date(2026, time.March, 23, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, time.March, 30, 0, 0, 0, 0, time.UTC)

	if got := reportPeriodLabel("en", api.ReportFrequencyWeekly, start, end); got != "Mar 23–29, 2026" {
		t.Fatalf("expected weekly label Mar 23–29, 2026, got %q", got)
	}
}

func TestReportPeriodLabelMonthlyUsesMonthAndYear(t *testing.T) {
	start := time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC)

	if got := reportPeriodLabel("en", api.ReportFrequencyMonthly, start, end); got != "March 2026" {
		t.Fatalf("expected monthly label March 2026, got %q", got)
	}
}

func TestInclusivePeriodEndUsesExclusiveBoundary(t *testing.T) {
	start := time.Date(2026, time.March, 30, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, time.March, 31, 0, 0, 0, 0, time.UTC)

	got := inclusivePeriodEnd(start, end)
	want := end.Add(-time.Nanosecond)
	if !got.Equal(want) {
		t.Fatalf("expected inclusive end %s, got %s", want.Format(time.RFC3339Nano), got.Format(time.RFC3339Nano))
	}
}
