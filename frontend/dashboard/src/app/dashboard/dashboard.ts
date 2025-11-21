import { Component, OnInit, effect, inject, signal } from '@angular/core';
import { CommonModule, DatePipe, DecimalPipe } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { forkJoin, finalize } from 'rxjs';

// PrimeNG Imports
import { SelectModule } from 'primeng/select';
import { CardModule } from 'primeng/card';
import { TableModule } from 'primeng/table';
import { ButtonModule } from 'primeng/button';
import { ChartModule } from 'primeng/chart';
import { SkeletonModule } from 'primeng/skeleton';
import { DialogModule } from 'primeng/dialog';
import { InputTextModule } from 'primeng/inputtext';
import { MessageModule } from 'primeng/message';
import { TooltipModule } from 'primeng/tooltip';
import { DatePickerModule } from 'primeng/datepicker';

import { AnalyticsService, Hit, Site, SiteStats } from '../core/services/analytics.service';

@Component({
  selector: 'app-dashboard',
  standalone: true,
  imports: [
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
    DatePickerModule
  ],
  templateUrl: './dashboard.html',
  styleUrl: './dashboard.css',
})
export class Dashboard implements OnInit {
  private analyticsService = inject(AnalyticsService);

  // --- State Signals ---
  protected sites = signal<Site[]>([]);
  protected selectedSite = signal<Site | null>(null);
  
  protected stats = signal<SiteStats | null>(null);
  protected recentHits = signal<Hit[]>([]);
  
  protected isLoadingSites = signal<boolean>(true);
  protected isLoadingData = signal<boolean>(false);

  protected timeRanges = [
    { label: 'Last 24 Hours', value: '24h' },
    { label: 'Last 7 Days', value: '7d' },
    { label: 'Last 30 Days', value: '30d' },
    { label: 'Last Year', value: '1y' },
    { label: 'Custom Range', value: 'custom' }
  ];
  protected selectedRange = signal(this.timeRanges[2]); // Default 30d

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
    interaction: {
        mode: 'index',
        intersect: false,
    },
    plugins: {
      legend: {
        labels: { color: '#94a3b8', usePointStyle: true, boxWidth: 8 },
        position: 'bottom'
      },
      tooltip: {
        mode: 'index',
        intersect: false,
        backgroundColor: 'rgba(15, 23, 42, 0.9)',
        titleColor: '#f8fafc',
        bodyColor: '#f8fafc',
        borderColor: '#334155',
        borderWidth: 1,
        padding: 10,
        cornerRadius: 8,
        displayColors: true
      }
    },
    scales: {
      x: {
        ticks: { color: '#64748b', maxTicksLimit: 8 },
        grid: { color: '#334155', drawBorder: false, tickLength: 0 },
        border: { display: false }
      },
      y: {
        ticks: { color: '#64748b', stepSize: 1 },
        grid: { color: '#334155', drawBorder: false, tickLength: 0 },
        border: { display: false },
        beginAtZero: true
      }
    }
  };

  constructor() {
    // Trigger data load when Site OR Time Range changes
    effect(() => {
      const site = this.selectedSite();
      const range = this.selectedRange();
      const customApplied = this.customRangeApplied(); 

      if (site && range) {
        if (range.value === 'custom') {
           if (customApplied) {
             const dates = this.customRangeDates();
             if (dates && dates.length === 2 && dates[0] && dates[1]) {
               this.loadDashboardData(site.id, dates[0].toISOString(), dates[1].toISOString());
             }
           }
        } else {
           this.customRangeApplied.set(false); 
           // Todo: weird ux
           this.resetCustomLabel(); 
           
           const dates = this.getDatesFromPreset(range.value);
           this.loadDashboardData(site.id, dates.from, dates.to);
        }
      } else {
        this.stats.set(null);
        this.chartData.set(null);
        this.recentHits.set([]);
      }
    });
  }

  ngOnInit(): void {
    this.loadSites();
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
      error: (err) => {
        console.error('Failed to load sites', err);
        this.isLoadingSites.set(false);
      }
    });
  }

  public loadDashboardData(siteId: string, from: string, to: string): void {
    this.isLoadingData.set(true);
    // TODO: revisit at some point
    forkJoin({
      stats: this.analyticsService.getSiteStats(siteId, from, to),
      hits: this.analyticsService.getHits(siteId)
    }).pipe(
      finalize(() => this.isLoadingData.set(false))
    ).subscribe({
      next: (result) => {
        this.stats.set(result.stats);
        this.updateChart(result.stats);
        this.recentHits.set(result.hits);
      },
      error: (err) => {
        console.error('Failed to load dashboard data', err);
      }
    });
  }

  public getDatesFromPreset(rangeValue: string): { from: string, to: string } {
    const end = new Date();
    const start = new Date();

    switch (rangeValue) {
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
      default:
        start.setDate(end.getDate() - 30);
        break;
    }

    return {
      from: start.toISOString(),
      to: end.toISOString()
    };
  }

  protected onRangeChange(event: any): void {
    if (event.value.value === 'custom') {
      this.isCustomRangeVisible.set(true);
    }
  }

  protected applyCustomRange(): void {
    const dates = this.customRangeDates();
    if (dates && dates[0] && dates[1]) {
      this.isCustomRangeVisible.set(false);
      
      // Format label: "Nov 20 - Nov 21"
      const startStr = dates[0].toLocaleDateString('en-US', { month: 'short', day: 'numeric' });
      const endStr = dates[1].toLocaleDateString('en-US', { month: 'short', day: 'numeric' });
      const label = `${startStr} - ${endStr}`;

      // Update the 'custom' entry in timeRanges
      this.timeRanges = this.timeRanges.map(r => {
        if (r.value === 'custom') {
            return { ...r, label: label };
        }
        return r;
      });

      // Re-set the selectedRange to point to the updated object
      // This ensures the Dropdown UI updates to show the dates
      const updatedCustom = this.timeRanges.find(r => r.value === 'custom');
      if (updatedCustom) {
          this.selectedRange.set(updatedCustom);
      }

      this.customRangeApplied.set(true); 
    }
  }

  private resetCustomLabel(): void {
    // If we switch back to a preset, reset the custom label to "Custom Range"
    // so it looks clean if they open it again later.
    this.timeRanges = this.timeRanges.map(r => {
        if (r.value === 'custom') {
            return { ...r, label: 'Custom Range' };
        }
        return r;
    });
  }

  // Helper to format seconds into "1m 30s"
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
      if (isShortRange) {
         return date.toLocaleTimeString('en-US', { hour: 'numeric', minute: '2-digit' });
      }
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

  protected onSiteChange(event: any): void {
    // The effect will handle the data fetching
  }

  protected openAddSiteDialog(): void {
    this.newSiteDomain.set('');
    this.createSiteError.set(null);
    this.isAddSiteVisible.set(true);
  }

  protected saveNewSite(): void {
    const domain = this.newSiteDomain().trim();
    if (!domain) {
      this.createSiteError.set('Domain is required');
      return;
    }

    this.isCreatingSite.set(true);
    this.analyticsService.createSite(domain).subscribe({
      next: (site) => {
        this.isCreatingSite.set(false);
        this.isAddSiteVisible.set(false);
        
        // Update sites list and select the new one
        this.sites.update(sites => [site, ...sites]);
        this.selectedSite.set(site);
      },
      error: (err) => {
        this.isCreatingSite.set(false);
        if (err.status === 409) {
          this.createSiteError.set('This domain is already registered.');
        } else {
          this.createSiteError.set('Failed to create site. Please try again.');
        }
      }
    });
  }

  protected showSnippet(): void {
    const origin = window.location.origin;
    const code = `<script src="${origin}/hk.js" async defer></script>`;
    this.snippetCode.set(code);
    this.isSnippetVisible.set(true);
  }
  
  protected copySnippet(): void {
    navigator.clipboard.writeText(this.snippetCode());
  }
}