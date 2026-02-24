import { Injectable, inject, signal } from "@angular/core";
import { HttpClient } from "@angular/common/http";
import { Observable, tap, finalize } from "rxjs";
import { FrequencyPrefs, ReportSubscriptions } from "@core/models/analytics.types";

@Injectable({ providedIn: "root" })
export class ReportSubscriptionsService {
    private http = inject(HttpClient);

    readonly subscriptions = signal<ReportSubscriptions | null>(null);
    readonly isLoading = signal(false);

    load(): Observable<ReportSubscriptions> {
        this.isLoading.set(true);
        return this.http.get<ReportSubscriptions>("/api/user/report-subscriptions").pipe(
            tap((s) => this.subscriptions.set(s)),
            finalize(() => this.isLoading.set(false))
        );
    }

    updateSiteSubscription(siteId: string, prefs: FrequencyPrefs): Observable<void> {
        return this.http.put<void>(`/api/user/report-subscriptions/sites/${siteId}`, prefs);
    }

    updateDigestSubscription(prefs: FrequencyPrefs): Observable<void> {
        return this.http.put<void>("/api/user/report-subscriptions/digest", prefs);
    }
}
