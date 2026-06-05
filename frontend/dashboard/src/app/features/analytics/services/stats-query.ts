import { DestroyRef, inject, signal } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { finalize, Subscription } from 'rxjs';
import { SiteStats } from '@models/analytics.types';
import { StatsService } from '@features/analytics/services/stats.service';

export interface StatsQueryRequest {
    siteId: string;
    from: string;
    to: string;
    filters?: { type: string; value: string }[];
    goalIds?: string[];
    funnelIds?: string[];
}

export class StatsQuery {
    readonly stats = signal<SiteStats | null>(null);
    readonly isLoading = signal(false);
    readonly comparisonRange = signal<{ from: string; to: string } | null>(null);

    private request: Subscription | null = null;

    constructor(
        private readonly statsService: StatsService,
        private readonly destroyRef: DestroyRef
    ) {
        this.destroyRef.onDestroy(() => this.request?.unsubscribe());
    }

    load(request: StatsQueryRequest): void {
        this.comparisonRange.set(this.statsService.comparisonRange(request.from, request.to));
        this.request?.unsubscribe();
        this.isLoading.set(true);
        this.request = this.statsService
            .fetchStats(request.siteId, request.from, request.to, request.filters ?? [], request.goalIds ?? [], request.funnelIds ?? [])
            .pipe(
                takeUntilDestroyed(this.destroyRef),
                finalize(() => this.isLoading.set(false))
            )
            .subscribe({
                next: (stats) => this.stats.set(stats),
                error: (e) => console.error(e)
            });
    }
}

export function injectStatsQuery(): StatsQuery {
    return new StatsQuery(inject(StatsService), inject(DestroyRef));
}
