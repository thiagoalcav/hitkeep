import { Component, input, signal, inject, ChangeDetectionStrategy } from '@angular/core';
import { HttpClient } from '@angular/common/http';

import { ButtonModule } from 'primeng/button';
import { Site } from '../../../core/models/analytics.types';

@Component({
  selector: 'app-site-general-settings',
  standalone: true,
  imports: [ButtonModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
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
        <div class="p-4 border rounded-lg bg-surface-0 dark:bg-surface-900 border-surface-200 dark:border-surface-700">
          <div class="flex flex-col md:flex-row md:items-center gap-4 md:justify-between">
            <div class="flex items-start gap-3">
              <div class="w-10 h-10 rounded-full bg-[var(--p-surface-100)] dark:bg-[var(--p-surface-800)] flex items-center justify-center text-[var(--p-primary-color)]">
                <i class="pi pi-database" aria-hidden="true"></i>
              </div>
              <div class="flex flex-col gap-1">
                <div class="font-medium">Export Site Data</div>
                <div class="text-sm text-muted-color">Download all hits and events for this site as an XLSX file.</div>
              </div>
            </div>
            <p-button
              label="{{ isExporting() ? 'Preparing...' : 'Download XLSX' }}"
              icon="pi pi-download"
              [loading]="isExporting()"
              [disabled]="isExporting()"
              (onClick)="downloadData()" />
          </div>
        </div>
      </div>
    </div>
  `
})
export class SiteGeneralSettings {
  site = input.required<Site | null>();
  protected isExporting = signal(false);
  private http = inject(HttpClient);

  downloadData() {
    const site = this.site();
    if (!site?.id || this.isExporting()) return;

    this.isExporting.set(true);
    this.http.get(`/api/sites/${site.id}/takeout`, { responseType: 'blob', observe: 'response' }).subscribe({
      next: (response) => {
        this.isExporting.set(false);
        const blob = response.body;
        if (!blob || blob.size === 0) return;

        const url = URL.createObjectURL(blob);
        const link = document.createElement('a');
        link.href = url;
        link.download = this.extractFilename(response.headers.get('content-disposition')) ?? this.buildFilename(site.domain);
        document.body.appendChild(link);
        link.click();
        link.remove();
        URL.revokeObjectURL(url);
      },
      error: () => this.isExporting.set(false)
    });
  }

  private extractFilename(header: string | null): string | null {
    if (!header) return null;
    const match = header.match(/filename="?([^\";]+)"?/i);
    return match ? match[1] : null;
  }

  private buildFilename(domain?: string): string {
    const safeDomain = (domain || 'site')
      .toLowerCase()
      .replace(/[^a-z0-9]+/g, '-')
      .replace(/(^-|-$)/g, '');
    const dateStamp = new Date().toISOString().slice(0, 10);
    return `${safeDomain || 'site'}-takeout-${dateStamp}.xlsx`;
  }
}
