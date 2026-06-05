import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import { CardModule } from 'primeng/card';
import { SkeletonModule } from 'primeng/skeleton';

@Component({
    selector: 'app-kpi-card',
    standalone: true,
    imports: [CardModule, SkeletonModule],
    changeDetection: ChangeDetectionStrategy.OnPush,
    template: `
        <p-card class="shadow-sm h-full border border-surface-200 dark:border-surface-700 surface-card">
            <div class="hk-kpi-card__body flex flex-col gap-2" [class.hk-kpi-card__body--highlight]="highlight() && !loading()">
                <span class="text-sm font-medium text-muted-color">{{ label() }}</span>
                <div [class]="displayClass()" [attr.aria-live]="highlight() && !loading() ? 'polite' : null">
                    @if (loading()) {
                        <p-skeleton width="60%" height="2rem" />
                    } @else {
                        {{ value() }}
                    }
                </div>
                @if (!loading() && delta() !== null) {
                    <div class="flex items-center gap-1">
                        <span [class]="deltaClass()">{{ deltaLabel() }}</span>
                    </div>
                }
            </div>
        </p-card>
    `,
    styles: [
        `
            .hk-kpi-card__body {
                border-radius: 0.5rem;
                margin: -0.25rem;
                padding: 0.25rem;
                transition:
                    background-color 160ms ease,
                    box-shadow 160ms ease;
            }

            .hk-kpi-card__body--highlight {
                animation: hk-kpi-live-update 900ms ease-out;
            }

            @keyframes hk-kpi-live-update {
                0% {
                    background: color-mix(in srgb, var(--p-primary-color) 16%, transparent);
                    box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--p-primary-color) 32%, transparent);
                }
                100% {
                    background: transparent;
                    box-shadow: inset 0 0 0 1px transparent;
                }
            }

            @media (prefers-reduced-motion: reduce) {
                .hk-kpi-card__body--highlight {
                    animation: none;
                    background: color-mix(in srgb, var(--p-primary-color) 10%, transparent);
                    box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--p-primary-color) 24%, transparent);
                }
            }
        `
    ]
})
export class KpiCard {
    label = input.required<string>();
    value = input.required<string | number>();
    loading = input<boolean>(false);
    highlight = input<boolean>(false);
    valueClass = input<string>('');
    delta = input<number | null>(null);
    invertDelta = input<boolean>(false);

    protected displayClass = computed(() => this.valueClass() || 'text-2xl xl:text-3xl font-bold');

    protected deltaClass = computed(() => {
        const d = this.delta();
        if (d === null) return '';
        const normalized = this.invertDelta() ? -d : d;
        const positive = normalized >= 0;
        return positive
            ? 'text-xs font-medium px-1.5 py-0.5 rounded-full bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400'
            : 'text-xs font-medium px-1.5 py-0.5 rounded-full bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400';
    });

    protected deltaLabel = computed(() => {
        const d = this.delta();
        if (d === null) return '';
        const normalized = this.invertDelta() ? -d : d;
        const sign = normalized >= 0 ? '+' : '';
        return `${sign}${normalized.toFixed(1)}%`;
    });
}
