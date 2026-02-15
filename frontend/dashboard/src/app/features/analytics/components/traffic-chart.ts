import { Component, input, output, computed, inject, ChangeDetectionStrategy } from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';

import { ChartModule } from 'primeng/chart';
import { ButtonModule } from 'primeng/button';
import { TranslocoPipe, TranslocoService } from '@jsverse/transloco';
import { TranslocoLocaleService } from '@jsverse/transloco-locale';
import { ChartDataPoint } from '@models/analytics.types';
import { PreferencesService } from '@services/preferences.service';

interface ChartContext {
    chart: {
        ctx: CanvasRenderingContext2D;
    };
}

@Component({
    selector: 'app-traffic-chart',
    standalone: true,
    imports: [ChartModule, ButtonModule, TranslocoPipe],
    changeDetection: ChangeDetectionStrategy.OnPush,
    template: `
        <div class="h-80 w-full relative" role="img" [attr.aria-label]="accessibilityLabel()">
            @if (isLoading()) {
                <div class="flex items-center justify-center h-full" aria-live="polite">
                    <i class="pi pi-spin pi-spinner text-4xl text-[var(--p-primary-color)]" aria-hidden="true"></i>
                </div>
            } @else if (hasTraffic()) {
                <p-chart type="line" [data]="chartPayload()" [options]="chartOptions()" height="100%" />
            } @else {
                <div class="absolute inset-0 flex flex-col items-center justify-center text-[var(--p-text-muted-color)] bg-[var(--p-surface-ground)]/50 rounded-lg border-2 border-dashed border-[var(--p-surface-border)] p-6 text-center">
                    <h3 class="font-semibold text-[var(--p-text-color)] text-lg mb-1">{{ 'dashboard.empty.noTrafficTitle' | transloco }}</h3>
                    <p class="text-sm mb-4 max-w-xs">{{ 'dashboard.empty.noTrafficDescription' | transloco }}</p>
                    <p-button [label]="'dashboard.empty.getTrackingCode' | transloco" icon="pi pi-code" size="small" (onClick)="snippetClicked.emit()"></p-button>
                </div>
            }
        </div>
    `
})
export class TrafficChart {
    data = input.required<ChartDataPoint[]>();
    isLoading = input<boolean>(false);
    isShortRange = input<boolean>(false);

    snippetClicked = output<void>();

    private prefs = inject(PreferencesService);
    private localeService = inject(TranslocoLocaleService);
    private transloco = inject(TranslocoService);
    private activeLanguage = toSignal(this.transloco.langChanges$, { initialValue: this.transloco.getActiveLang() });

    protected hasTraffic = computed(() => {
        const d = this.data();
        // It has traffic if data exists AND at least one bucket has > 0 pageviews
        return d && d.length > 0 && d.some((p) => p.pageviews > 0);
    });

    protected accessibilityLabel = computed(() => {
        this.activeLanguage();
        const count = this.data()?.length || 0;
        return this.transloco.translate('dashboard.trafficChartAria', { count });
    });

    protected chartPayload = computed(() => {
        this.activeLanguage();
        const raw = this.data() || [];

        const labels = raw.map((d) => {
            const date = new Date(d.time);
            if (this.isShortRange()) return this.localeService.localizeDate(date, undefined, { hour: 'numeric', minute: '2-digit' });
            return this.localeService.localizeDate(date, undefined, { month: 'short', day: 'numeric' });
        });

        return {
            labels,
            datasets: [
                {
                    label: this.transloco.translate('dashboard.kpis.pageviews'),
                    data: raw.map((d) => d.pageviews),
                    fill: true,
                    backgroundColor: (ctx: ChartContext) => this.getGradient(ctx, 'rgba(99, 102, 241, 0.5)', 'rgba(99, 102, 241, 0.0)'),
                    borderColor: '#6366f1',
                    pointBackgroundColor: '#6366f1',
                    tension: 0.4,
                    borderWidth: 2,
                    pointRadius: 0,
                    pointHoverRadius: 4
                },
                {
                    label: this.transloco.translate('dashboard.traffic.visitors'),
                    data: raw.map((d) => d.visitors),
                    fill: true,
                    backgroundColor: (ctx: ChartContext) => this.getGradient(ctx, 'rgba(20, 184, 166, 0.5)', 'rgba(20, 184, 166, 0.0)'),
                    borderColor: '#14b8a6',
                    pointBackgroundColor: '#14b8a6',
                    tension: 0.4,
                    borderWidth: 2,
                    pointRadius: 0,
                    pointHoverRadius: 4
                }
            ]
        };
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
                    ticks: { color: textColor, stepSize: 1 },
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
}
