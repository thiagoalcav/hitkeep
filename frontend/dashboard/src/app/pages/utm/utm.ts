import { ChangeDetectionStrategy, Component, computed, effect, inject, signal, untracked } from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { FormControl, ReactiveFormsModule } from '@angular/forms';
import { compatForm } from '@angular/forms/signals/compat';
import { RouterLink } from '@angular/router';
import { TranslocoPipe, TranslocoService } from '@jsverse/transloco';
import { DatePickerModule } from 'primeng/datepicker';
import { DialogModule } from 'primeng/dialog';
import { ButtonModule } from 'primeng/button';
import { CardModule } from 'primeng/card';
import { SiteService } from '@features/sites/services/site.service';
import { StatsService } from '@features/analytics/services/stats.service';
import { PageHeader } from '@components/page-header/page-header';
import { PageBreadcrumb, PageBreadcrumbItem } from '@components/page-breadcrumb/page-breadcrumb';
import { KpiCard } from '@features/analytics/components/kpi-card';
import { RangeToolbar } from '@components/range-toolbar/range-toolbar';
import { MetricList } from '@features/analytics/components/metric-list';
import { SeriesChart, SeriesChartPoint, SeriesDefinition } from '@features/analytics/components/series-chart';

interface RangeSelectEvent {
    value: {
        label: string;
        value: string;
    };
}

type MetricFilterType = 'utm_campaign' | 'utm_content' | 'utm_medium' | 'utm_source' | 'utm_term';
interface MetricFilter {
    type: MetricFilterType;
    value: string;
}

@Component({
    selector: 'app-utm-dashboard',
    standalone: true,
    imports: [ReactiveFormsModule, RouterLink, TranslocoPipe, DatePickerModule, DialogModule, ButtonModule, CardModule, PageHeader, PageBreadcrumb, RangeToolbar, KpiCard, MetricList, SeriesChart],
    templateUrl: './utm.html',
    styleUrl: './utm.css',
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class UtmDashboard {
    protected siteService = inject(SiteService);
    protected statsService = inject(StatsService);
    private transloco = inject(TranslocoService);
    private activeLanguage = toSignal(this.transloco.langChanges$, { initialValue: this.transloco.getActiveLang() });

    protected timeRanges = signal([
        { label: '', value: '24h' },
        { label: '', value: '7d' },
        { label: '', value: '30d' },
        { label: '', value: '1y' },
        { label: '', value: 'custom' }
    ]);
    protected selectedRange = signal({ label: '', value: '30d' });
    protected isCustomRangeVisible = signal(false);
    protected isRefreshing = computed(() => this.statsService.isLoading());
    private readonly rangeFormModel = signal({
        customRangeDates: new FormControl<Date[] | null>(null)
    });
    protected readonly rangeForm = compatForm(this.rangeFormModel);
    protected isShortRange = computed(() => {
        if (this.selectedRange().value === '24h') return true;
        const customRangeDates = this.rangeForm.customRangeDates().value();
        if (this.selectedRange().value === 'custom' && customRangeDates) {
            const d = customRangeDates;
            if (d.length === 2 && d[0] && d[1]) {
                const diff = d[1].getTime() - d[0].getTime();
                return diff < 48 * 60 * 60 * 1000;
            }
        }
        return false;
    });
    protected activeFilters = signal<MetricFilter[]>([]);
    protected hasFilters = computed(() => this.activeFilters().length > 0);
    protected filterChips = computed(() =>
        this.activeFilters().map((filter) => ({
            ...filter,
            label: this.filterLabel(filter)
        }))
    );
    protected readonly breadcrumbItems = computed<PageBreadcrumbItem[]>(() => {
        this.activeLanguage();
        const site = this.siteService.activeSite();
        if (!site) {
            return [{ label: this.transloco.translate('nav.utm'), isCurrent: true }];
        }
        return [
            { label: site.domain, favicon: site, routerLink: '/dashboard' },
            { label: this.transloco.translate('nav.utm'), isCurrent: true }
        ];
    });
    protected readonly utmSeriesData = computed<SeriesChartPoint[]>(() => {
        const chartData = this.statsService.stats()?.chart_data ?? [];
        return chartData.map((point) => ({
            time: point.time,
            pageviews: point.pageviews,
            visitors: point.visitors
        }));
    });
    protected readonly utmSeriesConfig = computed<SeriesDefinition[]>(() => {
        this.activeLanguage();
        return [
            {
                key: 'pageviews',
                label: this.transloco.translate('dashboard.kpis.pageviews'),
                color: '#6366f1',
                gradientFrom: 'rgba(99, 102, 241, 0.5)',
                gradientTo: 'rgba(99, 102, 241, 0.0)'
            },
            {
                key: 'visitors',
                label: this.transloco.translate('dashboard.traffic.visitors'),
                color: '#14b8a6',
                gradientFrom: 'rgba(20, 184, 166, 0.5)',
                gradientTo: 'rgba(20, 184, 166, 0.0)'
            }
        ];
    });
    protected readonly utmKpis = computed(() => {
        this.activeLanguage();
        const stats = this.statsService.stats();
        const loading = this.statsService.isLoading();

        return [
            {
                label: this.transloco.translate('utm.kpis.campaign'),
                value: stats?.utm_campaign_hits ?? 0,
                loading,
                valueClass: 'text-2xl xl:text-3xl font-bold'
            },
            {
                label: this.transloco.translate('utm.kpis.content'),
                value: stats?.utm_content_hits ?? 0,
                loading,
                valueClass: 'text-2xl xl:text-3xl font-bold'
            },
            {
                label: this.transloco.translate('utm.kpis.medium'),
                value: stats?.utm_medium_hits ?? 0,
                loading,
                valueClass: 'text-2xl xl:text-3xl font-bold'
            },
            {
                label: this.transloco.translate('utm.kpis.source'),
                value: stats?.utm_source_hits ?? 0,
                loading,
                valueClass: 'text-2xl xl:text-3xl font-bold'
            },
            {
                label: this.transloco.translate('utm.kpis.term'),
                value: stats?.utm_term_hits ?? 0,
                loading,
                valueClass: 'text-2xl xl:text-3xl font-bold'
            }
        ];
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

        effect(() => {
            const site = this.siteService.activeSite();
            const dates = this.getCurrentDateRange();
            const filters = this.activeFilters();
            if (!site || !dates) {
                return;
            }
            this.statsService.loadStats(site.id, dates.from, dates.to, filters);
        });
    }

    protected refreshStats() {
        const site = this.siteService.activeSite();
        const dates = this.getCurrentDateRange();
        if (!site || !dates) {
            return;
        }
        this.statsService.loadStats(site.id, dates.from, dates.to, this.activeFilters());
    }

    protected applyMetricFilter(type: MetricFilterType, metric: { name: string }) {
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
            case 'utm_campaign':
                return this.transloco.translate('utm.filters.campaign', { value: filter.value });
            case 'utm_content':
                return this.transloco.translate('utm.filters.content', { value: filter.value });
            case 'utm_medium':
                return this.transloco.translate('utm.filters.medium', { value: filter.value });
            case 'utm_source':
                return this.transloco.translate('utm.filters.source', { value: filter.value });
            case 'utm_term':
                return this.transloco.translate('utm.filters.term', { value: filter.value });
            default:
                return `${filter.type}: ${filter.value}`;
        }
    }

    protected onRangeChange(event: RangeSelectEvent) {
        if (event.value.value === 'custom') {
            this.isCustomRangeVisible.set(true);
        }
    }

    protected applyCustomRange() {
        this.isCustomRangeVisible.set(false);
        this.selectedRange.set({ ...this.selectedRange() });
    }

    private buildTimeRanges(): { label: string; value: string }[] {
        return [
            { label: this.transloco.translate('common.timeRanges.last24Hours'), value: '24h' },
            { label: this.transloco.translate('common.timeRanges.last7Days'), value: '7d' },
            { label: this.transloco.translate('common.timeRanges.last30Days'), value: '30d' },
            { label: this.transloco.translate('common.timeRanges.lastYear'), value: '1y' },
            { label: this.transloco.translate('common.timeRanges.customRange'), value: 'custom' }
        ];
    }

    private getCurrentDateRange() {
        const range = this.selectedRange();
        const end = new Date();
        const start = new Date();

        if (range.value === 'custom') {
            const dates = this.rangeForm.customRangeDates().value();
            if (dates && dates.length === 2 && dates[0] && dates[1]) {
                return { from: dates[0].toISOString(), to: dates[1].toISOString() };
            }
            return null;
        }

        switch (range.value) {
            case '24h':
                start.setHours(end.getHours() - 24);
                break;
            case '7d':
                start.setDate(end.getDate() - 7);
                break;
            case '30d':
                start.setDate(end.getDate() - 30);
                break;
            case '1y':
                start.setFullYear(end.getFullYear() - 1);
                break;
        }

        return { from: start.toISOString(), to: end.toISOString() };
    }
}
