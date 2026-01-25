import { Component, input, output, ChangeDetectionStrategy } from '@angular/core';

import { CardModule } from 'primeng/card';
import { ButtonModule } from 'primeng/button';
import { SkeletonModule } from 'primeng/skeleton';
import { TooltipModule } from 'primeng/tooltip';
import { Funnel } from '@models/analytics.types';
import { EmptyState } from '@components/molecules/empty-state';

@Component({
    selector: 'app-funnel-list',
    standalone: true,
    imports: [CardModule, ButtonModule, SkeletonModule, TooltipModule, EmptyState],
    changeDetection: ChangeDetectionStrategy.OnPush,
    template: `
        <p-card class="shadow-sm h-full border border-surface-200 dark:border-surface-700 surface-card">
            <div class="flex items-center justify-between mb-4">
                <div class="flex items-center gap-2">
                    <i class="pi pi-filter text-[var(--p-primary-color)]" aria-hidden="true"></i>
                    <h3 class="font-semibold text-lg">Funnels</h3>
                </div>
                @if (!readOnly() && !isLoading() && funnels() && funnels().length > 0) {
                    <p-button icon="pi pi-plus" (onClick)="manageClicked.emit()" [rounded]="true" [text]="true" pTooltip="Manage Funnels" styleClass="w-8 h-8" />
                }
            </div>

            @if (isLoading()) {
                <div class="flex flex-col gap-3">
                    @for (i of [1, 2]; track i) {
                        <p-skeleton height="3rem" styleClass="w-full" />
                    }
                </div>
            } @else if (!funnels() || funnels().length === 0) {
                <app-empty-state icon="pi-filter" title="No funnels yet" description="Build a path of events to measure drop-off and conversions." [actionLabel]="readOnly() ? '' : 'Create Funnel'" (actionClicked)="manageClicked.emit()" />
            } @else {
                <ul class="flex flex-col gap-3 m-0 p-0 list-none">
                    @for (funnel of funnels(); track funnel.id) {
                        <li>
                            <button
                                type="button"
                                class="w-full flex items-center justify-between text-sm p-3 border border-surface-100 dark:border-surface-800 rounded hover:bg-surface-50 dark:hover:bg-surface-800 transition-colors text-left"
                                (click)="funnelClicked.emit(funnel)"
                            >
                                <div class="flex flex-col gap-1 min-w-0">
                                    <span class="font-medium truncate">{{ funnel.name }}</span>
                                    <div class="flex items-center gap-1 text-xs text-muted-color">
                                        <span>{{ funnel.steps.length }} steps</span>
                                    </div>
                                </div>
                                <i class="pi pi-chevron-right text-muted-color" aria-hidden="true"></i>
                            </button>
                        </li>
                    }
                </ul>
            }
        </p-card>
    `
})
export class FunnelList {
    funnels = input.required<Funnel[]>();
    isLoading = input<boolean>(false);
    readOnly = input<boolean>(false);
    manageClicked = output<void>();
    funnelClicked = output<Funnel>();
}
