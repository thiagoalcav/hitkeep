import { Component, effect, inject, signal, computed, ChangeDetectionStrategy, untracked, DestroyRef } from "@angular/core";
import { takeUntilDestroyed } from "@angular/core/rxjs-interop";
import { injectActiveLang } from "@core/i18n/active-lang";
import { NgOptimizedImage } from "@angular/common";
import { FormControl, ReactiveFormsModule } from "@angular/forms";
import { compatForm } from "@angular/forms/signals/compat";
import { debounceTime, distinctUntilChanged, finalize, Subject } from "rxjs";
import { TranslocoService } from "@jsverse/transloco";
import { TranslocoPipe } from "@jsverse/transloco";
import { TranslocoLocaleService } from "@jsverse/transloco-locale";
// PrimeNG
import { CardModule } from "primeng/card";
import { TableModule, TableLazyLoadEvent } from "primeng/table";
import { SelectModule } from "primeng/select";
import { ButtonModule } from "primeng/button";
import { SplitButtonModule } from "primeng/splitbutton";
import { IconFieldModule } from "primeng/iconfield";
import { InputIconModule } from "primeng/inputicon";
import { InputTextModule } from "primeng/inputtext";
import { SkeletonModule } from "primeng/skeleton";
import { DialogModule } from "primeng/dialog";
import { DatePickerModule } from "primeng/datepicker";
import { MenuItem } from "primeng/api";
// Features
import { SiteService } from "@features/sites/services/site.service";
import { StatsService } from "@features/analytics/services/stats.service";
import { HitService } from "@features/hits/services/hit.service";
import { TrafficChart } from "@features/analytics/components/traffic-chart";
import { MetricList } from "@features/analytics/components/metric-list";
import { GoalList } from "@features/analytics/components/goal-list";
import { FunnelList } from "@features/analytics/components/funnel-list";
import { FunnelManager } from "@features/funnels/components/funnel-manager";
import { FunnelViewer } from "@features/funnels/components/funnel-viewer";
import { Funnel } from "@models/analytics.types";
import { MetricStat } from "@models/analytics.types";
import { PageHeader } from "@components/page-header/page-header";
import { PageBreadcrumb, PageBreadcrumbItem } from "@components/page-breadcrumb/page-breadcrumb";
import { KpiCard } from "@features/analytics/components/kpi-card";
import { ShareService } from "@services/share.service";
import { RangeToolbar } from "@components/range-toolbar/range-toolbar";
import { SiteSettingsService } from "@services/site-settings.service";
import { RelativeDateTime } from "@components/relative-date-time/relative-date-time";
import { buildTakeoutExportMenuItems, DEFAULT_HITS_EXPORT_FORMAT, TakeoutExportFormat } from "@core/export/export-formats";
import { TakeoutDownloadService } from "@services/takeout-download.service";
import { AddSiteDialog } from "@features/sites/components/add-site-dialog";

interface RangeSelectEvent {
    value: {
        label: string;
        value: string;
    };
}

type MetricFilterType = "path" | "referrer" | "device" | "country";
interface MetricFilter {
    type: MetricFilterType;
    value: string;
}
interface KpiCardData {
    label: string;
    value: number | string;
    loading: boolean;
    valueClass: string;
    delta?: number | null;
}
@Component({
    selector: "app-dashboard",
    standalone: true,
    imports: [
        ReactiveFormsModule,
        TranslocoPipe,
        CardModule,
        TableModule,
        SelectModule,
        ButtonModule,
        SplitButtonModule,
        IconFieldModule,
        InputIconModule,
        InputTextModule,
        SkeletonModule,
        DialogModule,
        DatePickerModule,
        PageHeader,
        PageBreadcrumb,
        RangeToolbar,
        RelativeDateTime,
        KpiCard,
        TrafficChart,
        MetricList,
        GoalList,
        FunnelList,
        FunnelManager,
        FunnelViewer,
        NgOptimizedImage,
        AddSiteDialog
    ],
    templateUrl: "./dashboard.html",
    styleUrl: "./dashboard.css",
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class Dashboard {
    protected siteService = inject(SiteService);
    protected statsService = inject(StatsService);
    protected hitService = inject(HitService);
    private shareService = inject(ShareService);
    private siteSettings = inject(SiteSettingsService);
    private takeoutDownloadService = inject(TakeoutDownloadService);
    private localeService = inject(TranslocoLocaleService);
    private transloco = inject(TranslocoService);
    private destroyRef = inject(DestroyRef);
    private readonly activeLanguage = injectActiveLang();
    protected timeRanges = signal([
        { label: "", value: "24h" },
        { label: "", value: "7d" },
        { label: "", value: "30d" },
        { label: "", value: "1y" },
        { label: "", value: "custom" }
    ]);
    protected selectedRange = signal({ label: "", value: "30d" });
    private readonly autoRefreshIntervalMs = 30000;
    protected isShareMode = computed(() => this.shareService.isShareMode());
    protected isCustomRangeVisible = signal(false);
    private readonly rangeFormModel = signal({
        customRangeDates: new FormControl<Date[] | null>(null)
    });
    protected readonly rangeForm = compatForm(this.rangeFormModel);
    protected showFunnelManager = signal(false);
    protected showFunnelViewer = signal(false);
    protected selectedFunnelId = signal<string | null>(null);
    protected isAddSiteVisible = signal(false);
    protected funnelDateRange = computed(() => this.getCurrentDateRange());
    protected siteDomain = computed(() => this.siteService.activeSite()?.domain ?? null);
    protected siteFaviconUrl = computed(() => {
        const domain = this.siteDomain();
        return domain ? `/api/favicon/${encodeURIComponent(domain)}` : "";
    });
    protected activeFilters = signal<MetricFilter[]>([]);
    protected hasFilters = computed(() => this.activeFilters().length > 0);
    protected isExportingFiltered = signal(false);
    protected filteredExportState = signal<"idle" | "success" | "error">("idle");
    protected readonly exportMenuItems = computed<MenuItem[]>(() => {
        this.activeLanguage();
        return buildTakeoutExportMenuItems(this.transloco, (format) => this.exportFiltered(format));
    });
    protected filterChips = computed(() =>
        this.activeFilters().map((filter) => ({
            ...filter,
            label: this.filterLabel(filter)
        }))
    );
    protected exportUrl = computed(() => {
        const shareToken = this.shareService.token();
        const site = this.siteService.activeSite();
        const dates = this.getCurrentDateRange();
        if (!site || !dates) return "";

        const params = new URLSearchParams({
            from: dates.from,
            to: dates.to
        });
        for (const filter of this.activeFilters()) {
            params.append("filter", `${filter.type}:${filter.value}`);
        }
        if (this.isShareMode() && shareToken) {
            return `/api/share/${encodeURIComponent(shareToken)}/sites/${site.id}/hits/export?${params.toString()}`;
        }
        return `/api/sites/${site.id}/hits/export?${params.toString()}`;
    });
    protected readonly kpiCards = computed<KpiCardData[]>(() => {
        this.activeLanguage();
        const stats = this.statsService.stats();
        const loading = this.statsService.isLoading();
        const cmp = stats?.comparison;
        const baseClass = "text-2xl xl:text-3xl font-bold";
        const liveVisitors = stats?.live_visitors ?? 0;
        const bounceValue = this.localeService.localizeNumber(stats?.bounce_rate ?? 0, "decimal", undefined, {
            minimumFractionDigits: 1,
            maximumFractionDigits: 1
        });
        const pagesValue = this.localeService.localizeNumber(stats?.pages_per_session ?? 0, "decimal", undefined, {
            minimumFractionDigits: 1,
            maximumFractionDigits: 2
        });

        return [
            {
                label: this.transloco.translate("dashboard.kpis.liveVisitors"),
                value: liveVisitors,
                loading,
                valueClass: liveVisitors > 0 ? `${baseClass} text-green-600 dark:text-green-400 animate-pulse` : baseClass,
                delta: null
            },
            {
                label: this.transloco.translate("dashboard.kpis.pageviews"),
                value: stats?.total_pageviews ?? 0,
                loading,
                valueClass: baseClass,
                delta: cmp ? this.calcDelta(stats?.total_pageviews ?? 0, cmp.total_pageviews) : null
            },
            {
                label: this.transloco.translate("dashboard.kpis.uniqueSessions"),
                value: stats?.unique_sessions ?? 0,
                loading,
                valueClass: baseClass,
                delta: cmp ? this.calcDelta(stats?.unique_sessions ?? 0, cmp.unique_sessions) : null
            },
            {
                label: this.transloco.translate("dashboard.kpis.bounceRate"),
                value: `${bounceValue}%`,
                loading,
                valueClass: baseClass,
                delta: cmp ? this.calcDelta(stats?.bounce_rate ?? 0, cmp.bounce_rate) : null
            },
            {
                label: this.transloco.translate("dashboard.kpis.avgDuration"),
                value: this.formatDuration(stats?.avg_session_duration || 0),
                loading,
                valueClass: baseClass,
                delta: cmp ? this.calcDelta(stats?.avg_session_duration ?? 0, cmp.avg_session_duration) : null
            },
            {
                label: this.transloco.translate("dashboard.kpis.pagesPerSession"),
                value: pagesValue,
                loading,
                valueClass: baseClass,
                delta: cmp ? this.calcDelta(stats?.pages_per_session ?? 0, cmp.pages_per_session) : null
            }
        ];
    });

    protected openTrackingSettings() {
        if (!this.siteService.activeSite()) {
            return;
        }
        this.siteSettings.open("1");
    }
    protected readonly breadcrumbItems = computed<PageBreadcrumbItem[]>(() => {
        this.activeLanguage();
        const site = this.siteService.activeSite();
        if (!site) {
            return [{ label: this.transloco.translate("dashboard.breadcrumbOverview"), isCurrent: true }];
        }
        return [{ label: site.domain, favicon: site, isCurrent: true }];
    });

    private searchSubject = new Subject<string>();
    protected searchQuery = signal("");
    private lastTableEvent: TableLazyLoadEvent | null = null;
    protected isShortRange = computed(() => {
        if (this.selectedRange().value === "24h") return true;
        const customRangeDates = this.rangeForm.customRangeDates().value();
        if (this.selectedRange().value === "custom" && customRangeDates) {
            const d = customRangeDates;
            if (d.length === 2 && d[0] && d[1]) {
                const diff = d[1].getTime() - d[0].getTime();
                return diff < 48 * 60 * 60 * 1000;
            }
        }
        return false;
    });
    protected chartTitle = computed(() => {
        this.activeLanguage();
        const range = this.selectedRange();

        if (range.value !== "custom") {
            return this.transloco.translate("dashboard.chartTitleWithRange", { range: range.label });
        }

        const dates = this.rangeForm.customRangeDates().value();
        if (dates && dates.length === 2 && dates[0] && dates[1]) {
            const start = this.localeService.localizeDate(dates[0], undefined, { month: "short", day: "numeric" });
            const end = this.localeService.localizeDate(dates[1], undefined, { month: "short", day: "numeric", year: "numeric" });
            return this.transloco.translate("dashboard.chartTitleCustomRange", { start, end });
        }

        return this.transloco.translate("dashboard.chartTitleOverview");
    });
    constructor() {
        effect(() => {
            this.activeLanguage();
            const ranges = this.buildTimeRanges();
            const selectedValue = untracked(() => this.selectedRange().value);
            const nextSelected = ranges.find((range) => range.value === selectedValue) ?? ranges[2]!;
            this.timeRanges.set(ranges);
            this.selectedRange.set(nextSelected);
        });

        this.searchSubject.pipe(debounceTime(400), distinctUntilChanged()).subscribe((q) => {
            this.searchQuery.set(q);
            this.refreshHits();
        });

        effect(() => {
            const site = this.siteService.activeSite();
            const dates = this.getCurrentDateRange();
            if (site && dates) {
                this.loadStatsForCurrentRange();
                this.refreshHits();
            }
        });

        effect((onCleanup) => {
            const site = this.siteService.activeSite();
            const dates = this.getCurrentDateRange();
            if (!site || !dates) return;
            const timerId = setInterval(() => this.refreshStatsOnly(), this.autoRefreshIntervalMs);
            onCleanup(() => clearInterval(timerId));
        });
    }

    refreshAll() {
        this.loadStatsForCurrentRange();
        this.refreshHits();
    }

    onSearch(event: Event) {
        this.searchSubject.next((event.target as HTMLInputElement).value);
    }

    loadHits(event: TableLazyLoadEvent) {
        this.lastTableEvent = event;
        const site = this.siteService.activeSite();
        const dates = this.getCurrentDateRange();
        if (!site || !dates) return;
        const filters = this.activeFilters();

        const rows = event.rows || 10;
        const first = event.first || 0;
        const page = first / rows + 1;

        this.hitService.loadHits(site.id, dates.from, dates.to, page, rows, event.sortField as string, event.sortOrder === 1 ? "asc" : "desc", this.searchQuery(), filters);
    }

    private refreshHits() {
        if (this.lastTableEvent) {
            this.lastTableEvent.first = 0;
            this.loadHits(this.lastTableEvent);
        }
    }

    private refreshStatsOnly() {
        if (this.statsService.isLoading()) return;
        this.loadStatsForCurrentRange();
    }

    protected comparisonLabel = computed(() => {
        this.activeLanguage();
        const r = this.statsService.currentComparisonRange();
        if (!r) return "";
        const showYear = new Date(r.from).getFullYear() !== new Date().getFullYear();
        const opts = showYear ? ({ month: "short", day: "numeric", year: "numeric" } as const) : ({ month: "short", day: "numeric" } as const);
        const fmt = (d: string) => this.localeService.localizeDate(new Date(d), undefined, opts);
        return `${fmt(r.from)} – ${fmt(r.to)}`;
    });

    private loadStatsForCurrentRange() {
        const site = this.siteService.activeSite();
        const dates = this.getCurrentDateRange();
        const filters = this.activeFilters();
        if (!site || !dates) return;
        this.statsService.loadStats(site.id, dates.from, dates.to, filters, [], []);
    }

    protected calcDelta(current: number, previous: number): number | null {
        if (previous === 0) return null;
        return ((current - previous) / previous) * 100;
    }

    onRangeChange(event: RangeSelectEvent) {
        if (event.value.value === "custom") this.isCustomRangeVisible.set(true);
    }

    applyCustomRange() {
        this.isCustomRangeVisible.set(false);
        this.selectedRange.set({ ...this.selectedRange() });
    }

    protected getCurrentDateRange() {
        const range = this.selectedRange();
        const end = new Date();
        const start = new Date();

        if (range.value === "custom") {
            const d = this.rangeForm.customRangeDates().value();
            if (d && d.length === 2 && d[0] && d[1]) {
                return { from: d[0].toISOString(), to: d[1].toISOString() };
            }
            return null;
        }

        switch (range.value) {
            case "24h":
                start.setHours(end.getHours() - 24);
                break;
            case "7d":
                start.setDate(end.getDate() - 7);
                break;
            case "30d":
                start.setDate(end.getDate() - 30);
                break;
            case "1y":
                start.setFullYear(end.getFullYear() - 1);
                break;
        }
        return { from: start.toISOString(), to: end.toISOString() };
    }

    protected formatDuration(seconds: number): string {
        if (!seconds) return this.transloco.translate("common.durationSeconds", { seconds: 0 });
        const m = Math.floor(seconds / 60);
        const s = Math.floor(seconds % 60);
        if (m > 0) {
            return this.transloco.translate("common.durationMinutesSeconds", { minutes: m, seconds: s });
        }
        return this.transloco.translate("common.durationSeconds", { seconds: s });
    }

    protected openFunnelViewer(funnel: Funnel) {
        this.selectedFunnelId.set(funnel.id);
        this.showFunnelViewer.set(true);
    }

    protected applyMetricFilter(type: MetricFilterType, metric: MetricStat) {
        if (!metric.name) return;
        this.activeFilters.update((filters) => {
            const existingIndex = filters.findIndex((filter) => filter.type === type);
            if (existingIndex >= 0) {
                const existing = filters[existingIndex];
                if (existing.value === metric.name) {
                    return filters.filter((_, idx) => idx !== existingIndex);
                }
                const next = [...filters];
                next[existingIndex] = { type, value: metric.name };
                return next;
            }
            return [...filters, { type, value: metric.name }];
        });
    }

    protected clearFilter() {
        this.activeFilters.set([]);
    }

    protected removeFilter(type: MetricFilterType, value: string) {
        this.activeFilters.update((filters) => filters.filter((filter) => !(filter.type === type && filter.value === value)));
    }

    protected activeFilterValue(type: MetricFilterType): string | null {
        return this.activeFilters().find((filter) => filter.type === type)?.value ?? null;
    }

    private filterLabel(filter: MetricFilter): string {
        switch (filter.type) {
            case "path":
                return this.transloco.translate("common.filters.page", { value: filter.value });
            case "referrer":
                return this.transloco.translate("common.filters.source", { value: filter.value });
            case "device":
                return this.transloco.translate("common.filters.device", { value: filter.value });
            case "country":
                return this.transloco.translate("common.filters.country", { value: filter.value });
            default:
                return `${filter.type}: ${filter.value}`;
        }
    }

    private buildTimeRanges(): { label: string; value: string }[] {
        return [
            { label: this.transloco.translate("common.timeRanges.last24Hours"), value: "24h" },
            { label: this.transloco.translate("common.timeRanges.last7Days"), value: "7d" },
            { label: this.transloco.translate("common.timeRanges.last30Days"), value: "30d" },
            { label: this.transloco.translate("common.timeRanges.lastYear"), value: "1y" },
            { label: this.transloco.translate("common.timeRanges.customRange"), value: "custom" }
        ];
    }

    protected exportFiltered(format: TakeoutExportFormat = DEFAULT_HITS_EXPORT_FORMAT) {
        const url = this.buildExportUrl(format);
        if (!url || this.isExportingFiltered()) return;

        this.isExportingFiltered.set(true);
        this.filteredExportState.set("idle");

        this.takeoutDownloadService
            .downloadFromUrl(url, this.buildFilteredExportFilename(format))
            .pipe(
                takeUntilDestroyed(this.destroyRef),
                finalize(() => this.isExportingFiltered.set(false))
            )
            .subscribe({
                next: () => this.filteredExportState.set("success"),
                error: () => this.filteredExportState.set("error")
            });
    }

    protected buildSiteUrl(path: string | null | undefined): string | null {
        const domain = this.siteDomain();
        if (!domain || !path) return null;
        const normalized = path.startsWith("/") ? path : `/${path}`;
        return `https://${domain}${normalized}`;
    }

    protected buildReferrerUrl(referrer: string | null | undefined): string | null {
        const url = this.normalizeUrl(referrer);
        return url ? url.href : null;
    }

    // TODO: Refactor global url vanity handling at some point
    protected displayReferrerUrl(url: string | null | undefined): string {
        if (!url) return "";

        return url.replace(/^https?:\/\//, "").replace(/^www\./, "");
    }

    protected referrerDomain(referrer: string | null | undefined): string | null {
        const url = this.normalizeUrl(referrer);
        return url ? url.hostname : null;
    }

    protected faviconUrlForDomain(domain: string | null | undefined): string | null {
        return domain ? `/api/favicon/${encodeURIComponent(domain)}` : null;
    }

    private buildExportUrl(format: TakeoutExportFormat): string {
        const baseUrl = this.exportUrl();
        if (!baseUrl) return "";
        const url = new URL(baseUrl, window.location.origin);
        url.searchParams.set("format", format);
        return url.pathname + `?${url.searchParams.toString()}`;
    }

    private buildFilteredExportFilename(format: TakeoutExportFormat): string {
        const siteDomain = this.siteService.activeSite()?.domain || "site";
        const safeDomain = siteDomain
            .toLowerCase()
            .replace(/[^a-z0-9]+/g, "-")
            .replace(/(^-|-$)/g, "");
        const dateStamp = new Date().toISOString().slice(0, 10);
        return `${safeDomain || "site"}-hits-${dateStamp}.${format}`;
    }

    private normalizeUrl(raw: string | null | undefined): URL | null {
        if (!raw) return null;
        const trimmed = raw.trim();
        if (!trimmed || trimmed.toLowerCase() === "direct") return null;
        const normalized = /^https?:\/\//i.test(trimmed) ? trimmed : `https://${trimmed}`;
        try {
            return new URL(normalized);
        } catch {
            return null;
        }
    }
}
