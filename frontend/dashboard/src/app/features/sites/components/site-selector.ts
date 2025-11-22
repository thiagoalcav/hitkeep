import { Component, input, output, ChangeDetectionStrategy } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { SelectModule } from 'primeng/select';
import { ButtonModule } from 'primeng/button';
import { SkeletonModule } from 'primeng/skeleton';
import { Site } from '../../../core/models/analytics.types';

@Component({
  selector: 'app-site-selector',
  standalone: true,
  imports: [CommonModule, FormsModule, SelectModule, ButtonModule, SkeletonModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="flex flex-col gap-2 w-full" role="region" aria-label="Site Selection">
      <label for="site-dropdown" class="text-xs font-semibold text-[var(--p-text-muted-color)] uppercase px-2">
        Website
      </label>

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
              class="w-full"
              aria-label="Select a website to view stats" />

            <p-button
              label="New Site"
              icon="pi pi-plus"
              size="small"
              variant="outlined"
              styleClass="w-full"
              (onClick)="addClicked.emit()"
              ariaLabel="Create a new website configuration" />
          } @else {
            <p-button label="Create Website" icon="pi pi-plus" (onClick)="addClicked.emit()" />
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
}
