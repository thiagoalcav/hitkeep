import { Component, inject, signal, effect, input } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { ButtonModule } from 'primeng/button';
import { InputNumberModule } from 'primeng/inputnumber';
import { MessageModule } from 'primeng/message';
import { SiteService } from '../../sites/services/site.service';
import { AnalyticsService } from '../../../core/services/analytics.service';
import { Site } from '../../../core/models/analytics.types';

@Component({
  selector: 'app-site-retention-settings',
  standalone: true,
  imports: [CommonModule, FormsModule, ButtonModule, InputNumberModule, MessageModule],
  template: `
    <div class="flex flex-col gap-4">
      <div class="bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 p-4 rounded-lg flex gap-3">
         <i class="pi pi-info-circle text-blue-600 dark:text-blue-400 mt-0.5"></i>
         <div class="text-sm text-blue-800 dark:text-blue-300">
            <p class="font-semibold mb-1">How retention works</p>
            <p class="opacity-90">
                Raw hits older than the retention period are archived to Parquet files for long-term storage and removed from the hot database to improve performance.
            </p>
         </div>
      </div>

      <div class="flex flex-col gap-2 mt-2">
          <label for="retentionDays" class="font-medium">Retention Period (Days)</label>
          <div class="flex items-center gap-4">
            <p-inputnumber 
                inputId="retentionDays" 
                [(ngModel)]="retentionDays" 
                [min]="1" 
                [max]="3650" 
                [showButtons]="true" 
                buttonLayout="horizontal" 
                spinnerMode="horizontal" 
                decrementButtonClass="p-button-secondary" 
                incrementButtonClass="p-button-secondary" 
                incrementButtonIcon="pi pi-plus" 
                decrementButtonIcon="pi pi-minus"
                class="w-48">
            </p-inputnumber>
            <span class="text-sm text-muted-color">days of hot data</span>
          </div>
          <small class="text-xs text-muted-color">Default is 365 days.</small>
      </div>

      <div class="flex justify-end mt-4 pt-4 border-t border-surface-200 dark:border-surface-700">
          <p-button 
              label="Save Policy" 
              icon="pi pi-check"
              (onClick)="savePolicy()" 
              [loading]="saving()" 
              [disabled]="!hasChanged()">
          </p-button>
      </div>
    </div>
  `
})
export class SiteRetentionSettings {
  site = input.required<Site | null>();
  private siteService = inject(SiteService);
  private analyticsService = inject(AnalyticsService);
  
  retentionDays = signal<number>(365);
  saving = signal(false);
  originalDays = signal<number>(365);

  constructor() {
    effect(() => {
        const site = this.site();
        if (site) {
            const days = site.data_retention_days ?? 365;
            this.retentionDays.set(days);
            this.originalDays.set(days);
        }
    });
  }

  hasChanged() {
      return this.retentionDays() !== this.originalDays();
  }

  savePolicy() {
      const site = this.site();
      if (!site) return;

      this.saving.set(true);
      this.analyticsService.updateSiteRetention(site.id, this.retentionDays()).subscribe({
          next: () => {
              this.saving.set(false);
              this.originalDays.set(this.retentionDays());
              // Optionally refresh site data
              this.siteService.loadSites();
          },
          error: () => this.saving.set(false)
      });
  }
}