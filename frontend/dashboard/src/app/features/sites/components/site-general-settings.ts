import { Component, input, signal, inject, ChangeDetectionStrategy, computed } from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { HttpClient } from '@angular/common/http';
import { TranslocoPipe, TranslocoService } from '@jsverse/transloco';

import { SplitButtonModule } from 'primeng/splitbutton';
import { MenuItem } from 'primeng/api';
import { Site } from '@models/analytics.types';

type ExportFormat = 'csv' | 'xlsx' | 'parquet';

@Component({
    selector: 'app-site-general-settings',
    standalone: true,
    imports: [SplitButtonModule, TranslocoPipe],
    changeDetection: ChangeDetectionStrategy.OnPush,
    templateUrl: './site-general-settings.html',
    styleUrl: './site-general-settings.css'
})
export class SiteGeneralSettings {
    site = input.required<Site | null>();
    protected isExporting = signal(false);
    private http = inject(HttpClient);
    private transloco = inject(TranslocoService);
    private activeLanguage = toSignal(this.transloco.langChanges$, { initialValue: this.transloco.getActiveLang() });
    protected readonly exportMenuItems = computed<MenuItem[]>(() => {
        this.activeLanguage();
        return [
            { label: this.transloco.translate('common.exportFormats.csv'), icon: 'pi pi-file', command: () => this.downloadData('csv') },
            { label: this.transloco.translate('common.exportFormats.xlsx'), icon: 'pi pi-file-excel', command: () => this.downloadData('xlsx') },
            { label: this.transloco.translate('common.exportFormats.parquet'), icon: 'pi pi-database', command: () => this.downloadData('parquet') }
        ];
    });

    downloadData(format: ExportFormat = 'xlsx') {
        const site = this.site();
        if (!site?.id || this.isExporting()) return;

        this.isExporting.set(true);
        this.http.get(`/api/sites/${site.id}/takeout?format=${format}`, { responseType: 'blob', observe: 'response' }).subscribe({
            next: (response) => {
                this.isExporting.set(false);
                const blob = response.body;
                if (!blob || blob.size === 0) return;

                const url = URL.createObjectURL(blob);
                const link = document.createElement('a');
                link.href = url;
                link.download = this.extractFilename(response.headers.get('content-disposition')) ?? this.buildFilename(site.domain, format);
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
        const match = header.match(/filename="?([^";]+)"?/i);
        return match ? match[1] : null;
    }

    private buildFilename(domain: string | undefined, format: ExportFormat): string {
        const safeDomain = (domain || 'site')
            .toLowerCase()
            .replace(/[^a-z0-9]+/g, '-')
            .replace(/(^-|-$)/g, '');
        const dateStamp = new Date().toISOString().slice(0, 10);
        return `${safeDomain || 'site'}-takeout-${dateStamp}.${format}`;
    }
}
