import { Component, input, output, ChangeDetectionStrategy } from '@angular/core';

import { FormsModule } from '@angular/forms';
import { SelectModule } from 'primeng/select';
import { SkeletonModule } from 'primeng/skeleton';
import { Site } from '../../../core/models/analytics.types';
import {SiteFavicon} from './site-favicon';
@Component({
  selector: 'app-site-selector',
  standalone: true,
  imports: [FormsModule, SelectModule, SkeletonModule, SiteFavicon],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="flex flex-col gap-2 w-full" role="region" aria-label="Site Selection">
      <div class="flex items-center justify-between">
        <label for="site-dropdown" class="text-xs font-semibold text-[var(--p-text-muted-color)] uppercase">
          Sites
        </label>
        <button
          type="button"
          (click)="addClicked.emit()"
          class="cursor-pointer flex items-center justify-center size-6 rounded-md border border-surface-200 dark:border-surface-700 text-muted-color hover:text-[var(--p-text-color)] hover:bg-surface-100 dark:hover:bg-surface-800 transition-colors focus:outline-none focus:ring-2 focus:ring-primary-500"
          aria-label="Add a new Site">
          <i class="pi pi-plus text-xs" aria-hidden="true"></i>
        </button>
      </div>

      @if (loading()) {
        <p-skeleton height="40px" class="rounded-md" />
      } @else {
        <div class="flex flex-col gap-2">
          @if (sites().length > 0) {
            <p-select
              inputId="site-dropdown"
              [options]="sites()"
              [ngModel]="current()"
              [filter]="true"
              filterBy="domain"
              (ngModelChange)="siteSelected.emit($event)"
              optionLabel="domain"
              placeholder="Select Site"
              class="w-full text-sm"
              aria-label="Select a Site to view stats">

              <ng-template pTemplate="selectedItem" let-selected>
                <div class="flex items-center shrink-0 grow-0 gap-2">
                  <app-site-favicon [site]="selected"/>
                  <span class="text-sm font-medium truncate">{{ selected.domain }}</span>
                </div>
              </ng-template>

              <ng-template pTemplate="item" let-site>
                <div class="flex items-center shrink-0 grow-0 gap-2">
                  <app-site-favicon [site]="site"/>
                  <span>{{ site.domain }}</span>
                </div>
              </ng-template>

            </p-select>
          }

          @if (sites().length > 0) {
            <div class="flex items-center gap-1 px-1">
              <button
                (click)="settingsClicked.emit()"
                class="cursor-pointer flex-1 flex items-center justify-center gap-2 p-1.5 text-xs font-medium text-muted-color hover:text-color hover:bg-surface-100 dark:hover:bg-surface-800 rounded transition-colors focus:outline-none focus:ring-2 focus:ring-primary-500"
                aria-label="Open Site Settings">
                <i class="pi pi-cog"></i>
                <span>Settings</span>
              </button>
              <div class="w-px h-4 bg-surface-200 dark:bg-surface-700"></div>
              <button
                (click)="trackingClicked.emit()"
                class="cursor-pointer flex-1 flex items-center justify-center gap-2 p-1.5 text-xs font-medium text-muted-color hover:text-color hover:bg-surface-100 dark:hover:bg-surface-800 rounded transition-colors focus:outline-none focus:ring-2 focus:ring-primary-500"
                aria-label="Get Tracking Code">
                <i class="pi pi-code"></i>
                <span>Code</span>
              </button>
            </div>
          }

        </div>
      }
    </div>
  `
})
export class SiteSelector {
  sites = input.required<Site[]>();
  current = input<Site | null>(null);
  loading = input<boolean>(false);
  siteSelected = output<Site>();
  addClicked = output<void>();
  settingsClicked = output<void>();
  trackingClicked = output<void>();
}
