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
            <div class="flex flex-col gap-2">
                <span class="text-sm font-medium text-muted-color">{{ label() }}</span>
                <div [class]="displayClass()">
                    @if (loading()) {
                        <p-skeleton width="60%" height="2rem" />
                    } @else {
                        {{ value() }}
                    }
                </div>
            </div>
        </p-card>
    `
})
export class KpiCard {
    label = input.required<string>();
    value = input.required<string | number>();
    loading = input<boolean>(false);
    valueClass = input<string>('');

    protected displayClass = computed(() => this.valueClass() || 'text-2xl xl:text-3xl font-bold');
}
