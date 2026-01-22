import { Component, inject, signal, effect, computed } from '@angular/core';

import { FormsModule } from '@angular/forms';
import { ButtonModule } from 'primeng/button';
import { CardModule } from 'primeng/card';
import { SelectModule } from 'primeng/select';
import { DatePickerModule } from 'primeng/datepicker';
import { DialogModule } from 'primeng/dialog';
import { SiteService } from '../../features/sites/services/site.service';
import { StatsService } from '../../features/analytics/services/stats.service';
import { AnalyticsService } from '../../core/services/analytics.service';
import { GoalList } from '../../features/analytics/components/goal-list';
import { MetricList } from '../../features/analytics/components/metric-list';
import { GoalManager } from '../../features/goals/components/goal-manager';
import { PageHeader } from '../../core/components/page-header/page-header';
import { PageBreadcrumb, PageBreadcrumbItem } from '../../core/components/page-breadcrumb/page-breadcrumb';
import { SeriesChart, SeriesDefinition, SeriesChartPoint } from '../../features/analytics/components/series-chart';
import { Goal, GoalSeriesPoint } from '../../core/models/analytics.types';
import { KpiCard } from '../../features/analytics/components/kpi-card';
import { finalize } from 'rxjs';

interface RangeSelectEvent {
  value: {
    label: string;
    value: string;
  };
}

type MetricFilterType = 'path' | 'referrer' | 'device' | 'country';
type MetricFilter = {
  type: MetricFilterType;
  value: string;
};

@Component({
  selector: 'app-goals',
  standalone: true,
  imports: [FormsModule, ButtonModule, CardModule, SelectModule, DatePickerModule, DialogModule, PageHeader, PageBreadcrumb, SeriesChart, KpiCard, MetricList, GoalList, GoalManager],
  templateUrl: './goals.html',
  styleUrl: './goals.css'
})
export class Goals {
  protected siteService = inject(SiteService);
  protected statsService = inject(StatsService);
  private analyticsService = inject(AnalyticsService);
  
  protected timeRanges = [
    {label: 'Last 24 Hours', value: '24h'},
    {label: 'Last 7 Days', value: '7d'},
    {label: 'Last 30 Days', value: '30d'},
    {label: 'Last Year', value: '1y'},
    {label: 'Custom Range', value: 'custom'}
  ];
  protected selectedRange = signal(this.timeRanges[2]);
  protected isCustomRangeVisible = signal(false);
  protected customRangeDates = signal<Date[] | null>(null);
  protected isShortRange = computed(() => {
    if (this.selectedRange().value === '24h') return true;
    if (this.selectedRange().value === 'custom' && this.customRangeDates()) {
      const d = this.customRangeDates()!;
      if (d.length === 2 && d[0] && d[1]) {
        const diff = d[1].getTime() - d[0].getTime();
        return diff < 48 * 60 * 60 * 1000;
      }
    }
    return false;
  });
  protected isGoalManagerVisible = signal(false);
  protected goals = signal<Goal[]>([]);
  protected goalsLoading = signal(false);
  protected goalSeries = signal<GoalSeriesPoint[]>([]);
  protected goalSeriesChart = computed<SeriesChartPoint[]>(() =>
    this.goalSeries().map(point => ({
      time: point.time,
      conversions: point.conversions
    }))
  );
  protected isGoalSeriesLoading = signal(false);
  protected activeGoalFilters = signal<Array<{ id: string; name: string }>>([]);
  protected activeFilters = signal<Array<{ type: MetricFilterType; value: string }>>([]);
  protected hasFilters = computed(() => this.activeFilters().length > 0);
  protected filterChips = computed(() => this.activeFilters().map(filter => ({
    ...filter,
    label: this.filterLabel(filter)
  })));
  protected readonly goalKpis = computed(() => {
    const activeIds = new Set(this.activeGoalFilters().map(filter => filter.id));
    const goals = this.goals();
    const totalGoals = activeIds.size > 0 ? goals.filter(goal => activeIds.has(goal.id)).length : goals.length;
    const totalConversions = this.goalSeries().reduce((sum, point) => sum + point.conversions, 0);
    const uniqueSessions = this.statsService.stats()?.unique_sessions ?? 0;
    const conversionRate = uniqueSessions > 0 ? (totalConversions / uniqueSessions) * 100 : 0;

    return [
      {
        label: 'Total Goals',
        value: totalGoals,
        loading: this.statsService.isLoading(),
        valueClass: 'text-2xl xl:text-3xl font-bold'
      },
      {
        label: 'Conversions',
        value: totalConversions,
        loading: this.statsService.isLoading(),
        valueClass: 'text-2xl xl:text-3xl font-bold'
      },
      {
        label: 'Conversion Rate',
        value: `${conversionRate.toFixed(1)}%`,
        loading: this.statsService.isLoading(),
        valueClass: 'text-2xl xl:text-3xl font-bold'
      },
      {
        label: 'Unique Sessions',
        value: uniqueSessions,
        loading: this.statsService.isLoading(),
        valueClass: 'text-2xl xl:text-3xl font-bold'
      }
    ];
  });
  protected readonly goalSeriesConfig: SeriesDefinition[] = [
    {
      key: 'conversions',
      label: 'Conversions',
      color: '#6366f1',
      gradientFrom: 'rgba(99, 102, 241, 0.5)',
      gradientTo: 'rgba(99, 102, 241, 0.0)'
    }
  ];
  protected readonly breadcrumbItems = computed<PageBreadcrumbItem[]>(() => {
    const site = this.siteService.activeSite();
    if (!site) {
      return [{ label: 'Goals', isCurrent: true }];
    }
    return [
      { label: site.domain, favicon: site, routerLink: '/dashboard' },
      { label: 'Goals', isCurrent: true }
    ];
  });

  constructor() {
    effect(() => {
      const site = this.siteService.activeSite();
      if (site) {
        this.loadGoals(site.id);
      } else {
        this.goals.set([]);
      }
    });

    effect(() => {
      const site = this.siteService.activeSite();
      const filters = this.activeGoalFilters();
      const metricFilters = this.activeFilters();
      const range = this.selectedRange();
      const dates = this.getCurrentDateRange();
      if (site && dates) {
        const goalIds = this.getGoalIdsForFilters();
        this.statsService.loadStats(
          site.id,
          dates.from,
          dates.to,
          metricFilters,
          goalIds,
          []
        );
        this.loadGoalSeries(site.id, dates.from, dates.to, goalIds);
      }
    });
  }

  openGoalManager() {
    this.isGoalManagerVisible.set(true);
  }

  refreshStats() {
    const site = this.siteService.activeSite();
    const dates = this.getCurrentDateRange();
    if (site && dates) {
        const goalIds = this.getGoalIdsForFilters();
        this.statsService.loadStats(
          site.id,
          dates.from,
          dates.to,
          this.activeFilters(),
          goalIds,
          []
        );
        this.loadGoalSeries(site.id, dates.from, dates.to, goalIds);
    }
  }

  protected availableGoalFilters = computed(() => {
    const selected = new Set(this.activeGoalFilters().map(filter => filter.id));
    return this.goals()
      .filter(goal => !selected.has(goal.id))
      .map(goal => ({ label: goal.name, value: { id: goal.id, name: goal.name } }));
  });

  protected addGoalFilter(filter: { id: string; name: string } | null) {
    if (!filter) return;
    const active = this.activeGoalFilters();
    if (active.some(existing => existing.id === filter.id)) return;
    this.activeGoalFilters.set([...active, filter]);
  }

  protected removeGoalFilter(id: string) {
    this.activeGoalFilters.update(list => list.filter(item => item.id !== id));
  }

  protected clearGoalFilters() {
    this.activeGoalFilters.set([]);
  }

  private getGoalIdsForFilters(): string[] {
    const active = this.activeGoalFilters();
    if (active.length > 0) {
      return active.map(filter => filter.id);
    }
    return this.goals().map(goal => goal.id);
  }

  private loadGoals(siteId: string) {
    this.goalsLoading.set(true);
    this.analyticsService.getGoals(siteId)
      .pipe(finalize(() => this.goalsLoading.set(false)))
      .subscribe({
        next: (data) => this.goals.set(data ?? []),
        error: () => this.goals.set([])
      });
  }

  protected applyMetricFilter(type: MetricFilterType, metric: { name: string }) {
    if (!metric.name) return;
    this.activeFilters.update(filters => {
      const existingIndex = filters.findIndex(filter => filter.type === type);
      if (existingIndex >= 0) {
        const existing = filters[existingIndex];
        if (existing.value === metric.name) {
          return filters.filter((_, idx) => idx !== existingIndex);
        }
        const next = [...filters];
        next[existingIndex] = { type, value: metric.name };
        return next;
      }
      return [...filters, { type, value: metric.name }];
    });
  }

  protected clearFilter() {
    this.activeFilters.set([]);
  }

  protected removeFilter(type: MetricFilterType, value: string) {
    this.activeFilters.update(filters => filters.filter(filter => !(filter.type === type && filter.value === value)));
  }

  protected activeFilterValue(type: MetricFilterType): string | null {
    return this.activeFilters().find(filter => filter.type === type)?.value ?? null;
  }

  private filterLabel(filter: MetricFilter): string {
    switch (filter.type) {
      case 'path':
        return `Page: ${filter.value}`;
      case 'referrer':
        return `Source: ${filter.value}`;
      case 'device':
        return `Device: ${filter.value}`;
      case 'country':
        return `Country: ${filter.value}`;
      default:
        return `${filter.type}: ${filter.value}`;
    }
  }

  private loadGoalSeries(siteId: string, from: string, to: string, goalIds: string[]) {
    this.isGoalSeriesLoading.set(true);
    this.analyticsService.getGoalTimeseries(siteId, from, to, goalIds)
      .pipe(finalize(() => this.isGoalSeriesLoading.set(false)))
      .subscribe({
        next: (data) => this.goalSeries.set(data ?? []),
        error: () => this.goalSeries.set([])
      });
  }

  protected onRangeChange(event: RangeSelectEvent) {
    if (event.value.value === 'custom') this.isCustomRangeVisible.set(true);
  }

  protected applyCustomRange() {
    this.isCustomRangeVisible.set(false);
    this.selectedRange.set({...this.selectedRange()});
  }

  protected getCurrentDateRange() {
    const range = this.selectedRange();
    const end = new Date();
    const start = new Date();

    if (range.value === 'custom') {
      const d = this.customRangeDates();
      if (d && d.length === 2 && d[0] && d[1]) {
        return {from: d[0].toISOString(), to: d[1].toISOString()};
      }
      return null;
    }

    switch (range.value) {
      case '24h':
        start.setHours(end.getHours() - 24);
        break;
      case '7d':
        start.setDate(end.getDate() - 7);
        break;
      case '30d':
        start.setDate(end.getDate() - 30);
        break;
      case '1y':
        start.setFullYear(end.getFullYear() - 1);
        break;
    }
    return {from: start.toISOString(), to: end.toISOString()};
  }
}
