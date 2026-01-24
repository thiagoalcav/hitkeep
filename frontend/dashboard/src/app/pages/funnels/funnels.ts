import { Component, inject, signal, effect, computed } from '@angular/core';

import { FormsModule } from '@angular/forms';
import { ButtonModule } from 'primeng/button';
import { CardModule } from 'primeng/card';
import { SelectModule } from 'primeng/select';
import { DatePickerModule } from 'primeng/datepicker';
import { DialogModule } from 'primeng/dialog';
import { SiteService } from '../../features/sites/services/site.service';
import { AnalyticsService } from '../../core/services/analytics.service';
import { StatsService } from '../../features/analytics/services/stats.service';
import { FunnelList } from '../../features/analytics/components/funnel-list';
import { MetricList } from '../../features/analytics/components/metric-list';
import { FunnelManager } from '../../features/funnels/components/funnel-manager';
import { FunnelViewer } from '../../features/funnels/components/funnel-viewer';
import { Funnel } from '../../core/models/analytics.types';
import { PageHeader } from '../../core/components/page-header/page-header';
import { PageBreadcrumb, PageBreadcrumbItem } from '../../core/components/page-breadcrumb/page-breadcrumb';
import { SeriesChart, SeriesDefinition, SeriesChartPoint } from '../../features/analytics/components/series-chart';
import { FunnelSeriesPoint } from '../../core/models/analytics.types';
import { KpiCard } from '../../features/analytics/components/kpi-card';
import { RangeToolbar } from '../../core/components/range-toolbar/range-toolbar';
import { finalize } from 'rxjs';

interface RangeSelectEvent {
  value: {
    label: string;
    value: string;
  };
}

type MetricFilterType = 'path' | 'referrer' | 'device' | 'country';
interface MetricFilter {
  type: MetricFilterType;
  value: string;
}

@Component({
  selector: 'app-funnels',
  standalone: true,
  imports: [FormsModule, ButtonModule, CardModule, SelectModule, DatePickerModule, DialogModule, PageHeader, PageBreadcrumb, RangeToolbar, SeriesChart, KpiCard, MetricList, FunnelList, FunnelManager, FunnelViewer],
  templateUrl: './funnels.html',
  styleUrl: './funnels.css'
})
export class Funnels {
  protected siteService = inject(SiteService);
  protected analyticsService = inject(AnalyticsService);
  protected statsService = inject(StatsService);
  
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
  protected isFunnelManagerVisible = signal(false);
  protected isFunnelViewerVisible = signal(false);
  protected selectedFunnel = signal<Funnel | null>(null);
  
  protected funnels = signal<Funnel[]>([]);
  protected loading = signal(false);
  protected funnelsLoaded = signal(false);
  protected isRefreshing = computed(() => this.statsService.isLoading() || this.isFunnelSeriesLoading() || this.loading());
  protected funnelSeries = signal<FunnelSeriesPoint[]>([]);
  protected funnelSeriesChart = computed<SeriesChartPoint[]>(() =>
    this.funnelSeries().map(point => ({
      time: point.time,
      entries: point.entries,
      completions: point.completions
    }))
  );
  protected isFunnelSeriesLoading = signal(false);
  protected activeFunnelFilters = signal<{ id: string; name: string }[]>([]);
  protected activeFilters = signal<{ type: MetricFilterType; value: string }[]>([]);
  protected hasFilters = computed(() => this.activeFilters().length > 0);
  protected filterChips = computed(() => this.activeFilters().map(filter => ({
    ...filter,
    label: this.filterLabel(filter)
  })));
  protected readonly funnelKpis = computed(() => {
    const activeIds = new Set(this.activeFunnelFilters().map(filter => filter.id));
    const funnelsCount = activeIds.size > 0
      ? this.funnels().filter(funnel => activeIds.has(funnel.id)).length
      : this.funnels().length;
    const entries = this.funnelSeries().reduce((sum, point) => sum + point.entries, 0);
    const completions = this.funnelSeries().reduce((sum, point) => sum + point.completions, 0);
    const completionRate = entries > 0 ? (completions / entries) * 100 : 0;

    return [
      {
        label: 'Funnels',
        value: funnelsCount,
        loading: this.loading(),
        valueClass: 'text-2xl xl:text-3xl font-bold'
      },
      {
        label: 'Entries',
        value: entries,
        loading: this.isFunnelSeriesLoading(),
        valueClass: 'text-2xl xl:text-3xl font-bold'
      },
      {
        label: 'Completions',
        value: completions,
        loading: this.isFunnelSeriesLoading(),
        valueClass: 'text-2xl xl:text-3xl font-bold'
      },
      {
        label: 'Completion Rate',
        value: `${completionRate.toFixed(1)}%`,
        loading: this.isFunnelSeriesLoading(),
        valueClass: 'text-2xl xl:text-3xl font-bold'
      }
    ];
  });
  protected readonly funnelSeriesConfig: SeriesDefinition[] = [
    {
      key: 'entries',
      label: 'Entries',
      color: '#6366f1',
      gradientFrom: 'rgba(99, 102, 241, 0.5)',
      gradientTo: 'rgba(99, 102, 241, 0.0)'
    },
    {
      key: 'completions',
      label: 'Completions',
      color: '#14b8a6',
      gradientFrom: 'rgba(20, 184, 166, 0.5)',
      gradientTo: 'rgba(20, 184, 166, 0.0)'
    }
  ];
  protected readonly breadcrumbItems = computed<PageBreadcrumbItem[]>(() => {
    const site = this.siteService.activeSite();
    if (!site) {
      return [{ label: 'Funnels', isCurrent: true }];
    }
    return [
      { label: site.domain, favicon: site, routerLink: '/dashboard' },
      { label: 'Funnels', isCurrent: true }
    ];
  });

  constructor() {
    effect(() => {
      const site = this.siteService.activeSite();
      if (site) {
        this.loadFunnels();
      } else {
        this.funnels.set([]);
        this.funnelsLoaded.set(false);
      }
    });

    effect(() => {
      const site = this.siteService.activeSite();
      const filters = this.activeFunnelFilters();
      const metricFilters = this.activeFilters();
      const dates = this.getCurrentDateRange();
      if (site && dates && this.funnelsLoaded()) {
        const funnelIds = this.getFunnelIdsForFilters();
        if (funnelIds.length === 0 && filters.length === 0) {
          this.statsService.stats.set(null);
          return;
        }
        this.loadFunnelSeries(site.id, dates.from, dates.to, filters.map(filter => filter.id));
        this.statsService.loadStats(
          site.id,
          dates.from,
          dates.to,
          metricFilters,
          [],
          funnelIds
        );
      }
    });
  }

  loadFunnels() {
    const site = this.siteService.activeSite();
    if (!site) return;
    this.loading.set(true);
    this.funnelsLoaded.set(false);
    this.analyticsService.getFunnels(site.id).subscribe({
      next: (data) => {
        this.funnels.set(data);
        this.loading.set(false);
        this.funnelsLoaded.set(true);
      },
      error: () => {
        this.loading.set(false);
        this.funnelsLoaded.set(true);
      }
    });
  }

  protected availableFunnelFilters = computed(() => {
    const selected = new Set(this.activeFunnelFilters().map(filter => filter.id));
    return this.funnels()
      .filter(funnel => !selected.has(funnel.id))
      .map(funnel => ({ label: funnel.name, value: { id: funnel.id, name: funnel.name } }));
  });

  protected addFunnelFilter(filter: { id: string; name: string } | null) {
    if (!filter) return;
    const active = this.activeFunnelFilters();
    if (active.some(existing => existing.id === filter.id)) return;
    this.activeFunnelFilters.set([...active, filter]);
  }

  protected removeFunnelFilter(id: string) {
    this.activeFunnelFilters.update(list => list.filter(item => item.id !== id));
  }

  protected clearFunnelFilters() {
    this.activeFunnelFilters.set([]);
  }

  private getFunnelIdsForFilters(): string[] {
    const active = this.activeFunnelFilters();
    if (active.length > 0) {
      return active.map(filter => filter.id);
    }
    return this.funnels().map(funnel => funnel.id);
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

  private loadFunnelSeries(siteId: string, from: string, to: string, funnelIds: string[]) {
    this.isFunnelSeriesLoading.set(true);
    this.analyticsService.getFunnelTimeseries(siteId, from, to, funnelIds)
      .pipe(finalize(() => this.isFunnelSeriesLoading.set(false)))
      .subscribe({
        next: (data) => this.funnelSeries.set(data ?? []),
        error: () => this.funnelSeries.set([])
      });
  }

  protected onRangeChange(event: RangeSelectEvent) {
    if (event.value.value === 'custom') this.isCustomRangeVisible.set(true);
  }

  protected refreshStats() {
    const site = this.siteService.activeSite();
    const dates = this.getCurrentDateRange();
    if (!site || !dates) return;

    const filters = this.activeFunnelFilters();
    const metricFilters = this.activeFilters();
    const funnelIds = this.getFunnelIdsForFilters();

    if (funnelIds.length === 0 && filters.length === 0) {
      this.statsService.stats.set(null);
      return;
    }

    this.loadFunnelSeries(site.id, dates.from, dates.to, filters.map(filter => filter.id));
    this.statsService.loadStats(
      site.id,
      dates.from,
      dates.to,
      metricFilters,
      [],
      funnelIds
    );
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

  openFunnelManager() {
    this.isFunnelManagerVisible.set(true);
  }

  viewFunnel(funnel: Funnel) {
    this.selectedFunnel.set(funnel);
    this.isFunnelViewerVisible.set(true);
  }

  getDateRange() {
    const range = this.getCurrentDateRange();
    if (range) return range;
    const end = new Date();
    const start = new Date();
    start.setDate(end.getDate() - 30);
    return { from: start.toISOString(), to: end.toISOString() };
  }
}
