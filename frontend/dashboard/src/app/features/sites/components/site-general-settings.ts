import { Component, input } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ButtonModule } from 'primeng/button';
import { Site } from '../../../core/models/analytics.types';

@Component({
  selector: 'app-site-general-settings',
  standalone: true,
  imports: [CommonModule, ButtonModule],
  template: `
    <div class="flex flex-col gap-6">
      <div>
        <h3 class="font-bold text-lg mb-2">General Settings</h3>
        <div class="p-4 border rounded-lg bg-surface-50 dark:bg-surface-900 border-surface-200 dark:border-surface-700">
          <div class="flex flex-col gap-2">
            <label class="text-sm font-medium text-muted-color">Domain</label>
            <div class="font-mono">{{ site()?.domain }}</div>
          </div>
        </div>
      </div>

      <div>
        <h3 class="font-bold text-lg mb-2">Data Export</h3>
        <div class="p-4 border rounded-lg bg-surface-50 dark:bg-surface-900 border-surface-200 dark:border-surface-700">
          <div class="flex items-center justify-between">
            <div>
              <div class="font-medium">Export Site Data</div>
              <div class="text-sm text-muted-color">Download all hits and events for this site as an XLSX file.</div>
            </div>
            <p-button
              label="Download Data"
              icon="pi pi-download"
              [outlined]="true"
              (onClick)="downloadData()" />
          </div>
        </div>
      </div>
    </div>
  `
})
export class SiteGeneralSettings {
  site = input.required<Site | null>();

  downloadData() {
    const siteId = this.site()?.id;
    if (!siteId) return;
    
    // Trigger download
    window.open(`/api/sites/${siteId}/takeout`, '_blank');
  }
}