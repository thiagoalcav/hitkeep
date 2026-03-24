import { ChangeDetectionStrategy, Component, computed, DestroyRef, effect, inject, linkedSignal, signal } from "@angular/core";
import { takeUntilDestroyed } from "@angular/core/rxjs-interop";
import { FormsModule } from "@angular/forms";
import { finalize, forkJoin } from "rxjs";
import { TranslocoPipe, TranslocoService } from "@jsverse/transloco";
import { TranslocoDecimalPipe, TranslocoLocaleService } from "@jsverse/transloco-locale";
import { ButtonModule } from "primeng/button";
import { CardModule } from "primeng/card";
import { SelectModule } from "primeng/select";
import { SplitButtonModule } from "primeng/splitbutton";
import { TableModule } from "primeng/table";
import { MenuItem } from "primeng/api";
import { SiteService } from "@features/sites/services/site.service";
import { AIFetchFilters, AnalyticsService } from "@core/services/analytics.service";
import { DEFAULT_RANGE_OPTIONS, RangeOption, RangeToolbar } from "@components/range-toolbar/range-toolbar";
import { PageHeader, PageHeaderLeft } from "@components/page-header/page-header";
import { PageBreadcrumb, PageBreadcrumbItem } from "@components/page-breadcrumb/page-breadcrumb";
import { SeriesChart, SeriesChartPoint, SeriesDefinition } from "@features/analytics/components/series-chart";
import { KpiCard } from "@features/analytics/components/kpi-card";
import { MetricList } from "@features/analytics/components/metric-list";
import { AIFetchCorrelationReport, AIFetchOverview, MetricStat } from "@models/analytics.types";
import { injectActiveLang } from "@core/i18n/active-lang";
import { buildTakeoutExportMenuItems, DEFAULT_HITS_EXPORT_FORMAT, TakeoutExportFormat } from "@core/export/export-formats";
import { TakeoutDownloadService } from "@services/takeout-download.service";
import { AIFilterChip, formatBytes, formatResponseMs, mapAIFetchSeries } from "@pages/ai-visibility/ai-visibility.utils";

type FilterKey = "assistantName" | "assistantFamily" | "resourceType";

@Component({
    selector: "app-ai-visibility",
    imports: [FormsModule, TranslocoPipe, TranslocoDecimalPipe, ButtonModule, CardModule, SelectModule, SplitButtonModule, TableModule, RangeToolbar, PageHeader, PageHeaderLeft, PageBreadcrumb, SeriesChart, KpiCard, MetricList],
    templateUrl: "./ai-visibility.html",
    styleUrl: "./ai-visibility.css",
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class AIVisibility {
    protected readonly docsUrl = "https://hitkeep.com/guides/analytics/ai-visibility/";
    private readonly siteService = inject(SiteService);
    private readonly analyticsService = inject(AnalyticsService);
    private readonly transloco = inject(TranslocoService);
    private readonly localeService = inject(TranslocoLocaleService);
    private readonly takeoutDownloadService = inject(TakeoutDownloadService);
    private readonly destroyRef = inject(DestroyRef);
    private readonly activeLanguage = injectActiveLang();

    protected readonly timeRanges = signal<RangeOption[]>(DEFAULT_RANGE_OPTIONS);
    protected readonly selectedRange = linkedSignal<RangeOption[], RangeOption>({
        source: this.timeRanges,
        computation: (ranges, previous) => {
            const value = previous?.value.value ?? "30d";
            return ranges.find((range) => range.value === value) ?? ranges[2]!;
        }
    });
    protected readonly customRangeDates = signal<Date[] | null>(null);

    protected readonly filters = signal<AIFetchFilters>({});

    protected readonly overview = signal<AIFetchOverview | null>(null);
    protected readonly correlation = signal<AIFetchCorrelationReport | null>(null);
    protected readonly series = signal<SeriesChartPoint[]>([]);

    protected readonly isLoadingOverview = signal(false);
    protected readonly isLoadingSeries = signal(false);
    protected readonly isLoadingCorrelation = signal(false);
    protected readonly isLoading = computed(() => this.isLoadingOverview() || this.isLoadingSeries() || this.isLoadingCorrelation());

    protected readonly isExporting = signal(false);
    protected readonly exportState = signal<"idle" | "success" | "error">("idle");

    protected readonly activeSite = computed(() => this.siteService.activeSite());
    protected readonly noSite = computed(() => !this.activeSite());

    protected readonly breadcrumbItems = computed<PageBreadcrumbItem[]>(() => {
        this.activeLanguage();
        const site = this.activeSite();
        if (!site) return [{ label: this.transloco.translate("aiVisibility.title"), isCurrent: true }];
        return [
            { label: site.domain, favicon: site, routerLink: "/dashboard" },
            { label: this.transloco.translate("aiVisibility.title"), isCurrent: true }
        ];
    });

    protected readonly isShortRange = computed(() => {
        if (this.selectedRange().value === "24h") return true;
        const dates = this.customRangeDates();
        if (this.selectedRange().value === "custom" && dates && dates.length === 2 && dates[0] && dates[1]) {
            return dates[1].getTime() - dates[0].getTime() < 48 * 60 * 60 * 1000;
        }
        return false;
    });

    protected readonly assistantOptions = computed(() => this.toOptions(this.overview()?.top_assistants ?? []));
    protected readonly familyOptions = computed(() => this.toOptions(this.overview()?.top_families ?? []));
    protected readonly resourceTypeOptions = computed(() => {
        const values = new Set(["html", "document", "image", "other", ...(this.overview()?.resource_type_split ?? []).map((item) => item.name)]);
        return [...values].filter(Boolean).map((value) => ({ label: value, value }));
    });

    protected readonly filterChips = computed<AIFilterChip[]>(() => {
        this.activeLanguage();
        const filters = this.filters();
        const chips: AIFilterChip[] = [];
        if (filters.assistantName) {
            chips.push({ key: "assistantName", label: `${this.transloco.translate("aiVisibility.filters.assistant")}: ${filters.assistantName}` });
        }
        if (filters.assistantFamily) {
            chips.push({ key: "assistantFamily", label: `${this.transloco.translate("aiVisibility.filters.family")}: ${filters.assistantFamily}` });
        }
        if (filters.resourceType) {
            chips.push({ key: "resourceType", label: `${this.transloco.translate("aiVisibility.filters.resourceType")}: ${filters.resourceType}` });
        }
        return chips;
    });

    protected readonly chartConfig = computed<SeriesDefinition[]>(() => {
        this.activeLanguage();
        return [
            {
                key: "count",
                label: this.transloco.translate("aiVisibility.chart.fetches"),
                color: "#0f766e",
                gradientFrom: "rgba(15, 118, 110, 0.4)",
                gradientTo: "rgba(15, 118, 110, 0.0)"
            }
        ];
    });

    protected readonly primaryKpiCards = computed(() => {
        this.activeLanguage();
        const overview = this.overview();
        const loading = this.isLoadingOverview();
        return [
            { label: this.transloco.translate("aiVisibility.kpis.totalFetches"), value: overview?.total_requests ?? 0, loading },
            { label: this.transloco.translate("aiVisibility.kpis.uniquePaths"), value: overview?.unique_paths ?? 0, loading },
            { label: this.transloco.translate("aiVisibility.kpis.uniqueAssistants"), value: overview?.unique_assistants ?? 0, loading },
            { label: this.transloco.translate("aiVisibility.kpis.errorRate4xx"), value: `${(overview?.error_rate_4xx ?? 0).toFixed(1)}%`, loading }
        ];
    });

    protected readonly secondaryKpiCards = computed(() => {
        this.activeLanguage();
        const overview = this.overview();
        const loading = this.isLoadingOverview();
        return [
            { label: this.transloco.translate("aiVisibility.kpis.errorRate5xx"), value: `${(overview?.error_rate_5xx ?? 0).toFixed(1)}%`, loading },
            { label: this.transloco.translate("aiVisibility.kpis.medianResponse"), value: this.formatResponse(overview?.median_response_ms ?? 0), loading },
            { label: this.transloco.translate("aiVisibility.kpis.bytesServed"), value: this.formatBytes(overview?.total_bytes ?? 0), loading }
        ];
    });

    protected readonly correlationKpis = computed(() => {
        this.activeLanguage();
        const summary = this.correlation()?.summary;
        const loading = this.isLoadingCorrelation();
        return [
            { label: this.transloco.translate("aiVisibility.correlation.kpis.correlatedPaths"), value: summary?.correlated_paths ?? 0, loading },
            { label: this.transloco.translate("aiVisibility.correlation.kpis.aiVisits"), value: summary?.ai_referred_visits ?? 0, loading },
            { label: this.transloco.translate("aiVisibility.correlation.kpis.uncorrelatedFetches"), value: summary?.uncorrelated_fetches ?? 0, loading }
        ];
    });

    protected readonly citationYieldRows = computed(() => this.correlation()?.citation_yield ?? []);
    protected readonly opportunityRows = computed(() => this.correlation()?.opportunity_pages ?? []);
    protected readonly failureHotspotRows = computed(() => this.correlation()?.failure_hotspots ?? []);

    protected readonly exportUrl = computed(() => {
        const site = this.activeSite();
        const dates = this.getCurrentDateRange();
        if (!site || !dates) return "";

        const params = new URLSearchParams({ from: dates.from, to: dates.to });
        const filters = this.filters();
        if (filters.assistantName) params.set("assistant_name", filters.assistantName);
        if (filters.assistantFamily) params.set("assistant_family", filters.assistantFamily);
        if (filters.resourceType) params.set("resource_type", filters.resourceType);
        return `/api/sites/${site.id}/ai-fetch/export?${params.toString()}`;
    });

    protected readonly exportMenuItems = computed<MenuItem[]>(() => {
        this.activeLanguage();
        return buildTakeoutExportMenuItems(this.transloco, (format) => this.exportFiltered(format));
    });

    constructor() {
        effect(() => {
            const site = this.activeSite();
            this.selectedRange();
            const filters = this.filters();

            if (!site) {
                this.overview.set(null);
                this.correlation.set(null);
                this.series.set([]);
                return;
            }

            const dates = this.getCurrentDateRange();
            if (!dates) return;
            this.loadData(site.id, dates.from, dates.to, filters);
        });
    }

    protected clearFilters() {
        this.filters.set({});
    }

    protected clearFilter(key: FilterKey) {
        this.filters.update((current) => ({
            ...current,
            [key]: null
        }));
    }

    protected refreshData() {
        const site = this.activeSite();
        const dates = this.getCurrentDateRange();
        if (!site || !dates) return;
        this.loadData(site.id, dates.from, dates.to, this.filters());
    }

    protected filterValue(key: FilterKey): string | null {
        return this.filters()[key] ?? null;
    }

    protected setFilter(key: FilterKey, value: string | null) {
        this.filters.update((current) => ({
            ...current,
            [key]: value || null
        }));
    }

    protected formatPercent(value: number): string {
        return `${value.toFixed(1)}%`;
    }

    protected formatBytes(value: number): string {
        return formatBytes(value, this.localeTag());
    }

    protected formatResponse(value: number): string {
        return formatResponseMs(value, this.localeTag());
    }

    protected exportFiltered(format: TakeoutExportFormat = DEFAULT_HITS_EXPORT_FORMAT) {
        const url = this.buildExportUrl(format);
        if (!url || this.isExporting()) return;

        this.isExporting.set(true);
        this.exportState.set("idle");

        this.takeoutDownloadService
            .downloadFromUrl(url, this.buildExportFilename(format))
            .pipe(
                takeUntilDestroyed(this.destroyRef),
                finalize(() => this.isExporting.set(false))
            )
            .subscribe({
                next: () => this.exportState.set("success"),
                error: () => this.exportState.set("error")
            });
    }

    private loadData(siteId: string, from: string, to: string, filters: AIFetchFilters) {
        this.isLoadingOverview.set(true);
        this.isLoadingSeries.set(true);
        this.isLoadingCorrelation.set(true);

        forkJoin({
            overview: this.analyticsService.getAIFetchOverview(siteId, from, to, filters).pipe(finalize(() => this.isLoadingOverview.set(false))),
            timeseries: this.analyticsService.getAIFetchTimeseries(siteId, from, to, filters).pipe(finalize(() => this.isLoadingSeries.set(false))),
            correlation: this.analyticsService.getAIFetchCorrelation(siteId, from, to, filters).pipe(finalize(() => this.isLoadingCorrelation.set(false)))
        }).subscribe({
            next: ({ overview, timeseries, correlation }) => {
                this.overview.set(overview);
                this.series.set(mapAIFetchSeries(timeseries));
                this.correlation.set(correlation);
            },
            error: () => {
                this.overview.set({
                    total_requests: 0,
                    unique_paths: 0,
                    unique_assistants: 0,
                    error_rate_4xx: 0,
                    error_rate_5xx: 0,
                    median_response_ms: 0,
                    total_bytes: 0,
                    top_assistants: [],
                    top_families: [],
                    top_paths: [],
                    top_error_paths: [],
                    resource_type_split: []
                });
                this.correlation.set({
                    summary: {
                        total_fetches: 0,
                        fetched_paths: 0,
                        correlated_paths: 0,
                        ai_referred_visits: 0,
                        uncorrelated_fetches: 0
                    },
                    citation_yield: [],
                    opportunity_pages: [],
                    failure_hotspots: []
                });
                this.series.set([]);
            }
        });
    }

    private getCurrentDateRange(): { from: string; to: string } | null {
        const range = this.selectedRange().value;
        const now = new Date();

        if (range === "custom") {
            const dates = this.customRangeDates();
            if (!dates || dates.length !== 2 || !dates[0] || !dates[1]) return null;
            return {
                from: dates[0].toISOString(),
                to: dates[1].toISOString()
            };
        }

        const from = new Date(now);
        switch (range) {
            case "24h":
                from.setHours(now.getHours() - 24);
                break;
            case "7d":
                from.setDate(now.getDate() - 7);
                break;
            case "30d":
                from.setDate(now.getDate() - 30);
                break;
            case "90d":
                from.setDate(now.getDate() - 90);
                break;
            default:
                from.setDate(now.getDate() - 30);
        }

        return { from: from.toISOString(), to: now.toISOString() };
    }

    private toOptions(items: MetricStat[]) {
        return items.map((item) => ({ label: item.name, value: item.name }));
    }

    private localeTag(): string {
        const locale = this.localeService.getLocale();
        if (typeof locale === "string" && locale.trim()) {
            return locale;
        }
        return this.activeLanguage();
    }

    private buildExportUrl(format: TakeoutExportFormat): string {
        const baseUrl = this.exportUrl();
        if (!baseUrl) return "";
        const url = new URL(baseUrl, window.location.origin);
        url.searchParams.set("format", format);
        return url.pathname + `?${url.searchParams.toString()}`;
    }

    private buildExportFilename(format: TakeoutExportFormat): string {
        const siteDomain = this.activeSite()?.domain || "site";
        const safeDomain = siteDomain
            .toLowerCase()
            .replace(/[^a-z0-9]+/g, "-")
            .replace(/(^-|-$)/g, "");
        const dateStamp = new Date().toISOString().slice(0, 10);
        return `${safeDomain || "site"}-ai-fetches-${dateStamp}.${format}`;
    }
}
