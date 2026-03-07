import { Injectable, inject, signal } from "@angular/core";
import { HttpClient } from "@angular/common/http";
import { tap } from "rxjs";
import { Site } from "@models/analytics.types";

const LAST_SITE_KEY = "hk_last_site_id";

@Injectable({ providedIn: "root" })
export class SiteService {
    private http = inject(HttpClient);

    // Global State for Sites
    readonly sites = signal<Site[]>([]);
    readonly activeSite = signal<Site | null>(null);
    readonly isLoading = signal<boolean>(false);

    loadSites() {
        this.isLoading.set(true);
        this.http.get<Site[]>("/api/sites").subscribe({
            next: (data) => {
                this.sites.set(data);

                if (data.length === 0) {
                    this.activeSite.set(null);
                    localStorage.removeItem(LAST_SITE_KEY);
                } else if (!this.activeSite() || !data.some((site) => site.id === this.activeSite()?.id)) {
                    const lastId = localStorage.getItem(LAST_SITE_KEY);
                    const matchedSite = lastId ? data.find((s) => s.id === lastId) : null;
                    this.activeSite.set(matchedSite || data[0]);
                }

                this.isLoading.set(false);
            },
            error: () => this.isLoading.set(false)
        });
    }

    selectSite(site: Site) {
        this.activeSite.set(site);
        localStorage.setItem(LAST_SITE_KEY, site.id);
    }

    createSite(domain: string) {
        return this.http.post<Site>("/api/sites", { domain }).pipe(
            tap((newSite) => {
                this.sites.update((list) => [newSite, ...list]);
                this.selectSite(newSite);
            })
        );
    }

    deleteSite(siteId: string) {
        return this.http.delete<void>(`/api/sites/${siteId}`).pipe(
            tap(() => {
                const updatedSites = this.sites().filter((site) => site.id !== siteId);
                this.sites.set(updatedSites);

                if (this.activeSite()?.id === siteId) {
                    const nextSite = updatedSites[0] ?? null;
                    this.activeSite.set(nextSite);
                    if (nextSite) {
                        localStorage.setItem(LAST_SITE_KEY, nextSite.id);
                    } else {
                        localStorage.removeItem(LAST_SITE_KEY);
                    }
                }
            })
        );
    }
}
