import { HttpClient, HttpResponse } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { TakeoutExportFormat } from '@core/export/export-formats';
import { map, Observable } from 'rxjs';

@Injectable({ providedIn: 'root' })
export class TakeoutDownloadService {
    private http = inject(HttpClient);

    downloadUserTakeout(format: TakeoutExportFormat): Observable<string> {
        const dateStamp = this.currentDateStamp();
        const fallbackFilename = `user-takeout-${dateStamp}.${format}`;
        return this.downloadFromUrl(`/api/user/takeout?format=${format}`, fallbackFilename);
    }

    downloadSiteTakeout(siteID: string, domain: string | undefined, format: TakeoutExportFormat): Observable<string> {
        const safeDomain = (domain || 'site')
            .toLowerCase()
            .replace(/[^a-z0-9]+/g, '-')
            .replace(/(^-|-$)/g, '');
        const dateStamp = this.currentDateStamp();
        const fallbackFilename = `${safeDomain || 'site'}-takeout-${dateStamp}.${format}`;
        return this.downloadFromUrl(`/api/sites/${siteID}/takeout?format=${format}`, fallbackFilename);
    }

    downloadFromUrl(url: string, fallbackFilename: string): Observable<string> {
        return this.http.get(url, { responseType: 'blob', observe: 'response' }).pipe(map((response) => this.persistDownload(response, fallbackFilename)));
    }

    private persistDownload(response: HttpResponse<Blob>, fallbackFilename: string): string {
        const blob = response.body;
        if (!blob) {
            throw new Error('missing_takeout_download');
        }

        const filename = this.extractFilename(response.headers.get('content-disposition')) ?? fallbackFilename;
        this.saveBlob(blob, filename);
        return filename;
    }

    private saveBlob(blob: Blob, filename: string): void {
        const objectURL = URL.createObjectURL(blob);
        const link = document.createElement('a');
        link.href = objectURL;
        link.download = filename;
        link.style.display = 'none';
        document.body.appendChild(link);
        link.click();
        link.remove();
        URL.revokeObjectURL(objectURL);
    }

    private extractFilename(header: string | null): string | null {
        if (!header) return null;

        const encodedMatch = header.match(/filename\*=UTF-8''([^;]+)/i);
        if (encodedMatch?.[1]) {
            try {
                return decodeURIComponent(encodedMatch[1]);
            } catch {
                return encodedMatch[1];
            }
        }

        const match = header.match(/filename="?([^";]+)"?/i);
        return match?.[1] ?? null;
    }

    private currentDateStamp(): string {
        return new Date().toISOString().slice(0, 10);
    }
}
