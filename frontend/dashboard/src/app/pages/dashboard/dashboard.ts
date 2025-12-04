import { Component, effect, inject, signal, computed } from '@angular/core';
import {CommonModule, DecimalPipe, DatePipe, NgOptimizedImage} from '@angular/common';
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
import { TooltipModule } from 'primeng/tooltip';
// Features
import { SiteService } from '../../features/sites/services/site.service';
import { StatsService } from '../../features/analytics/services/stats.service';
import { HitService } from '../../features/hits/services/hit.service';
import { TrafficChart } from '../../features/analytics/components/traffic-chart';
import {SiteFavicon} from '../../features/sites/components/site-favicon';
import {MetricList} from '../../features/analytics/components/metric-list';
import {GoalList} from '../../features/analytics/components/goal-list';
import {FunnelList} from '../../features/analytics/components/funnel-list';

interface RangeSelectEvent {
  value: {
    label: string;
    value: string;
  };
}
@Component({
  selector: 'app-dashboard',
  standalone: true,
  imports: [
    CommonModule, FormsModule, DecimalPipe, DatePipe,
    CardModule, TableModule, SelectModule, ButtonModule,
    IconFieldModule, InputIconModule, InputTextModule,
    SkeletonModule, DialogModule, DatePickerModule, TooltipModule,
    TrafficChart, SiteFavicon, MetricList, GoalList, FunnelList
  ],
  templateUrl: './dashboard.html',
  styleUrl: './dashboard.css',
  providers: [DatePipe]
})
export class Dashboard {
  protected siteService = inject(SiteService);
  protected statsService = inject(StatsService);
  protected hitService = inject(HitService);
  private datePipe = inject(DatePipe);
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
        this.statsService.loadStats(site.id, dates.from, dates.to);
        this.refreshHits();
      }
    });
  }

  refreshAll() {
    const site = this.siteService.activeSite();
    const dates = this.getCurrentDateRange();
    if (site && dates) {
      this.statsService.loadStats(site.id, dates.from, dates.to);
      this.refreshHits();
    }
  }

  onSearch(event: Event) {
    this.searchSubject.next((event.target as HTMLInputElement).value);
  }

  loadHits(event: TableLazyLoadEvent) {
    this.lastTableEvent = event;
    const site = this.siteService.activeSite();
    const dates = this.getCurrentDateRange();
    if (!site || !dates) return;

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
      this.searchQuery()
    );
  }

  private refreshHits() {
    if (this.lastTableEvent) {
      this.lastTableEvent.first = 0;
      this.loadHits(this.lastTableEvent);
    }
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
}
