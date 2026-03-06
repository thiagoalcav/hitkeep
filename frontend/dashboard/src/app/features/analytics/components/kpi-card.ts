import { ChangeDetectionStrategy, Component, computed, input } from "@angular/core";
import { CardModule } from "primeng/card";
import { SkeletonModule } from "primeng/skeleton";

@Component({
    selector: "app-kpi-card",
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
                @if (!loading() && delta() !== null) {
                    <div class="flex items-center gap-1">
                        <span [class]="deltaClass()">{{ deltaLabel() }}</span>
                    </div>
                }
            </div>
        </p-card>
    `
})
export class KpiCard {
    label = input.required<string>();
    value = input.required<string | number>();
    loading = input<boolean>(false);
    valueClass = input<string>("");
    delta = input<number | null>(null);
    invertDelta = input<boolean>(false);

    protected displayClass = computed(() => this.valueClass() || "text-2xl xl:text-3xl font-bold");

    protected deltaClass = computed(() => {
        const d = this.delta();
        if (d === null) return "";
        const normalized = this.invertDelta() ? -d : d;
        const positive = normalized >= 0;
        return positive
            ? "text-xs font-medium px-1.5 py-0.5 rounded-full bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400"
            : "text-xs font-medium px-1.5 py-0.5 rounded-full bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400";
    });

    protected deltaLabel = computed(() => {
        const d = this.delta();
        if (d === null) return "";
        const normalized = this.invertDelta() ? -d : d;
        const sign = normalized >= 0 ? "+" : "";
        return `${sign}${normalized.toFixed(1)}%`;
    });
}
