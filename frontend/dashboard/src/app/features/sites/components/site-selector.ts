import { Component, input, output, ChangeDetectionStrategy } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { SelectModule } from 'primeng/select';
import { ButtonModule } from 'primeng/button';
import { SkeletonModule } from 'primeng/skeleton';
import { Site } from '../../../core/models/analytics.types';
import {SiteFavicon} from './site-favicon';
@Component({
  selector: 'app-site-selector',
  standalone: true,
  imports: [CommonModule, FormsModule, SelectModule, ButtonModule, SkeletonModule, SiteFavicon],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="flex flex-col gap-2 w-full" role="region" aria-label="Site Selection">
      <label for="site-dropdown" class="text-xs font-semibold text-[var(--p-text-muted-color)] uppercase px-2">
        Site
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

          <p-button
            [label]="sites().length > 0 ? 'New Site' : 'Create Site'"
            [icon]="sites().length > 0 ? 'pi pi-plus' : 'pi pi-plus-circle'"
            [size]="sites().length > 0 ? 'small' : 'large'"
            styleClass="w-full"
            (onClick)="addClicked.emit()"
            ariaLabel="Create a new Site configuration" />
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
