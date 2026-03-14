import { Component, input, computed, output, ChangeDetectionStrategy, inject } from "@angular/core";
import { NgOptimizedImage } from "@angular/common";
import { TranslocoPipe, TranslocoService } from "@jsverse/transloco";
import { TranslocoDecimalPipe } from "@jsverse/transloco-locale";
import { CardModule } from "primeng/card";
import { SkeletonModule } from "primeng/skeleton";
import { MetricStat } from "@models/analytics.types";

@Component({
    selector: "app-metric-list",
    imports: [CardModule, SkeletonModule, TranslocoPipe, TranslocoDecimalPipe, NgOptimizedImage],
    templateUrl: "./metric-list.html",
    styleUrl: "./metric-list.css",
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class MetricList {
    private readonly transloco = inject(TranslocoService);

    title = input.required<string>();
    icon = input<string>("pi-list");
    data = input.required<MetricStat[]>();
    isLoading = input<boolean>(false);
    linkMode = input<"none" | "path" | "url">("none");
    siteDomain = input<string | null>(null);
    isRowClickable = input<boolean>(false);
    activeValue = input<string | null>(null);
    showCountryFlags = input<boolean>(false);
    showCountryNames = input<boolean>(false);
    rowClicked = output<MetricStat>();

    protected readonly totalValue = computed(() => this.data().reduce((sum, item) => sum + item.value, 0));

    protected readonly maxValue = computed(() => {
        const list = this.data();
        if (list.length === 0) return 0;
        return Math.max(...list.map((item) => item.value), 0);
    });

    protected linkInfo(item: MetricStat): { href: string; faviconUrl: string | null } | null {
        const mode = this.linkMode();
        if (mode === "none" || !item.name) return null;

        if (mode === "path") {
            const domain = this.siteDomain();
            if (!domain) return null;
            const path = item.name.startsWith("/") ? item.name : `/${item.name}`;
            return {
                href: `https://${domain}${path}`,
                faviconUrl: null
            };
        }

        const url = this.normalizeUrl(item.name);
        if (!url) return null;

        return {
            href: url.href,
            faviconUrl: this.buildFaviconUrl(url.hostname)
        };
    }

    protected countryFlagUrl(value: string): string | null {
        const trimmed = value.trim();
        const code = trimmed.toLowerCase();
        if (!/^[a-z]{2}$/.test(code)) return "/flags/other/earth.svg";
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

    protected shareForItem(item: MetricStat): number {
        const total = this.totalValue();
        if (!total) return 0;
        return (item.value / total) * 100;
    }

    protected isDeviceMetric(): boolean {
        return this.icon() === "pi-mobile" && this.linkMode() === "none" && !this.showCountryFlags();
    }

    protected deviceIconClass(item: MetricStat): string {
        const normalized = item.name.trim().toLowerCase();
        if (normalized.includes("tablet")) {
            return "pi pi-tablet";
        }
        if (normalized.includes("mobile")) {
            return "pi pi-mobile";
        }
        return "pi pi-desktop";
    }

    protected barWidth(item: MetricStat): number {
        const max = this.maxValue();
        if (!max) return 0;
        return (item.value / max) * 100;
    }

    protected onRowClick(item: MetricStat): void {
        if (!this.isRowClickable()) return;
        this.rowClicked.emit(item);
    }

    protected rowClass(item: MetricStat): string {
        const base = "metric-list__row group relative flex items-center justify-between overflow-hidden rounded-md text-sm transition-colors";
        const clickable = this.isRowClickable() ? " cursor-pointer hover:bg-surface-50 dark:hover:bg-surface-800" : "";
        const active = this.isActive(item) ? " ring-1 ring-[var(--p-primary-color)] bg-[var(--p-primary-50)] dark:bg-[var(--p-primary-900)]/30" : "";
        return base + clickable + active;
    }

    private buildFaviconUrl(domain: string): string {
        return `/api/favicon/${encodeURIComponent(domain)}`;
    }

    private normalizeUrl(raw: string): URL | null {
        const trimmed = raw.trim();
        if (!trimmed || trimmed.startsWith("(")) return null;
        const normalized = /^https?:\/\//i.test(trimmed) ? trimmed : `https://${trimmed}`;
        try {
            return new URL(normalized);
        } catch {
            return null;
        }
    }

    private countryDisplayName(value: string): string | null {
        const code = value.trim().toUpperCase();
        if (!/^[A-Z]{2}$/.test(code)) return null;
        try {
            const language = this.transloco.getActiveLang();
            const displayNames = new Intl.DisplayNames([language], { type: "region" });
            return displayNames.of(code) ?? null;
        } catch {
            return null;
        }
    }

    private isActive(item: MetricStat): boolean {
        const active = this.activeValue();
        return !!active && active === item.name;
    }
}
