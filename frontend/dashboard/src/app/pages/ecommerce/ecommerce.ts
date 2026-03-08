import { ChangeDetectionStrategy, Component, computed, effect, inject, linkedSignal, signal } from "@angular/core";
import { ReactiveFormsModule } from "@angular/forms";
import { finalize, forkJoin } from "rxjs";
import { TranslocoPipe, TranslocoService } from "@jsverse/transloco";
import { injectActiveLang } from "@core/i18n/active-lang";
import { TranslocoLocaleService } from "@jsverse/transloco-locale";
import { ButtonModule } from "primeng/button";
import { CardModule } from "primeng/card";
import { TableModule } from "primeng/table";
import { SiteService } from "@features/sites/services/site.service";
import { AnalyticsService } from "@core/services/analytics.service";
import { PageHeader } from "@components/page-header/page-header";
import { PageBreadcrumb, PageBreadcrumbItem } from "@components/page-breadcrumb/page-breadcrumb";
import { KpiCard } from "@features/analytics/components/kpi-card";
import { DEFAULT_RANGE_OPTIONS, RangeOption, RangeToolbar } from "@components/range-toolbar/range-toolbar";
import { MetricList } from "@features/analytics/components/metric-list";
import { SeriesChart, SeriesChartPoint, SeriesDefinition } from "@features/analytics/components/series-chart";
import { EcommerceProductStat, EcommerceSeriesPoint, EcommerceSourceStat, EcommerceSummary, MetricStat, SiteStats } from "@models/analytics.types";

type MetricFilterType = "referrer" | "device" | "country" | "utm_source";

interface MetricFilter {
    type: MetricFilterType;
    value: string;
}

interface ProductFilter {
    itemId: string;
    itemName: string;
}

@Component({
    selector: "app-ecommerce",
    imports: [ReactiveFormsModule, TranslocoPipe, ButtonModule, CardModule, TableModule, PageHeader, PageBreadcrumb, RangeToolbar, KpiCard, MetricList, SeriesChart],
    templateUrl: "./ecommerce.html",
    styleUrl: "./ecommerce.css",
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class EcommercePage {
    protected siteService = inject(SiteService);
    private analyticsService = inject(AnalyticsService);
    private localeService = inject(TranslocoLocaleService);
    private transloco = inject(TranslocoService);
    private readonly activeLanguage = injectActiveLang();

    protected readonly summary = signal<EcommerceSummary | null>(null);
    protected readonly series = signal<EcommerceSeriesPoint[]>([]);
    protected readonly products = signal<EcommerceProductStat[]>([]);
    protected readonly sources = signal<EcommerceSourceStat[]>([]);
    protected readonly filterStats = signal<SiteStats | null>(null);
    protected readonly isLoading = signal(false);

    protected readonly timeRanges = signal<RangeOption[]>(DEFAULT_RANGE_OPTIONS);
    protected readonly selectedRange = linkedSignal<RangeOption[], RangeOption>({
        source: this.timeRanges,
        computation: (ranges, previous) => {
            const value = previous?.value.value ?? "30d";
            return ranges.find((range) => range.value === value) ?? ranges[2]!;
        }
    });
    protected readonly customRangeDates = signal<Date[] | null>(null);
    protected readonly isShortRange = computed(() => {
        if (this.selectedRange().value === "24h") return true;
        const customRangeDates = this.customRangeDates();
        if (this.selectedRange().value === "custom" && customRangeDates?.length === 2 && customRangeDates[0] && customRangeDates[1]) {
            return customRangeDates[1].getTime() - customRangeDates[0].getTime() < 48 * 60 * 60 * 1000;
        }
        return false;
    });

    protected readonly activeFilters = signal<MetricFilter[]>([]);
    protected readonly selectedProduct = signal<ProductFilter | null>(null);
    protected readonly hasFilters = computed(() => this.activeFilters().length > 0 || this.selectedProduct() !== null);
    protected readonly breadcrumbItems = computed<PageBreadcrumbItem[]>(() => {
        this.activeLanguage();
        const site = this.siteService.activeSite();
        if (!site) {
            return [{ label: this.transloco.translate("nav.ecommerce"), isCurrent: true }];
        }
        return [
            { label: site.domain, favicon: site, routerLink: "/dashboard" },
            { label: this.transloco.translate("nav.ecommerce"), isCurrent: true }
        ];
    });

    protected readonly kpiCards = computed(() => {
        this.activeLanguage();
        const summary = this.summary();
        const loading = this.isLoading();
        const currency = summary?.currency || "USD";

        return [
            {
                label: this.transloco.translate("ecommerce.kpis.revenue"),
                value: summary ? this.formatCurrency(summary.revenue, currency) : this.formatCurrency(0, currency),
                loading,
                valueClass: "text-2xl xl:text-3xl font-bold"
            },
            {
                label: this.transloco.translate("ecommerce.kpis.orders"),
                value: summary?.orders ?? 0,
                loading,
                valueClass: "text-2xl xl:text-3xl font-bold"
            },
            {
                label: this.transloco.translate("ecommerce.kpis.averageOrderValue"),
                value: summary ? this.formatCurrency(summary.average_order_value, currency) : this.formatCurrency(0, currency),
                loading,
                valueClass: "text-2xl xl:text-3xl font-bold"
            },
            {
                label: this.transloco.translate("ecommerce.kpis.checkoutConversion"),
                value: `${this.formatPercent(summary?.checkout_conversion_rate ?? 0)}%`,
                loading,
                valueClass: "text-2xl xl:text-3xl font-bold"
            }
        ];
    });
    protected readonly chartData = computed<SeriesChartPoint[]>(() =>
        this.series().map((point) => ({
            time: point.time,
            revenue: point.revenue,
            orders: point.orders
        }))
    );
    protected readonly chartConfig = computed<SeriesDefinition[]>(() => {
        this.activeLanguage();
        return [
            {
                key: "revenue",
                label: this.transloco.translate("ecommerce.chart.revenue"),
                color: "#0f9d58",
                gradientFrom: "rgba(15, 157, 88, 0.45)",
                gradientTo: "rgba(15, 157, 88, 0.0)"
            },
            {
                key: "orders",
                label: this.transloco.translate("ecommerce.chart.orders"),
                color: "#2563eb",
                gradientFrom: "rgba(37, 99, 235, 0.35)",
                gradientTo: "rgba(37, 99, 235, 0.0)"
            }
        ];
    });
    protected readonly filterChips = computed(() => {
        this.activeLanguage();
        const chips = this.activeFilters().map((filter) => ({
            key: `${filter.type}:${filter.value}`,
            label: this.filterLabel(filter),
            remove: () => this.removeFilter(filter.type, filter.value)
        }));
        const product = this.selectedProduct();
        if (product) {
            chips.push({
                key: `item:${product.itemId || product.itemName}`,
                label: this.transloco.translate("ecommerce.filters.product", { value: product.itemName || product.itemId }),
                remove: () => this.selectedProduct.set(null)
            });
        }
        return chips;
    });

    constructor() {
        effect(() => {
            const site = this.siteService.activeSite();
            const dates = this.getCurrentDateRange();
            const filters = this.activeFilters();
            const product = this.selectedProduct();
            if (!site || !dates) {
                this.summary.set(null);
                this.series.set([]);
                this.products.set([]);
                this.sources.set([]);
                this.filterStats.set(null);
                return;
            }
            this.loadData(site.id, dates.from, dates.to, filters, product);
        });
    }

    protected refreshData() {
        const site = this.siteService.activeSite();
        const dates = this.getCurrentDateRange();
        if (!site || !dates) {
            return;
        }
        this.loadData(site.id, dates.from, dates.to, this.activeFilters(), this.selectedProduct());
    }

    protected applyMetricFilter(type: MetricFilterType, metric: MetricStat) {
        if (!metric.name) {
            return;
        }
        this.activeFilters.update((filters) => {
            const existingIndex = filters.findIndex((filter) => filter.type === type);
            if (existingIndex >= 0) {
                const existing = filters[existingIndex];
                if (existing.value === metric.name) {
                    return filters.filter((_, index) => index !== existingIndex);
                }
                const next = [...filters];
                next[existingIndex] = { type, value: metric.name };
                return next;
            }
            return [...filters, { type, value: metric.name }];
        });
    }

    protected activeFilterValue(type: MetricFilterType): string | null {
        return this.activeFilters().find((filter) => filter.type === type)?.value ?? null;
    }

    protected removeFilter(type: MetricFilterType, value: string) {
        this.activeFilters.update((filters) => filters.filter((filter) => !(filter.type === type && filter.value === value)));
    }

    protected clearAllFilters() {
        this.activeFilters.set([]);
        this.selectedProduct.set(null);
    }

    protected toggleProductFilter(product: EcommerceProductStat) {
        const current = this.selectedProduct();
        if (current && current.itemId === product.item_id && current.itemName === product.item_name) {
            this.selectedProduct.set(null);
            return;
        }
        this.selectedProduct.set({
            itemId: product.item_id,
            itemName: product.item_name
        });
    }

    protected isProductFilterActive(product: EcommerceProductStat): boolean {
        const current = this.selectedProduct();
        return current?.itemId === product.item_id && current?.itemName === product.item_name;
    }

    protected formatCurrency(value: number, currency: string): string {
        return new Intl.NumberFormat(this.activeLanguage(), {
            style: "currency",
            currency: currency || "USD",
            maximumFractionDigits: 2
        }).format(value);
    }

    protected formatNumber(value: number): string {
        return this.localeService.localizeNumber(value, "decimal");
    }

    protected formatPercent(value: number): string {
        return this.localeService.localizeNumber(value, "decimal", undefined, {
            minimumFractionDigits: 1,
            maximumFractionDigits: 1
        });
    }

    private loadData(siteId: string, from: string, to: string, filters: MetricFilter[], product: ProductFilter | null) {
        this.isLoading.set(true);
        forkJoin({
            summary: this.analyticsService.getEcommerceSummary(siteId, from, to, filters, product?.itemId, product?.itemName),
            series: this.analyticsService.getEcommerceTimeseries(siteId, from, to, filters, product?.itemId, product?.itemName),
            products: this.analyticsService.getEcommerceProducts(siteId, from, to, filters, product?.itemId, product?.itemName),
            sources: this.analyticsService.getEcommerceSources(siteId, from, to, filters, product?.itemId, product?.itemName),
            stats: this.analyticsService.getSiteStats(siteId, from, to, undefined, undefined, filters)
        })
            .pipe(finalize(() => this.isLoading.set(false)))
            .subscribe({
                next: ({ summary, series, products, sources, stats }) => {
                    this.summary.set(summary);
                    this.series.set(series);
                    this.products.set(products);
                    this.sources.set(sources);
                    this.filterStats.set(stats);
                },
                error: (error) => console.error(error)
            });
    }

    private filterLabel(filter: MetricFilter): string {
        switch (filter.type) {
            case "referrer":
                return this.transloco.translate("common.filters.source", { value: filter.value });
            case "device":
                return this.transloco.translate("common.filters.device", { value: filter.value });
            case "country":
                return this.transloco.translate("common.filters.country", { value: filter.value });
            case "utm_source":
                return this.transloco.translate("ecommerce.filters.utmSource", { value: filter.value });
            default:
                return `${filter.type}: ${filter.value}`;
        }
    }

    private getCurrentDateRange() {
        const range = this.selectedRange();
        const end = new Date();
        const start = new Date();

        if (range.value === "custom") {
            const dates = this.customRangeDates();
            if (dates && dates.length === 2 && dates[0] && dates[1]) {
                return { from: dates[0].toISOString(), to: dates[1].toISOString() };
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
}
