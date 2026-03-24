import { ChangeDetectionStrategy, Component, computed, effect, inject, linkedSignal, signal, untracked } from "@angular/core";
import { FormsModule } from "@angular/forms";
import { finalize, forkJoin } from "rxjs";
import { TranslocoPipe, TranslocoService } from "@jsverse/transloco";
import { TranslocoLocaleService } from "@jsverse/transloco-locale";
import { SelectModule } from "primeng/select";
import { ButtonModule } from "primeng/button";
import { CardModule } from "primeng/card";
import { SiteService } from "@features/sites/services/site.service";
import { AnalyticsService } from "@core/services/analytics.service";
import { DEFAULT_RANGE_OPTIONS, RangeOption, RangeToolbar } from "@components/range-toolbar/range-toolbar";
import { PageHeader, PageHeaderLeft } from "@components/page-header/page-header";
import { PageBreadcrumb, PageBreadcrumbItem } from "@components/page-breadcrumb/page-breadcrumb";
import { SeriesChart, SeriesChartPoint, SeriesDefinition } from "@features/analytics/components/series-chart";
import { KpiCard } from "@features/analytics/components/kpi-card";
import { MetricList } from "@features/analytics/components/metric-list";
import { EventAudience, EventSeriesPoint, MetricStat } from "@models/analytics.types";
import { injectActiveLang } from "@core/i18n/active-lang";
import { calcDelta, ChatbotMetricKey, ChatbotSeriesState, computeComparisonPeriod, createEmptySeries, safeRate, totalFor } from "@pages/ai-chatbots/ai-chatbots.utils";

type ScopeKey = "provider" | "bot_id" | "surface" | "model";
interface ScopeFilter {
    key: ScopeKey;
    value: string;
}

interface FilterChip {
    key: "scope";
    label: string;
}

const CHATBOT_EVENTS: Record<ChatbotMetricKey, string> = {
    started: "assistant.chat_started",
    sent: "assistant.message_sent",
    rendered: "assistant.response_rendered",
    clicked: "assistant.citation_clicked",
    handoff: "assistant.handoff_requested",
    assisted: "assistant.goal_assisted"
};

@Component({
    selector: "app-ai-chatbots",
    imports: [FormsModule, TranslocoPipe, SelectModule, ButtonModule, CardModule, RangeToolbar, PageHeader, PageHeaderLeft, PageBreadcrumb, SeriesChart, KpiCard, MetricList],
    templateUrl: "./ai-chatbots.html",
    styleUrl: "./ai-chatbots.css",
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class AIChatbots {
    protected readonly docsUrl = "https://hitkeep.com/guides/analytics/ai-chatbot-analytics/";
    private readonly siteService = inject(SiteService);
    private readonly analyticsService = inject(AnalyticsService);
    private readonly localeService = inject(TranslocoLocaleService);
    private readonly transloco = inject(TranslocoService);
    private readonly activeLanguage = injectActiveLang();

    protected readonly timeRanges = signal<RangeOption[]>(DEFAULT_RANGE_OPTIONS);
    protected readonly selectedRange = linkedSignal<RangeOption[], RangeOption>({
        source: this.timeRanges,
        computation: (ranges, previous) => {
            const value = previous?.value.value ?? "30d";
            return ranges.find((r) => r.value === value) ?? ranges[2]!;
        }
    });
    protected readonly customRangeDates = signal<Date[] | null>(null);
    protected readonly comparisonRange = signal<{ from: string; to: string } | null>(null);

    protected readonly selectedScopeKey = signal<ScopeKey>("provider");
    protected readonly selectedScopeValue = signal<string | null>(null);
    protected readonly activeScopeFilter = computed<ScopeFilter | null>(() => {
        const value = this.selectedScopeValue();
        if (!value) return null;
        return { key: this.selectedScopeKey(), value };
    });
    protected readonly scopeValues = signal<MetricStat[]>([]);
    protected readonly topIntents = signal<MetricStat[]>([]);
    protected readonly topProviders = signal<MetricStat[]>([]);
    protected readonly topSurfaces = signal<MetricStat[]>([]);
    protected readonly audience = signal<EventAudience | null>(null);

    protected readonly series = signal<ChatbotSeriesState>(createEmptySeries());
    protected readonly comparisonSeries = signal<ChatbotSeriesState>(createEmptySeries());

    protected readonly isLoadingBreakdowns = signal(false);
    protected readonly isLoadingSeries = signal(false);
    protected readonly isLoadingComparison = signal(false);
    protected readonly isLoadingAudience = signal(false);

    protected readonly activeSite = computed(() => this.siteService.activeSite());
    protected readonly noSite = computed(() => !this.activeSite());
    protected readonly isLoading = computed(() => this.isLoadingSeries() || this.isLoadingComparison() || this.isLoadingBreakdowns() || this.isLoadingAudience());

    protected readonly scopeKeyOptions = computed(() => {
        this.activeLanguage();
        return [
            { label: this.transloco.translate("aiChatbots.filters.provider"), value: "provider" },
            { label: this.transloco.translate("aiChatbots.filters.botId"), value: "bot_id" },
            { label: this.transloco.translate("aiChatbots.filters.surface"), value: "surface" },
            { label: this.transloco.translate("aiChatbots.filters.model"), value: "model" }
        ] satisfies { label: string; value: ScopeKey }[];
    });
    protected readonly scopeValueOptions = computed(() => this.scopeValues().map((item) => ({ label: item.name, value: item.name })));

    protected readonly breadcrumbItems = computed<PageBreadcrumbItem[]>(() => {
        this.activeLanguage();
        const site = this.activeSite();
        if (!site) return [{ label: this.transloco.translate("aiChatbots.title"), isCurrent: true }];
        return [
            { label: site.domain, favicon: site, routerLink: "/dashboard" },
            { label: this.transloco.translate("aiChatbots.title"), isCurrent: true }
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

    protected readonly chartSeries = computed<SeriesChartPoint[]>(() => {
        const state = this.series();
        const byTime = new Map<string, SeriesChartPoint>();
        const merge = (points: EventSeriesPoint[], key: ChatbotMetricKey) => {
            for (const point of points) {
                const current = byTime.get(point.time) ?? { time: point.time, started: 0, rendered: 0, handoff: 0, assisted: 0 };
                current[key] = point.count;
                byTime.set(point.time, current);
            }
        };
        merge(state.started, "started");
        merge(state.rendered, "rendered");
        merge(state.handoff, "handoff");
        merge(state.assisted, "assisted");
        return [...byTime.values()].sort((a, b) => new Date(a.time).getTime() - new Date(b.time).getTime());
    });

    protected readonly comparisonChartSeries = computed<SeriesChartPoint[]>(() => {
        const state = this.comparisonSeries();
        const byTime = new Map<string, SeriesChartPoint>();
        const merge = (points: EventSeriesPoint[], key: ChatbotMetricKey) => {
            for (const point of points) {
                const current = byTime.get(point.time) ?? { time: point.time, started: 0, rendered: 0, handoff: 0, assisted: 0 };
                current[key] = point.count;
                byTime.set(point.time, current);
            }
        };
        merge(state.started, "started");
        merge(state.rendered, "rendered");
        merge(state.handoff, "handoff");
        merge(state.assisted, "assisted");
        return [...byTime.values()].sort((a, b) => new Date(a.time).getTime() - new Date(b.time).getTime());
    });

    protected readonly chartConfig = computed<SeriesDefinition[]>(() => {
        this.activeLanguage();
        return [
            { key: "started", label: this.transloco.translate("aiChatbots.kpis.conversations"), color: "#0f766e", gradientFrom: "rgba(15, 118, 110, 0.4)", gradientTo: "rgba(15, 118, 110, 0.0)" },
            { key: "rendered", label: this.transloco.translate("aiChatbots.kpis.responses"), color: "#2563eb", gradientFrom: "rgba(37, 99, 235, 0.35)", gradientTo: "rgba(37, 99, 235, 0.0)" },
            { key: "handoff", label: this.transloco.translate("aiChatbots.kpis.handoffs"), color: "#dc2626", gradientFrom: "rgba(220, 38, 38, 0.3)", gradientTo: "rgba(220, 38, 38, 0.0)" },
            { key: "assisted", label: this.transloco.translate("aiChatbots.kpis.assistedConversions"), color: "#a16207", gradientFrom: "rgba(161, 98, 7, 0.3)", gradientTo: "rgba(161, 98, 7, 0.0)" }
        ];
    });

    protected readonly comparisonLabel = computed(() => {
        this.activeLanguage();
        const range = this.comparisonRange();
        if (!range) return "";
        const showYear = new Date(range.from).getFullYear() !== new Date().getFullYear();
        const options = showYear ? ({ month: "short", day: "numeric", year: "numeric" } as const) : ({ month: "short", day: "numeric" } as const);
        const format = (value: string) => this.localeService.localizeDate(new Date(value), undefined, options);
        return `${format(range.from)} – ${format(range.to)}`;
    });

    protected readonly filterChips = computed<FilterChip[]>(() => {
        const value = this.selectedScopeValue();
        if (!value) return [];
        this.activeLanguage();
        const keyLabel = this.scopeKeyOptions().find((item) => item.value === this.selectedScopeKey())?.label ?? this.selectedScopeKey();
        return [{ key: "scope", label: `${keyLabel}: ${value}` }];
    });

    protected readonly totalStarted = computed(() => this.totalFor("started", this.series()));
    protected readonly totalSent = computed(() => this.totalFor("sent", this.series()));
    protected readonly totalRendered = computed(() => this.totalFor("rendered", this.series()));
    protected readonly totalClicked = computed(() => this.totalFor("clicked", this.series()));
    protected readonly totalHandoff = computed(() => this.totalFor("handoff", this.series()));
    protected readonly totalAssisted = computed(() => this.totalFor("assisted", this.series()));

    protected readonly comparisonStarted = computed(() => this.totalFor("started", this.comparisonSeries()));
    protected readonly comparisonSent = computed(() => this.totalFor("sent", this.comparisonSeries()));
    protected readonly comparisonRendered = computed(() => this.totalFor("rendered", this.comparisonSeries()));
    protected readonly comparisonClicked = computed(() => this.totalFor("clicked", this.comparisonSeries()));
    protected readonly comparisonHandoff = computed(() => this.totalFor("handoff", this.comparisonSeries()));
    protected readonly comparisonAssisted = computed(() => this.totalFor("assisted", this.comparisonSeries()));

    protected readonly handoffRate = computed(() => this.safeRate(this.totalHandoff(), this.totalStarted()));
    protected readonly comparisonHandoffRate = computed(() => this.safeRate(this.comparisonHandoff(), this.comparisonStarted()));
    protected readonly citationCtr = computed(() => this.safeRate(this.totalClicked(), this.totalRendered()));
    protected readonly comparisonCitationCtr = computed(() => this.safeRate(this.comparisonClicked(), this.comparisonRendered()));

    protected readonly kpiCards = computed(() => {
        this.activeLanguage();
        return [
            this.kpi(this.transloco.translate("aiChatbots.kpis.conversations"), this.totalStarted(), this.calcDelta(this.totalStarted(), this.comparisonStarted())),
            this.kpi(this.transloco.translate("aiChatbots.kpis.prompts"), this.totalSent(), this.calcDelta(this.totalSent(), this.comparisonSent())),
            this.kpi(this.transloco.translate("aiChatbots.kpis.responses"), this.totalRendered(), this.calcDelta(this.totalRendered(), this.comparisonRendered())),
            this.kpi(this.transloco.translate("aiChatbots.kpis.assistedConversions"), this.totalAssisted(), this.calcDelta(this.totalAssisted(), this.comparisonAssisted())),
            this.kpi(this.transloco.translate("aiChatbots.kpis.handoffRate"), `${this.handoffRate().toFixed(1)}%`, this.calcDelta(this.handoffRate(), this.comparisonHandoffRate())),
            this.kpi(this.transloco.translate("aiChatbots.kpis.citationCtr"), `${this.citationCtr().toFixed(1)}%`, this.calcDelta(this.citationCtr(), this.comparisonCitationCtr()))
        ];
    });

    constructor() {
        effect(() => {
            this.selectedScopeKey();
            untracked(() => {
                if (this.selectedScopeValue() !== null) {
                    this.selectedScopeValue.set(null);
                }
            });
        });

        effect(() => {
            const site = this.activeSite();
            this.selectedRange();
            this.selectedScopeKey();
            if (!site) {
                this.scopeValues.set([]);
                return;
            }
            const dates = this.getCurrentDateRange();
            if (!dates) return;
            this.loadScopeValues(site.id, dates.from, dates.to, this.selectedScopeKey());
        });

        effect(() => {
            const site = this.activeSite();
            this.selectedRange();
            const scopeFilter = this.activeScopeFilter();
            if (!site) {
                this.series.set(createEmptySeries());
                this.comparisonSeries.set(createEmptySeries());
                this.topIntents.set([]);
                this.topProviders.set([]);
                this.topSurfaces.set([]);
                this.audience.set(null);
                this.comparisonRange.set(null);
                return;
            }

            const dates = this.getCurrentDateRange();
            if (!dates) return;
            const comparison = computeComparisonPeriod(dates.from, dates.to);
            this.comparisonRange.set(comparison);

            this.loadPrimaryData(site.id, dates.from, dates.to, scopeFilter, () => this.loadComparisonData(site.id, comparison.from, comparison.to, scopeFilter));
            this.loadAudience(site.id, dates.from, dates.to, scopeFilter);
        });
    }

    protected clearFilters() {
        this.selectedScopeValue.set(null);
    }

    private loadPrimaryData(siteId: string, from: string, to: string, scopeFilter: ScopeFilter | null, onSettled?: () => void) {
        this.isLoadingSeries.set(true);
        this.isLoadingBreakdowns.set(true);
        const propertyKey = scopeFilter?.key;
        const propertyValue = scopeFilter?.value;

        forkJoin({
            started: this.analyticsService.getEventTimeseries(siteId, from, to, CHATBOT_EVENTS.started, propertyKey, propertyValue),
            sent: this.analyticsService.getEventTimeseries(siteId, from, to, CHATBOT_EVENTS.sent, propertyKey, propertyValue),
            rendered: this.analyticsService.getEventTimeseries(siteId, from, to, CHATBOT_EVENTS.rendered, propertyKey, propertyValue),
            clicked: this.analyticsService.getEventTimeseries(siteId, from, to, CHATBOT_EVENTS.clicked, propertyKey, propertyValue),
            handoff: this.analyticsService.getEventTimeseries(siteId, from, to, CHATBOT_EVENTS.handoff, propertyKey, propertyValue),
            assisted: this.analyticsService.getEventTimeseries(siteId, from, to, CHATBOT_EVENTS.assisted, propertyKey, propertyValue),
            intents: this.analyticsService.getEventPropertyBreakdown(siteId, from, to, CHATBOT_EVENTS.sent, "intent"),
            providers: this.analyticsService.getEventPropertyBreakdown(siteId, from, to, CHATBOT_EVENTS.started, "provider"),
            surfaces: this.analyticsService.getEventPropertyBreakdown(siteId, from, to, CHATBOT_EVENTS.started, "surface")
        })
            .pipe(
                finalize(() => {
                    this.isLoadingSeries.set(false);
                    this.isLoadingBreakdowns.set(false);
                    onSettled?.();
                })
            )
            .subscribe({
                next: (data) => {
                    this.series.set({
                        started: data.started ?? [],
                        sent: data.sent ?? [],
                        rendered: data.rendered ?? [],
                        clicked: data.clicked ?? [],
                        handoff: data.handoff ?? [],
                        assisted: data.assisted ?? []
                    });
                    this.topIntents.set(data.intents ?? []);
                    this.topProviders.set(data.providers ?? []);
                    this.topSurfaces.set(data.surfaces ?? []);
                },
                error: () => {
                    this.series.set(createEmptySeries());
                    this.topIntents.set([]);
                    this.topProviders.set([]);
                    this.topSurfaces.set([]);
                }
            });
    }

    private loadComparisonData(siteId: string, from: string, to: string, scopeFilter: ScopeFilter | null) {
        this.isLoadingComparison.set(true);
        const propertyKey = scopeFilter?.key;
        const propertyValue = scopeFilter?.value;

        forkJoin({
            started: this.analyticsService.getEventTimeseries(siteId, from, to, CHATBOT_EVENTS.started, propertyKey, propertyValue),
            sent: this.analyticsService.getEventTimeseries(siteId, from, to, CHATBOT_EVENTS.sent, propertyKey, propertyValue),
            rendered: this.analyticsService.getEventTimeseries(siteId, from, to, CHATBOT_EVENTS.rendered, propertyKey, propertyValue),
            clicked: this.analyticsService.getEventTimeseries(siteId, from, to, CHATBOT_EVENTS.clicked, propertyKey, propertyValue),
            handoff: this.analyticsService.getEventTimeseries(siteId, from, to, CHATBOT_EVENTS.handoff, propertyKey, propertyValue),
            assisted: this.analyticsService.getEventTimeseries(siteId, from, to, CHATBOT_EVENTS.assisted, propertyKey, propertyValue)
        })
            .pipe(finalize(() => this.isLoadingComparison.set(false)))
            .subscribe({
                next: (data) => {
                    this.comparisonSeries.set({
                        started: data.started ?? [],
                        sent: data.sent ?? [],
                        rendered: data.rendered ?? [],
                        clicked: data.clicked ?? [],
                        handoff: data.handoff ?? [],
                        assisted: data.assisted ?? []
                    });
                },
                error: () => this.comparisonSeries.set(createEmptySeries())
            });
    }

    private loadScopeValues(siteId: string, from: string, to: string, key: ScopeKey) {
        this.isLoadingBreakdowns.set(true);
        this.analyticsService
            .getEventPropertyBreakdown(siteId, from, to, CHATBOT_EVENTS.started, key)
            .pipe(finalize(() => this.isLoadingBreakdowns.set(false)))
            .subscribe({
                next: (data) => this.scopeValues.set(data ?? []),
                error: () => this.scopeValues.set([])
            });
    }

    private loadAudience(siteId: string, from: string, to: string, scopeFilter: ScopeFilter | null) {
        this.isLoadingAudience.set(true);
        const propertyKey = scopeFilter?.key;
        const propertyValue = scopeFilter?.value;

        this.analyticsService
            .getEventAudience(siteId, from, to, CHATBOT_EVENTS.started, propertyKey, propertyValue)
            .pipe(finalize(() => this.isLoadingAudience.set(false)))
            .subscribe({
                next: (data) => this.audience.set(data),
                error: () => this.audience.set(null)
            });
    }

    protected getCurrentDateRange(): { from: string; to: string } | null {
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
            case "1y":
                start.setFullYear(end.getFullYear() - 1);
                break;
            default:
                start.setDate(end.getDate() - 30);
        }
        return { from: start.toISOString(), to: end.toISOString() };
    }

    private totalFor(key: ChatbotMetricKey, state: ChatbotSeriesState): number {
        return totalFor(key, state);
    }

    private safeRate(numerator: number, denominator: number): number {
        return safeRate(numerator, denominator);
    }

    private calcDelta(current: number, previous: number): number | null {
        return calcDelta(current, previous);
    }

    private kpi(label: string, value: number | string, delta: number | null): { label: string; value: number | string; delta: number | null } {
        return {
            label,
            value,
            delta
        };
    }
}
