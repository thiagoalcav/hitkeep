import { Component, input, output, ChangeDetectionStrategy, ViewChild, inject } from '@angular/core';

import { FormsModule } from '@angular/forms';
import { SelectModule } from 'primeng/select';
import { SkeletonModule } from 'primeng/skeleton';
import { ButtonModule } from 'primeng/button';
import { ButtonGroup } from 'primeng/buttongroup';
import { Site } from '@models/analytics.types';
import { SiteFavicon } from '@features/sites/components/site-favicon';
import { ShareDashboardLink } from '@features/share/components/share-dashboard-link';
import { ShareService } from '@services/share.service';
@Component({
    selector: 'app-site-selector',
    standalone: true,
    imports: [FormsModule, SelectModule, SkeletonModule, ButtonModule, ButtonGroup, SiteFavicon, ShareDashboardLink],
    changeDetection: ChangeDetectionStrategy.OnPush,
    template: `
        <div class="flex flex-col gap-2 w-full" role="region" aria-label="Site Selection">
            <div class="flex items-center justify-between">
                <label for="site-dropdown" class="text-xs font-semibold text-[var(--p-text-muted-color)] uppercase"> Sites </label>
                @if (!shareService.isShareMode()) {
                    <button
                        type="button"
                        (click)="addClicked.emit()"
                        class="cursor-pointer flex items-center justify-center size-6 rounded-md border border-surface-200 dark:border-surface-700 text-muted-color hover:text-[var(--p-text-color)] hover:bg-surface-100 dark:hover:bg-surface-800 transition-colors focus:outline-none focus:ring-2 focus:ring-primary-500"
                        aria-label="Add a new Site"
                    >
                        <i class="pi pi-plus text-xs" aria-hidden="true"></i>
                    </button>
                }
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
                            aria-label="Select a Site to view stats"
                        >
                            <ng-template pTemplate="selectedItem" let-selected>
                                <div class="flex items-center shrink-0 grow-0 gap-2">
                                    <app-site-favicon [site]="selected" />
                                    <span class="text-sm font-medium truncate">{{ selected.domain }}</span>
                                </div>
                            </ng-template>

                            <ng-template pTemplate="item" let-site>
                                <div class="flex items-center shrink-0 grow-0 gap-2">
                                    <app-site-favicon [site]="site" />
                                    <span>{{ site.domain }}</span>
                                </div>
                            </ng-template>
                        </p-select>
                    }

                    @if (sites().length > 0) {
                        @if (!shareService.isShareMode()) {
                            <p-buttonGroup>
                                <p-button icon="pi pi-cog" ariaLabel="Site settings" [text]="true" size="small" (onClick)="settingsClicked.emit()" />
                                <p-button icon="pi pi-code" ariaLabel="Tracking code" [text]="true" size="small" (onClick)="trackingClicked.emit()" />
                                <p-button icon="pi pi-share-alt" ariaLabel="Share dashboard" [text]="true" size="small" (onClick)="openShareDialog()" [disabled]="!current()" />
                            </p-buttonGroup>
                            <app-share-dashboard-link />
                        }
                    }
                </div>
            }
        </div>
    `
})
export class SiteSelector {
    protected shareService = inject(ShareService);
    @ViewChild(ShareDashboardLink) private shareDialog?: ShareDashboardLink;

    sites = input.required<Site[]>();
    current = input<Site | null>(null);
    loading = input<boolean>(false);
    siteSelected = output<Site>();
    addClicked = output<void>();
    settingsClicked = output<void>();
    trackingClicked = output<void>();

    protected openShareDialog() {
        this.shareDialog?.open();
    }
}
