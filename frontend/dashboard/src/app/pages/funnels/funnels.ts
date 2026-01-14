import { Component, inject, signal, effect } from '@angular/core';

import { ButtonModule } from 'primeng/button';
import { SiteService } from '../../features/sites/services/site.service';
import { AnalyticsService } from '../../core/services/analytics.service';
import { FunnelList } from '../../features/analytics/components/funnel-list';
import { FunnelManager } from '../../features/funnels/components/funnel-manager';
import { FunnelViewer } from '../../features/funnels/components/funnel-viewer';
import { SiteFavicon } from '../../features/sites/components/site-favicon';
import { Funnel } from '../../core/models/analytics.types';

@Component({
  selector: 'app-funnels',
  standalone: true,
  imports: [ButtonModule, FunnelList, FunnelManager, FunnelViewer, SiteFavicon],
  template: `
    <div class="flex flex-col h-full w-full bg-[var(--p-surface-ground)] transition-colors duration-200">
      <!-- Header -->
      <div class="flex flex-col md:flex-row md:items-center justify-between gap-4 mb-8">
        <div>
          <div class="flex items-center gap-3">
            @if (siteService.activeSite()) {
              <app-site-favicon [site]="siteService.activeSite()"/>
            }
            <h1 class="text-2xl md:text-3xl font-bold text-[var(--p-text-color)]">Funnels</h1>
          </div>
          <p class="text-sm text-[var(--p-text-muted-color)] mt-1">
            {{ siteService.activeSite()?.domain || 'Select a Site' }}
          </p>
        </div>
        <div class="flex items-center gap-3">
          <p-button icon="pi pi-plus" label="Create Funnel" (onClick)="openFunnelManager()"
                    class="p-button-rounded p-button-secondary" size="small">
          </p-button>
        </div>
      </div>

      @if (siteService.activeSite()) {
        <div class="grid grid-cols-1 gap-4">
          <app-funnel-list
            [funnels]="funnels()"
            [isLoading]="loading()"
            (manageClicked)="openFunnelManager()"
            (funnelClicked)="viewFunnel($event)"/>
        </div>
      } @else {
        <div class="flex flex-col items-center justify-center h-[50vh] gap-4">
          <i class="pi pi-globe text-6xl text-primary opacity-20"></i>
          <h2 class="text-2xl font-semibold text-muted-color">No Site Selected</h2>
          <p class="text-muted-color">Select or create a site to view funnels.</p>
        </div>
      }

      <app-funnel-manager [(visible)]="isFunnelManagerVisible" [siteId]="siteService.activeSite()?.id || null" (funnelsChanged)="loadFunnels()"/>
      <app-funnel-viewer [(visible)]="isFunnelViewerVisible" [siteId]="siteService.activeSite()?.id || null" [funnelId]="selectedFunnel()?.id || null" [dateRange]="getDateRange()"/>
    </div>
  `
})
export class Funnels {
  protected siteService = inject(SiteService);
  protected analyticsService = inject(AnalyticsService);
  
  protected isFunnelManagerVisible = signal(false);
  protected isFunnelViewerVisible = signal(false);
  protected selectedFunnel = signal<Funnel | null>(null);
  
  protected funnels = signal<Funnel[]>([]);
  protected loading = signal(false);

  constructor() {
    effect(() => {
      const site = this.siteService.activeSite();
      if (site) {
        this.loadFunnels();
      }
    });
  }

  loadFunnels() {
    const site = this.siteService.activeSite();
    if (!site) return;
    this.loading.set(true);
    this.analyticsService.getFunnels(site.id).subscribe({
      next: (data) => {
        this.funnels.set(data);
        this.loading.set(false);
      },
      error: () => this.loading.set(false)
    });
  }

  openFunnelManager() {
    this.isFunnelManagerVisible.set(true);
  }

  viewFunnel(funnel: Funnel) {
    this.selectedFunnel.set(funnel);
    this.isFunnelViewerVisible.set(true);
  }

  getDateRange() {
    // Default to last 30 days for funnel viewer
    const end = new Date();
    const start = new Date();
    start.setDate(end.getDate() - 30);
    return { from: start.toISOString(), to: end.toISOString() };
  }
}