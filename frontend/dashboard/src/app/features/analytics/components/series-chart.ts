import { ChangeDetectionStrategy, Component, computed, input, inject } from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { ChartModule } from 'primeng/chart';
import { TranslocoPipe, TranslocoService } from '@jsverse/transloco';
import { TranslocoLocaleService } from '@jsverse/transloco-locale';
import { PreferencesService } from '@services/preferences.service';

export type SeriesChartPoint = Record<string, number | string> & { time: string };

export interface SeriesDefinition {
    key: string;
    label: string;
    color: string;
    gradientFrom: string;
    gradientTo: string;
}

interface ChartContext {
    chart: {
        ctx: CanvasRenderingContext2D;
    };
}

@Component({
    selector: 'app-series-chart',
    changeDetection: ChangeDetectionStrategy.OnPush,
    imports: [ChartModule, TranslocoPipe],
    template: `
        <div class="h-80 w-full relative" role="img" [attr.aria-label]="accessibilityLabel()">
            @if (isLoading()) {
                <div class="flex items-center justify-center h-full" aria-live="polite">
                    <i class="pi pi-spin pi-spinner text-4xl text-[var(--p-primary-color)]" aria-hidden="true"></i>
                </div>
            } @else if (hasData()) {
                <p-chart type="line" [data]="chartPayload()" [options]="chartOptions()" height="100%" />
            } @else {
                <div class="absolute inset-0 flex flex-col items-center justify-center text-[var(--p-text-muted-color)] bg-[var(--p-surface-ground)]/50 rounded-lg border-2 border-dashed border-[var(--p-surface-border)] p-6 text-center">
                    <h3 class="font-semibold text-[var(--p-text-color)] text-lg mb-1">{{ emptyTitle() || ('common.empty.noDataTitle' | transloco) }}</h3>
                    <p class="text-sm max-w-xs">{{ emptyDescription() || ('common.empty.noDataDescription' | transloco) }}</p>
                </div>
            }
        </div>
        @if (comparisonLabel()) {
            <p class="text-xs text-[var(--p-text-muted-color)] text-right mt-2">{{ 'comparison.vsLabel' | transloco }} {{ comparisonLabel() }}</p>
        }
    `
})
export class SeriesChart {
    data = input.required<SeriesChartPoint[]>();
    series = input.required<SeriesDefinition[]>();
    comparisonData = input<SeriesChartPoint[]>([]);
    isLoading = input<boolean>(false);
    isShortRange = input<boolean>(false);
    emptyTitle = input<string>('');
    emptyDescription = input<string>('');
    comparisonLabel = input<string>('');

    private prefs = inject(PreferencesService);
    private localeService = inject(TranslocoLocaleService);
    private transloco = inject(TranslocoService);
    private activeLanguage = toSignal(this.transloco.langChanges$, { initialValue: this.transloco.getActiveLang() });

    protected hasData = computed(() => {
        const data = this.data() || [];
        const series = this.series() || [];
        if (data.length === 0 || series.length === 0) return false;
        return data.some((point) => series.some((s) => Number(point[s.key] ?? 0) > 0));
    });

    protected accessibilityLabel = computed(() => {
        this.activeLanguage();
        const count = this.data()?.length || 0;
        return this.transloco.translate('common.seriesChartAria', { count });
    });

    protected chartPayload = computed(() => {
        this.activeLanguage();
        const raw = this.data() || [];
        const cmp = this.comparisonData() || [];
        const series = this.series() || [];

        const labels = raw.map((d) => {
            const date = new Date(d.time);
            if (this.isShortRange()) return this.localeService.localizeDate(date, undefined, { hour: 'numeric', minute: '2-digit' });
            return this.localeService.localizeDate(date, undefined, { month: 'short', day: 'numeric' });
        });

        const datasets: object[] = series.map((s) => ({
            label: s.label,
            data: raw.map((d) => Number(d[s.key] ?? 0)),
            fill: true,
            backgroundColor: (ctx: ChartContext) => this.getGradient(ctx, s.gradientFrom, s.gradientTo),
            borderColor: s.color,
            pointBackgroundColor: s.color,
            tension: 0.4,
            borderWidth: 2,
            pointRadius: 0,
            pointHoverRadius: 4
        }));

        if (cmp.length > 0) {
            for (const s of series) {
                datasets.push({
                    label: `${s.label} (prev.)`,
                    data: cmp.map((d) => Number(d[s.key] ?? 0)),
                    fill: false,
                    borderColor: this.hexToRgba(s.color, 0.4),
                    pointBackgroundColor: this.hexToRgba(s.color, 0.4),
                    tension: 0.4,
                    borderWidth: 1.5,
                    pointRadius: 0,
                    pointHoverRadius: 3,
                    borderDash: [5, 5]
                });
            }
        }

        return { labels, datasets };
    });

    protected chartOptions = computed(() => {
        const isDark = this.prefs.isDarkMode();
        const textColor = isDark ? '#94a3b8' : '#64748b';
        const gridColor = isDark ? 'rgba(255, 255, 255, 0.05)' : 'rgba(0, 0, 0, 0.05)';
        const tooltipBg = isDark ? 'rgba(15, 23, 42, 0.9)' : 'rgba(255, 255, 255, 0.9)';
        const tooltipText = isDark ? '#f8fafc' : '#0f172a';
        const tooltipBorder = isDark ? '#334155' : '#e2e8f0';

        return {
            maintainAspectRatio: false,
            aspectRatio: 0.5,
            responsive: true,
            interaction: { mode: 'index', intersect: false },
            plugins: {
                legend: { labels: { color: textColor, usePointStyle: true, boxWidth: 8 }, position: 'bottom' },
                tooltip: {
                    mode: 'index',
                    intersect: false,
                    backgroundColor: tooltipBg,
                    titleColor: tooltipText,
                    bodyColor: tooltipText,
                    borderColor: tooltipBorder,
                    borderWidth: 1,
                    padding: 10,
                    cornerRadius: 8,
                    displayColors: true
                }
            },
            scales: {
                x: {
                    ticks: { color: textColor, maxTicksLimit: 8 },
                    grid: { color: gridColor, drawBorder: false, tickLength: 0 },
                    border: { display: false }
                },
                y: {
                    ticks: { color: textColor },
                    grid: { color: gridColor, drawBorder: false, tickLength: 0 },
                    border: { display: false },
                    beginAtZero: true
                }
            }
        };
    });

    private getGradient(context: ChartContext, c1: string, c2: string) {
        const ctx = context.chart.ctx;
        if (!ctx) return c1;
        const gradient = ctx.createLinearGradient(0, 0, 0, 300);
        gradient.addColorStop(0, c1);
        gradient.addColorStop(1, c2);
        return gradient;
    }

    private hexToRgba(hex: string, alpha: number): string {
        const result = /^#?([a-f\d]{2})([a-f\d]{2})([a-f\d]{2})$/i.exec(hex);
        if (!result) return hex;
        const r = parseInt(result[1]!, 16);
        const g = parseInt(result[2]!, 16);
        const b = parseInt(result[3]!, 16);
        return `rgba(${r}, ${g}, ${b}, ${alpha})`;
    }
}
