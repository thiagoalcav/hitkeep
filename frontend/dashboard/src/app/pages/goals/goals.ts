import { Component, inject, signal, effect } from '@angular/core';

import { ButtonModule } from 'primeng/button';
import { SiteService } from '../../features/sites/services/site.service';
import { StatsService } from '../../features/analytics/services/stats.service';
import { GoalList } from '../../features/analytics/components/goal-list';
import { GoalManager } from '../../features/goals/components/goal-manager';
import { SiteFavicon } from '../../features/sites/components/site-favicon';

@Component({
  selector: 'app-goals',
  standalone: true,
  imports: [ButtonModule, GoalList, GoalManager, SiteFavicon],
  template: `
    <div class="flex flex-col h-full w-full bg-[var(--p-surface-ground)] transition-colors duration-200">
      <!-- Header -->
      <div class="flex flex-col md:flex-row md:items-center justify-between gap-4 mb-8">
        <div>
          <div class="flex items-center gap-3">
            @if (siteService.activeSite()) {
              <app-site-favicon [site]="siteService.activeSite()"/>
            }
            <h1 class="text-2xl md:text-3xl font-bold text-[var(--p-text-color)]">Goals</h1>
          </div>
          <p class="text-sm text-[var(--p-text-muted-color)] mt-1">
            {{ siteService.activeSite()?.domain || 'Select a Site' }}
          </p>
        </div>
        <div class="flex items-center gap-3">
          <p-button icon="pi pi-plus" label="Manage Goals" (onClick)="openGoalManager()"
                    class="p-button-rounded p-button-secondary" size="small">
          </p-button>
        </div>
      </div>

      @if (siteService.activeSite()) {
        <div class="grid grid-cols-1 gap-4">
          <app-goal-list
            [data]="statsService.stats()?.goals || []"
            [siteId]="siteService.activeSite()?.id || null"
            [isLoading]="statsService.isLoading()"
            (refresh)="refreshStats()" />
        </div>
      } @else {
        <div class="flex flex-col items-center justify-center h-[50vh] gap-4">
          <i class="pi pi-globe text-6xl text-primary opacity-20"></i>
          <h2 class="text-2xl font-semibold text-muted-color">No Site Selected</h2>
          <p class="text-muted-color">Select or create a site to view goals.</p>
        </div>
      }

      <app-goal-manager [(visible)]="isGoalManagerVisible" [siteId]="siteService.activeSite()?.id || null" (goalsChanged)="refreshStats()"/>
    </div>
  `
})
export class Goals {
  protected siteService = inject(SiteService);
  protected statsService = inject(StatsService);
  
  protected isGoalManagerVisible = signal(false);

  constructor() {
    effect(() => {
      const site = this.siteService.activeSite();
      if (site) {
        // Default to last 30 days for goal stats overview
        const end = new Date();
        const start = new Date();
        start.setDate(end.getDate() - 30);
        this.statsService.loadStats(site.id, start.toISOString(), end.toISOString());
      }
    });
  }

  openGoalManager() {
    this.isGoalManagerVisible.set(true);
  }

  refreshStats() {
    const site = this.siteService.activeSite();
    if (site) {
        const end = new Date();
        const start = new Date();
        start.setDate(end.getDate() - 30);
        this.statsService.loadStats(site.id, start.toISOString(), end.toISOString());
    }
  }
}