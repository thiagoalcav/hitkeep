import { Component, OnInit, effect, inject, signal, PLATFORM_ID } from '@angular/core';
import { CommonModule, DatePipe, DecimalPipe, isPlatformBrowser } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { forkJoin, finalize, Subject, debounceTime, distinctUntilChanged } from 'rxjs';
import { Router } from '@angular/router';

// PrimeNG Imports
import { SelectModule } from 'primeng/select';
import { CardModule } from 'primeng/card';
import { TableModule, TableLazyLoadEvent } from 'primeng/table';
import { ButtonModule } from 'primeng/button';
import { ChartModule } from 'primeng/chart';
import { SkeletonModule } from 'primeng/skeleton';
import { DialogModule } from 'primeng/dialog';
import { InputTextModule } from 'primeng/inputtext';
import { MessageModule } from 'primeng/message';
import { TooltipModule } from 'primeng/tooltip';
import { DatePickerModule } from 'primeng/datepicker';
import { IconFieldModule } from 'primeng/iconfield';
import { InputIconModule } from 'primeng/inputicon';
import { AvatarModule } from 'primeng/avatar';
import { MenuModule } from 'primeng/menu';
import { MenuItem } from 'primeng/api';
import { DrawerModule } from 'primeng/drawer';
import { RippleModule } from 'primeng/ripple';

import { AnalyticsService, Hit, Site, SiteStats } from '../core/services/analytics.service';
import { Brand } from '../core/components/brand/brand';

@Component({
  selector: 'app-dashboard',
  standalone: true,
  imports: [
    Brand,
    CommonModule,
    FormsModule,
    DatePipe,
    DecimalPipe,
    SelectModule,
    CardModule,
    TableModule,
    ButtonModule,
    ChartModule,
    SkeletonModule,
    DialogModule,
    InputTextModule,
    MessageModule,
    TooltipModule,
    DatePickerModule,
    IconFieldModule,
    InputIconModule,
    AvatarModule,
    MenuModule,
    DrawerModule,
    RippleModule
  ],
  templateUrl: './dashboard.html',
  styleUrl: './dashboard.css',
})
export class Dashboard implements OnInit {
  private analyticsService = inject(AnalyticsService);
  private router = inject(Router);
  private platformId = inject(PLATFORM_ID);

  protected isDrawerVisible = signal<boolean>(false);
  protected isDarkMode = signal<boolean>(false);
  protected systemVersion = signal<string>('');

  protected userMenuItems: MenuItem[] = [
    {
        label: 'Sign Out',
        icon: 'pi pi-sign-out',
        command: () => {
            document.cookie = 'hk_token=; Max-Age=0; path=/;';
            this.router.navigate(['/login']);
        }
    }
  ];

  // --- State Signals ---
  protected sites = signal<Site[]>([]);
  protected selectedSite = signal<Site | null>(null);
  protected stats = signal<SiteStats | null>(null);
  protected recentHits = signal<Hit[]>([]);
  protected totalHits = signal<number>(0); 
  protected isLoadingSites = signal<boolean>(true);
  protected isLoadingData = signal<boolean>(false); 
  protected isLoadingHits = signal<boolean>(false); 

  protected timeRanges = [
    { label: 'Last 24 Hours', value: '24h' },
    { label: 'Last 7 Days', value: '7d' },
    { label: 'Last 30 Days', value: '30d' },
    { label: 'Last Year', value: '1y' },
    { label: 'Custom Range', value: 'custom' }
  ];
  protected selectedRange = signal(this.timeRanges[2]);

  // --- Search State ---
  protected searchQuery = signal<string>('');
  private searchSubject = new Subject<string>();

  // --- Custom Range Dialog State ---
  protected isCustomRangeVisible = signal<boolean>(false);
  protected customRangeDates = signal<Date[] | null>(null);
  protected customRangeApplied = signal<boolean>(false);

  // --- Add Site Dialog State ---
  protected isAddSiteVisible = signal<boolean>(false);
  protected newSiteDomain = signal<string>('');
  protected isCreatingSite = signal<boolean>(false);
  protected createSiteError = signal<string | null>(null);

  // --- Snippet Dialog State ---
  protected isSnippetVisible = signal<boolean>(false);
  protected snippetCode = signal<string>('');

  // --- Chart Configuration ---
  protected chartData = signal<any>(null);
  protected chartOptions = {
    maintainAspectRatio: false,
    aspectRatio: 0.5,
    responsive: true,
    interaction: { mode: 'index', intersect: false },
    plugins: {
      legend: { labels: { color: '#94a3b8', usePointStyle: true, boxWidth: 8 }, position: 'bottom' },
      tooltip: { mode: 'index', intersect: false, backgroundColor: 'rgba(15, 23, 42, 0.9)', titleColor: '#f8fafc', bodyColor: '#f8fafc', borderColor: '#334155', borderWidth: 1, padding: 10, cornerRadius: 8, displayColors: true }
    },
    scales: {
      x: { ticks: { color: '#64748b', maxTicksLimit: 8 }, grid: { color: '#334155', drawBorder: false, tickLength: 0 }, border: { display: false } },
      y: { ticks: { color: '#64748b', stepSize: 1 }, grid: { color: '#334155', drawBorder: false, tickLength: 0 }, border: { display: false }, beginAtZero: true }
    }
  };

  private lastTableEvent: TableLazyLoadEvent | null = null;

  constructor() {
    if (isPlatformBrowser(this.platformId)) {
        const savedTheme = localStorage.getItem('hk_theme');
        const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
        
        if (savedTheme === 'dark' || (!savedTheme && prefersDark)) {
            this.setDarkMode(true);
        } else {
            this.setDarkMode(false);
        }
    }

    this.searchSubject.pipe(debounceTime(400), distinctUntilChanged()).subscribe(query => {
      this.searchQuery.set(query);
      if (this.lastTableEvent) {
        this.lastTableEvent.first = 0;
        this.loadHits(this.lastTableEvent);
      }
    });

    effect(() => {
      const site = this.selectedSite();
      const range = this.selectedRange();
      const customApplied = this.customRangeApplied(); 

      if (site && range) {
        let dates: {from: string, to: string} | null = null;

        if (range.value === 'custom') {
           if (customApplied) {
             const d = this.customRangeDates();
             if (d && d.length === 2 && d[0] && d[1]) {
               dates = { from: d[0].toISOString(), to: d[1].toISOString() };
             }
           }
        } else {
           this.customRangeApplied.set(false); 
           this.resetCustomLabel(); 
           dates = this.getDatesFromPreset(range.value);
        }

        if (dates) {
          this.loadDashboardStats(site.id, dates.from, dates.to);
          if (this.lastTableEvent) {
            this.lastTableEvent.first = 0; 
            this.loadHits(this.lastTableEvent);
          }
        }
      } else {
        this.stats.set(null);
        this.chartData.set(null);
        this.recentHits.set([]);
        this.totalHits.set(0);
      }
    });
  }

  ngOnInit(): void {
    this.loadSites();

    this.analyticsService.getSystemStatus().subscribe({
        next: (status) => this.systemVersion.set(status.version),
        error: () => this.systemVersion.set('unknown')
    });
  }

  protected toggleDarkMode(): void {
    this.setDarkMode(!this.isDarkMode());
  }

  private setDarkMode(isDark: boolean): void {
    this.isDarkMode.set(isDark);
    const html = document.querySelector('html');
    if (isDark) {
        html?.classList.add('p-dark');
        localStorage.setItem('hk_theme', 'dark');
    } else {
        html?.classList.remove('p-dark');
        localStorage.setItem('hk_theme', 'light');
    }
    this.updateChartTheme(isDark);
  }

    private updateChartTheme(isDark: boolean): void {
    const textColor = isDark ? '#94a3b8' : '#64748b';
    const gridColor = isDark ? 'rgba(255, 255, 255, 0.05)' : 'rgba(0, 0, 0, 0.05)';
    const tooltipBg = isDark ? 'rgba(15, 23, 42, 0.9)' : 'rgba(255, 255, 255, 0.9)';
    const tooltipText = isDark ? '#f8fafc' : '#0f172a';
    const tooltipBorder = isDark ? '#334155' : '#e2e8f0';

    this.chartOptions = {
        maintainAspectRatio: false,
        aspectRatio: 0.5,
        responsive: true,
        interaction: { mode: 'index', intersect: false },
        plugins: {
            legend: { labels: { color: textColor, usePointStyle: true, boxWidth: 8 }, position: 'bottom' },
            tooltip: { 
                mode: 'index', 
                intersect: false, 
                backgroundColor: tooltipBg, 
                titleColor: tooltipText, 
                bodyColor: tooltipText, 
                borderColor: tooltipBorder, 
                borderWidth: 1, 
                padding: 10, 
                cornerRadius: 8, 
                displayColors: true 
            }
        },
        scales: {
            x: { 
                ticks: { color: textColor, maxTicksLimit: 8 }, 
                grid: { color: gridColor, drawBorder: false, tickLength: 0 }, 
                border: { display: false } 
            },
            y: { 
                ticks: { color: textColor, stepSize: 1 }, 
                grid: { color: gridColor, drawBorder: false, tickLength: 0 }, 
                border: { display: false }, 
                beginAtZero: true 
            }
        }
    };
  }


  onSearch(event: Event): void {
    const val = (event.target as HTMLInputElement).value;
    this.searchSubject.next(val);
  }

  loadHits(event: TableLazyLoadEvent): void {
    this.lastTableEvent = event;
    const site = this.selectedSite();
    if (!site) return;

    let dates: {from: string, to: string};
    if (this.selectedRange().value === 'custom' && this.customRangeDates()) {
       const d = this.customRangeDates()!;
       if (d.length === 2 && d[0] && d[1]) {
         dates = { from: d[0].toISOString(), to: d[1].toISOString() };
       } else {
         dates = this.getDatesFromPreset('30d');
       }
    } else {
       dates = this.getDatesFromPreset(this.selectedRange().value);
    }

    this.isLoadingHits.set(true);

    const rows = event.rows ?? 10;
    const first = event.first ?? 0;
    const page = (first / rows) + 1;
    const sortField = event.sortField as string | undefined;
    const sortOrder = event.sortOrder === 1 ? 'asc' : 'desc';

    this.analyticsService.getHits(
      site.id, 
      dates.from, 
      dates.to, 
      page, 
      rows, 
      sortField, 
      sortOrder,
      this.searchQuery()
    ).pipe(finalize(() => this.isLoadingHits.set(false))).subscribe({
      next: (res) => {
        this.recentHits.set(res.data);
        this.totalHits.set(res.total);
      },
      error: (err) => console.error('Failed to load hits', err)
    });
  }

  public loadSites(): void {
    this.isLoadingSites.set(true);
    this.analyticsService.getSites().subscribe({
      next: (sites) => {
        this.sites.set(sites);
        this.isLoadingSites.set(false);
        if (sites.length > 0 && !this.selectedSite()) {
          this.selectedSite.set(sites[0]);
        }
      },
      error: (err) => { console.error(err); this.isLoadingSites.set(false); }
    });
  }

  public loadDashboardStats(siteId: string, from: string, to: string): void {
    this.isLoadingData.set(true);
    this.analyticsService.getSiteStats(siteId, from, to)
    .pipe(finalize(() => this.isLoadingData.set(false))).subscribe({
      next: (stats) => {
        this.stats.set(stats);
        this.updateChart(stats);
      },
      error: (err) => console.error(err)
    });
  }

  public getDatesFromPreset(rangeValue: string): { from: string, to: string } {
    const end = new Date();
    const start = new Date();
    switch (rangeValue) {
      case '24h': start.setHours(end.getHours() - 24); break;
      case '7d': start.setDate(end.getDate() - 7); break;
      case '30d': start.setDate(end.getDate() - 30); break;
      case '1y': start.setFullYear(end.getFullYear() - 1); break;
      default: start.setDate(end.getDate() - 30); break;
    }
    return { from: start.toISOString(), to: end.toISOString() };
  }

  protected onRangeChange(event: any): void {
    if (event.value.value === 'custom') this.isCustomRangeVisible.set(true);
  }

  protected applyCustomRange(): void {
    const dates = this.customRangeDates();
    if (dates && dates[0] && dates[1]) {
      this.isCustomRangeVisible.set(false);
      const startStr = dates[0].toLocaleDateString('en-US', { month: 'short', day: 'numeric' });
      const endStr = dates[1].toLocaleDateString('en-US', { month: 'short', day: 'numeric' });
      const label = `${startStr} - ${endStr}`;
      this.timeRanges = this.timeRanges.map(r => r.value === 'custom' ? { ...r, label: label } : r);
      const updatedCustom = this.timeRanges.find(r => r.value === 'custom');
      if (updatedCustom) this.selectedRange.set(updatedCustom);
      this.customRangeApplied.set(true); 
    }
  }

  private resetCustomLabel(): void {
    this.timeRanges = this.timeRanges.map(r => r.value === 'custom' ? { ...r, label: 'Custom Range' } : r);
  }

  protected formatDuration(seconds: number): string {
    if (!seconds) return '0s';
    const m = Math.floor(seconds / 60);
    const s = Math.floor(seconds % 60);
    if (m > 0) return `${m}m ${s}s`;
    return `${s}s`;
  }

  private updateChart(stats: SiteStats): void {
    const is24h = this.selectedRange().value === '24h';
    let isShortRange = is24h;
    if (this.selectedRange().value === 'custom' && this.customRangeDates()) {
        const dates = this.customRangeDates()!;
        if (dates[0] && dates[1]) {
            const diffHours = (dates[1].getTime() - dates[0].getTime()) / (1000 * 60 * 60);
            if (diffHours < 48) isShortRange = true;
        }
    }
    const labels = stats.chart_data.map(d => {
      const date = new Date(d.time);
      if (isShortRange) return date.toLocaleTimeString('en-US', { hour: 'numeric', minute: '2-digit' });
      return date.toLocaleDateString('en-US', { month: 'short', day: 'numeric' });
    });
    const pageviews = stats.chart_data.map(d => d.pageviews);
    const visitors = stats.chart_data.map(d => d.visitors);
    this.chartData.set({
      labels: labels,
      datasets: [
        {
          label: 'Pageviews',
          data: pageviews,
          fill: true,
          backgroundColor: (context: any) => {
            const ctx = context.chart.ctx;
            if (!ctx) return 'rgba(99, 102, 241, 0.1)';
            const gradient = ctx.createLinearGradient(0, 0, 0, 300);
            gradient.addColorStop(0, 'rgba(99, 102, 241, 0.5)');
            gradient.addColorStop(1, 'rgba(99, 102, 241, 0.0)');
            return gradient;
          },
          borderColor: '#6366f1',
          pointBackgroundColor: '#6366f1',
          tension: 0.4,
          borderWidth: 2,
          pointRadius: 0,
          pointHoverRadius: 4
        },
        {
            label: 'Visitors',
            data: visitors,
            fill: true,
            backgroundColor: (context: any) => {
              const ctx = context.chart.ctx;
              if (!ctx) return 'rgba(20, 184, 166, 0.1)';
              const gradient = ctx.createLinearGradient(0, 0, 0, 300);
              gradient.addColorStop(0, 'rgba(20, 184, 166, 0.5)');
              gradient.addColorStop(1, 'rgba(20, 184, 166, 0.0)');
              return gradient;
            },
            borderColor: '#14b8a6',
            pointBackgroundColor: '#14b8a6',
            tension: 0.4,
            borderWidth: 2,
            pointRadius: 0,
            pointHoverRadius: 4
          }
      ]
    });
  }

  protected onSiteChange(event: any): void {}
  protected openAddSiteDialog(): void {
    this.newSiteDomain.set('');
    this.createSiteError.set(null);
    this.isAddSiteVisible.set(true);
  }
  protected saveNewSite(): void {
    const domain = this.newSiteDomain().trim();
    if (!domain) { this.createSiteError.set('Domain is required'); return; }
    this.isCreatingSite.set(true);
    this.analyticsService.createSite(domain).subscribe({
      next: (site) => {
        this.isCreatingSite.set(false);
        this.isAddSiteVisible.set(false);
        this.sites.update(sites => [site, ...sites]);
        this.selectedSite.set(site);
      },
      error: (err) => {
        this.isCreatingSite.set(false);
        if (err.status === 409) this.createSiteError.set('This domain is already registered.');
        else this.createSiteError.set('Failed to create site. Please try again.');
      }
    });
  }
  protected showSnippet(): void {
    const origin = window.location.origin;
    const code = `<script src="${origin}/hk.js" async defer></script>`;
    this.snippetCode.set(code);
    this.isSnippetVisible.set(true);
  }
  protected copySnippet(): void { navigator.clipboard.writeText(this.snippetCode()); }
}