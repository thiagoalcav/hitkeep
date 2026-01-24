import { Component, effect, inject, signal, computed, ChangeDetectionStrategy } from '@angular/core';
import { CommonModule, DatePipe, DecimalPipe, NgOptimizedImage } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { debounceTime, distinctUntilChanged, Subject } from 'rxjs';
// PrimeNG
import { CardModule } from 'primeng/card';
import { TableModule, TableLazyLoadEvent } from 'primeng/table';
import { SelectModule } from 'primeng/select';
import { ButtonModule } from 'primeng/button';
import { IconFieldModule } from 'primeng/iconfield';
import { InputIconModule } from 'primeng/inputicon';
import { InputTextModule } from 'primeng/inputtext';
import { SkeletonModule } from 'primeng/skeleton';
import { DialogModule } from 'primeng/dialog';
import { DatePickerModule } from 'primeng/datepicker';
// Features
import { SiteService } from '../../features/sites/services/site.service';
import { StatsService } from '../../features/analytics/services/stats.service';
import { HitService } from '../../features/hits/services/hit.service';
import { TrafficChart } from '../../features/analytics/components/traffic-chart';
import { MetricList } from '../../features/analytics/components/metric-list';
import { GoalList } from '../../features/analytics/components/goal-list';
import { FunnelList } from '../../features/analytics/components/funnel-list';
import { FunnelManager } from '../../features/funnels/components/funnel-manager';
import { FunnelViewer } from '../../features/funnels/components/funnel-viewer';
import { Funnel } from '../../core/models/analytics.types';
import { MetricStat } from '../../core/models/analytics.types';
import { PageHeader } from '../../core/components/page-header/page-header';
import { PageBreadcrumb, PageBreadcrumbItem } from '../../core/components/page-breadcrumb/page-breadcrumb';
import { KpiCard } from '../../features/analytics/components/kpi-card';
import { ShareService } from '../../core/services/share.service';
import { RangeToolbar } from '../../core/components/range-toolbar/range-toolbar';

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
interface KpiCardData {
  label: string;
  value: number | string;
  loading: boolean;
  valueClass: string;
}
@Component({
  selector: 'app-dashboard',
  standalone: true,
  imports: [
    CommonModule, FormsModule, DatePipe,
    CardModule, TableModule, SelectModule, ButtonModule,
    IconFieldModule, InputIconModule, InputTextModule,
    SkeletonModule, DialogModule, DatePickerModule,
    PageHeader,
    PageBreadcrumb,
    RangeToolbar,
    KpiCard,
    TrafficChart, MetricList, GoalList, FunnelList, FunnelManager, FunnelViewer, NgOptimizedImage
  ],
  templateUrl: './dashboard.html',
  styleUrl: './dashboard.css',
  changeDetection: ChangeDetectionStrategy.OnPush,
  providers: [DatePipe, DecimalPipe]
})
export class Dashboard {
  protected siteService = inject(SiteService);
  protected statsService = inject(StatsService);
  protected hitService = inject(HitService);
  private shareService = inject(ShareService);
  private datePipe = inject(DatePipe);
  private decimalPipe = inject(DecimalPipe);
  protected timeRanges = [
    {label: 'Last 24 Hours', value: '24h'},
    {label: 'Last 7 Days', value: '7d'},
    {label: 'Last 30 Days', value: '30d'},
    {label: 'Last Year', value: '1y'},
    {label: 'Custom Range', value: 'custom'}
  ];
  protected selectedRange = signal(this.timeRanges[2]);
  private readonly autoRefreshIntervalMs = 30000;
  protected isShareMode = computed(() => this.shareService.isShareMode());
  protected isCustomRangeVisible = signal(false);
  protected customRangeDates = signal<Date[] | null>(null);
  protected showFunnelManager = signal(false);
  protected showFunnelViewer = signal(false);
  protected selectedFunnelId = signal<string | null>(null);
  protected funnelDateRange = computed(() => this.getCurrentDateRange());
  protected siteDomain = computed(() => this.siteService.activeSite()?.domain ?? null);
  protected siteFaviconUrl = computed(() => {
    const domain = this.siteDomain();
    return domain ? `/api/favicon/${encodeURIComponent(domain)}` : '';
  });
  protected activeFilters = signal<MetricFilter[]>([]);
  protected hasFilters = computed(() => this.activeFilters().length > 0);
  protected filterChips = computed(() => this.activeFilters().map(filter => ({
    ...filter,
    label: this.filterLabel(filter)
  })));
  protected exportUrl = computed(() => {
    const shareToken = this.shareService.token();
    const site = this.siteService.activeSite();
    const dates = this.getCurrentDateRange();
    if (!site || !dates) return '';

    const params = new URLSearchParams({
      from: dates.from,
      to: dates.to,
    });
    for (const filter of this.activeFilters()) {
      params.append('filter', `${filter.type}:${filter.value}`);
    }
    if (this.isShareMode() && shareToken) {
      return `/api/share/${encodeURIComponent(shareToken)}/sites/${site.id}/hits/export?${params.toString()}`;
    }
    return `/api/sites/${site.id}/hits/export?${params.toString()}`;
  });
  protected readonly kpiCards = computed<KpiCardData[]>(() => {
    const stats = this.statsService.stats();
    const loading = this.statsService.isLoading();
    const baseClass = 'text-2xl xl:text-3xl font-bold';
    const liveVisitors = stats?.live_visitors ?? 0;
    const bounceValue = this.decimalPipe.transform(stats?.bounce_rate ?? 0, '1.0-1') ?? '0';
    const pagesValue = this.decimalPipe.transform(stats?.pages_per_session ?? 0, '1.1-2') ?? '0';

    return [
      {
        label: 'Live Visitors',
        value: liveVisitors,
        loading,
        valueClass: liveVisitors > 0 ? `${baseClass} text-green-600 dark:text-green-400 animate-pulse` : baseClass
      },
      {
        label: 'Pageviews',
        value: stats?.total_pageviews ?? 0,
        loading,
        valueClass: baseClass
      },
      {
        label: 'Unique Sessions',
        value: stats?.unique_sessions ?? 0,
        loading,
        valueClass: baseClass
      },
      {
        label: 'Bounce Rate',
        value: `${bounceValue}%`,
        loading,
        valueClass: baseClass
      },
      {
        label: 'Avg. Duration',
        value: this.formatDuration(stats?.avg_session_duration || 0),
        loading,
        valueClass: baseClass
      },
      {
        label: 'Pages / Session',
        value: pagesValue,
        loading,
        valueClass: baseClass
      }
    ];
  });
  protected readonly breadcrumbItems = computed<PageBreadcrumbItem[]>(() => {
    const site = this.siteService.activeSite();
    if (!site) {
      return [{ label: 'Overview', isCurrent: true }];
    }
    return [{ label: site.domain, favicon: site, isCurrent: true }];
  });

  private searchSubject = new Subject<string>();
  protected searchQuery = signal('');
  private lastTableEvent: TableLazyLoadEvent | null = null;
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
  protected chartTitle = computed(() => {
    const range = this.selectedRange();

    if (range.value !== 'custom') {
      return `Traffic: ${range.label}`;
    }

    const dates = this.customRangeDates();
    if (dates && dates.length === 2 && dates[0] && dates[1]) {
      const start = this.datePipe.transform(dates[0], 'MMM d');
      const end = this.datePipe.transform(dates[1], 'MMM d, y');
      return `Traffic: ${start} - ${end}`;
    }

    return 'Traffic Overview';
  });
  constructor() {
    this.searchSubject.pipe(debounceTime(400), distinctUntilChanged())
      .subscribe(q => {
        this.searchQuery.set(q);
        this.refreshHits();
      });

    effect(() => {
      const site = this.siteService.activeSite();
      const dates = this.getCurrentDateRange();
      if (site && dates) {
        this.loadStatsForCurrentRange();
        this.refreshHits();
      }
    });

    effect((onCleanup) => {
      const site = this.siteService.activeSite();
      const dates = this.getCurrentDateRange();
      if (!site || !dates) return;
      const timerId = setInterval(() => this.refreshStatsOnly(), this.autoRefreshIntervalMs);
      onCleanup(() => clearInterval(timerId));
    });
  }

  refreshAll() {
    this.loadStatsForCurrentRange();
    this.refreshHits();
  }

  onSearch(event: Event) {
    this.searchSubject.next((event.target as HTMLInputElement).value);
  }

  loadHits(event: TableLazyLoadEvent) {
    this.lastTableEvent = event;
    const site = this.siteService.activeSite();
    const dates = this.getCurrentDateRange();
    if (!site || !dates) return;
    const filters = this.activeFilters();

    const rows = event.rows || 10;
    const first = event.first || 0;
    const page = (first / rows) + 1;

    this.hitService.loadHits(
      site.id,
      dates.from,
      dates.to,
      page,
      rows,
      event.sortField as string,
      event.sortOrder === 1 ? 'asc' : 'desc',
      this.searchQuery(),
      filters
    );
  }

  private refreshHits() {
    if (this.lastTableEvent) {
      this.lastTableEvent.first = 0;
      this.loadHits(this.lastTableEvent);
    }
  }

  private refreshStatsOnly() {
    if (this.statsService.isLoading()) return;
    this.loadStatsForCurrentRange();
  }

  private loadStatsForCurrentRange() {
    const site = this.siteService.activeSite();
    const dates = this.getCurrentDateRange();
    const filters = this.activeFilters();
    if (!site || !dates) return;
    this.statsService.loadStats(site.id, dates.from, dates.to, filters);
  }

  onRangeChange(event: RangeSelectEvent) {
    if (event.value.value === 'custom') this.isCustomRangeVisible.set(true);
  }

  applyCustomRange() {
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

  protected formatDuration(seconds: number): string {
    if (!seconds) return '0s';
    const m = Math.floor(seconds / 60);
    const s = Math.floor(seconds % 60);
    return m > 0 ? `${m}m ${s}s` : `${s}s`;
  }

  protected openFunnelViewer(funnel: Funnel) {
    this.selectedFunnelId.set(funnel.id);
    this.showFunnelViewer.set(true);
  }

  protected applyMetricFilter(type: MetricFilterType, metric: MetricStat) {
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

  protected exportFiltered() {
    const url = this.exportUrl();
    if (!url) return;
    window.location.href = url;
  }

  protected buildSiteUrl(path: string | null | undefined): string | null {
    const domain = this.siteDomain();
    if (!domain || !path) return null;
    const normalized = path.startsWith('/') ? path : `/${path}`;
    return `https://${domain}${normalized}`;
  }

  protected buildReferrerUrl(referrer: string | null | undefined): string | null {
    const url = this.normalizeUrl(referrer);
    return url ? url.href : null;
  }

  protected referrerDomain(referrer: string | null | undefined): string | null {
    const url = this.normalizeUrl(referrer);
    return url ? url.hostname : null;
  }

  protected faviconUrlForDomain(domain: string | null | undefined): string | null {
    return domain ? `/api/favicon/${encodeURIComponent(domain)}` : null;
  }

  private normalizeUrl(raw: string | null | undefined): URL | null {
    if (!raw) return null;
    const trimmed = raw.trim();
    if (!trimmed || trimmed.toLowerCase() === 'direct') return null;
    const normalized = /^https?:\/\//i.test(trimmed) ? trimmed : `https://${trimmed}`;
    try {
      return new URL(normalized);
    } catch {
      return null;
    }
  }
}
