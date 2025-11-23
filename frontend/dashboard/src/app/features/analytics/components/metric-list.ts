import { Component, input, computed, ChangeDetectionStrategy } from '@angular/core';
import { CommonModule, DecimalPipe } from '@angular/common';
import { CardModule } from 'primeng/card';
import { SkeletonModule } from 'primeng/skeleton';
import { MetricStat } from '../../../core/models/analytics.types';

@Component({
  selector: 'app-metric-list',
  standalone: true,
  imports: [CommonModule, CardModule, SkeletonModule, DecimalPipe],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <p-card class="shadow-sm h-full border border-surface-200 dark:border-surface-700 surface-card">
      <div class="flex items-center gap-2 mb-4">
        <i [class]="'pi ' + icon() + ' text-[var(--p-primary-color)]'" aria-hidden="true"></i>
        <h3 class="font-semibold text-lg">{{ title() }}</h3>
      </div>

      @if (isLoading()) {
        <div class="flex flex-col gap-3">
          @for (i of [1, 2, 3, 4, 5]; track i) {
            <p-skeleton height="1.5rem" styleClass="w-full" />
          }
        </div>
      } @else if (!data() || data().length === 0) {
        <div class="text-muted-color text-sm italic py-4 text-center">No data available</div>
      } @else {
        <ul class="flex flex-col gap-3 m-0 p-0 list-none">
          @for (item of data(); track item.name) {
            <li class="relative flex items-center justify-between text-sm group">
              <!-- Background Bar -->
              <div class="absolute left-0 top-0 h-full bg-[var(--p-primary-50)] dark:bg-[var(--p-primary-900)]/30 rounded-r transition-all duration-500"
                   [style.width.%]="(item.value / maxValue()) * 100"></div>

              <!-- Content -->
              <span class="relative z-10 truncate font-medium px-2 py-1" [title]="item.name">
                {{ item.name }}
              </span>
              <span class="relative z-10 font-semibold text-[var(--p-text-color)] px-2">
                {{ item.value | number }}
              </span>
            </li>
          }
        </ul>
      }
    </p-card>
  `
})
export class MetricList {
  title = input.required<string>();
  icon = input<string>('pi-list');
  data = input.required<MetricStat[]>();
  isLoading = input<boolean>(false);

  protected maxValue = computed(() => {
    const list = this.data();
    if (!list || list.length === 0) return 0;
    return Math.max(...list.map(i => i.value));
  });
}
