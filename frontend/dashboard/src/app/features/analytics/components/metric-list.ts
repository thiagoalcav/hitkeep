import { Component, input, computed, output, ChangeDetectionStrategy, inject } from '@angular/core';
import { CommonModule, NgOptimizedImage } from '@angular/common';
import { TranslocoPipe, TranslocoService } from '@jsverse/transloco';
import { TranslocoDecimalPipe } from '@jsverse/transloco-locale';
import { CardModule } from 'primeng/card';
import { SkeletonModule } from 'primeng/skeleton';
import { MetricStat } from '@models/analytics.types';

@Component({
    selector: 'app-metric-list',
    standalone: true,
    imports: [CommonModule, CardModule, SkeletonModule, TranslocoPipe, TranslocoDecimalPipe, NgOptimizedImage],
    changeDetection: ChangeDetectionStrategy.OnPush,
    template: `
        <p-card class="shadow-sm h-full border border-surface-200 dark:border-surface-700 surface-card">
            <div class="flex items-center gap-2 mb-4">
                <i [class]="'pi ' + icon() + ' text-[var(--p-primary-color)]'" aria-hidden="true"></i>
                <h3 class="font-semibold text-lg">{{ title() }}</h3>
            </div>

            @if (isLoading()) {
                <div class="flex flex-col gap-3">
                    @for (i of [1, 2, 3, 4, 5]; track i) {
                        <p-skeleton height="1.5rem" styleClass="w-full" />
                    }
                </div>
            } @else if (!data() || data().length === 0) {
                <ul class="flex flex-col gap-3 m-0 p-0 list-none">
                    <li class="relative flex items-center justify-between text-sm text-muted-color">
                        <div class="absolute left-0 top-0 h-full w-full bg-[var(--p-surface-100)] dark:bg-[var(--p-surface-800)] rounded-r"></div>
                        <span class="relative z-10 truncate font-medium px-2 py-1">{{ 'common.unspecified' | transloco }}</span>
                        <span class="relative z-10 font-semibold px-2">0</span>
                    </li>
                </ul>
            } @else {
                <ul class="flex flex-col gap-3 m-0 p-0 list-none">
                    @for (item of data(); track item.name) {
                        <li
                            [class]="rowClass(item)"
                            [attr.role]="isRowClickable() ? 'button' : null"
                            [attr.tabindex]="isRowClickable() ? 0 : -1"
                            (click)="onRowClick(item)"
                            (keydown.enter)="onRowClick(item)"
                            (keydown.space)="$event.preventDefault(); onRowClick(item)"
                        >
                            <!-- Background Bar -->
                            <div class="absolute left-0 top-0 h-full bg-[var(--p-primary-50)] dark:bg-[var(--p-primary-900)]/30 rounded-r transition-all duration-500" [style.width.%]="(item.value / maxValue()) * 100"></div>

                            <!-- Content -->
                            <div class="relative z-10 flex items-center gap-2 min-w-0 px-2 py-1">
                                @if (showCountryFlags() && countryFlagUrl(item.name); as flagUrl) {
                                    <img [ngSrc]="flagUrl" class="size-4 shrink-0" [width]="16" [height]="16" alt="" />
                                }
                                @if (linkInfo(item); as info) {
                                    @if (info.faviconUrl) {
                                        <img [ngSrc]="info.faviconUrl" class="size-4 shrink-0 rounded-full" [width]="16" [height]="16" alt="" />
                                    }
                                    <span class="truncate font-medium text-[var(--p-text-color)]" [title]="titleForItem(item)">
                                        {{ displayLabel(item) }}
                                    </span>
                                    <a
                                        class="shrink-0 text-muted-color hover:text-[var(--p-text-color)]"
                                        [href]="info.href"
                                        target="_blank"
                                        rel="noopener noreferrer"
                                        (click)="$event.stopPropagation()"
                                        [attr.aria-label]="'common.openInNewTabAria' | transloco"
                                    >
                                        <i class="pi pi-external-link text-xs" aria-hidden="true"></i>
                                    </a>
                                } @else {
                                    <span class="truncate font-medium" [title]="titleForItem(item)">{{ displayLabel(item) }}</span>
                                }
                            </div>
                            <span class="relative z-10 font-semibold text-[var(--p-text-color)] px-2">
                                {{ item.value | translocoDecimal }}
                            </span>
                        </li>
                    }
                </ul>
            }
        </p-card>
    `
})
export class MetricList {
    private transloco = inject(TranslocoService);

    title = input.required<string>();
    icon = input<string>('pi-list');
    data = input.required<MetricStat[]>();
    isLoading = input<boolean>(false);
    linkMode = input<'none' | 'path' | 'url'>('none');
    siteDomain = input<string | null>(null);
    isRowClickable = input<boolean>(false);
    activeValue = input<string | null>(null);
    showCountryFlags = input<boolean>(false);
    showCountryNames = input<boolean>(false);
    rowClicked = output<MetricStat>();

    protected maxValue = computed(() => {
        const list = this.data();
        if (!list || list.length === 0) return 0;
        return Math.max(...list.map((i) => i.value));
    });

    protected linkInfo(item: MetricStat): { href: string; faviconUrl: string | null } | null {
        const mode = this.linkMode();
        if (mode === 'none') return null;

        if (!item.name) return null;

        if (mode === 'path') {
            const domain = this.siteDomain();
            if (!domain) return null;
            const path = item.name.startsWith('/') ? item.name : `/${item.name}`;
            return {
                href: `https://${domain}${path}`,
                faviconUrl: this.buildFaviconUrl(domain)
            };
        }

        const url = this.normalizeUrl(item.name);
        if (!url) return null;

        return {
            href: url.href,
            faviconUrl: this.buildFaviconUrl(url.hostname)
        };
    }

    private buildFaviconUrl(domain: string): string {
        return `/api/favicon/${encodeURIComponent(domain)}`;
    }

    private normalizeUrl(raw: string): URL | null {
        const trimmed = raw.trim();
        if (!trimmed || trimmed.toLowerCase() === 'direct') return null;
        const normalized = /^https?:\/\//i.test(trimmed) ? trimmed : `https://${trimmed}`;
        try {
            return new URL(normalized);
        } catch {
            return null;
        }
    }

    protected countryFlagUrl(value: string): string | null {
        const trimmed = value.trim();
        const code = trimmed.toLowerCase();
        if (!/^[a-z]{2}$/.test(code)) return '/flags/other/earth.svg';
        return `/flags/${code}.svg`;
    }

    protected displayLabel(item: MetricStat): string {
        if (!this.showCountryNames()) return item.name;
        const name = this.countryDisplayName(item.name);
        return name ?? item.name;
    }

    protected titleForItem(item: MetricStat): string {
        const display = this.displayLabel(item);
        if (display === item.name) return item.name;
        return `${item.name} · ${display}`;
    }

    private countryDisplayName(value: string): string | null {
        const code = value.trim().toUpperCase();
        if (!/^[A-Z]{2}$/.test(code)) return null;
        try {
            const language = this.transloco.getActiveLang();
            const displayNames = new Intl.DisplayNames([language], { type: 'region' });
            return displayNames.of(code) ?? null;
        } catch {
            return null;
        }
    }

    protected onRowClick(item: MetricStat) {
        if (!this.isRowClickable()) return;
        this.rowClicked.emit(item);
    }

    protected rowClass(item: MetricStat): string {
        const base = 'relative flex items-center justify-between text-sm group border border-transparent rounded-md';
        const clickable = this.isRowClickable() ? ' cursor-pointer' : '';
        const active = this.isActive(item) ? ' border-[var(--p-primary-color)] ring-1 ring-[var(--p-primary-color)] bg-[var(--p-primary-50)]/40' : '';
        return base + clickable + active;
    }

    private isActive(item: MetricStat): boolean {
        const active = this.activeValue();
        return !!active && active === item.name;
    }
}
