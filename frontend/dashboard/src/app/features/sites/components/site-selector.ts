import { Component, input, output, ChangeDetectionStrategy, inject, effect, signal, viewChild } from "@angular/core";

import { FormControl, ReactiveFormsModule } from "@angular/forms";
import { compatForm } from "@angular/forms/signals/compat";
import { TranslocoPipe } from "@jsverse/transloco";
import { SelectModule } from "primeng/select";
import { SkeletonModule } from "primeng/skeleton";
import { ButtonModule } from "primeng/button";
import { ButtonGroup } from "primeng/buttongroup";
import { Site } from "@models/analytics.types";
import { SiteFavicon } from "@features/sites/components/site-favicon";
import { ShareDashboardLink } from "@features/share/components/share-dashboard-link";
import { ShareService } from "@services/share.service";
@Component({
    selector: "app-site-selector",
    standalone: true,
    imports: [ReactiveFormsModule, SelectModule, SkeletonModule, ButtonModule, ButtonGroup, SiteFavicon, ShareDashboardLink, TranslocoPipe],
    changeDetection: ChangeDetectionStrategy.OnPush,
    template: `
        <div class="flex flex-col gap-2 w-full" role="region" [attr.aria-label]="'sites.selector.regionAria' | transloco">
            <div class="flex items-center justify-between">
                <label for="site-dropdown" class="text-xs font-semibold text-[var(--p-text-muted-color)] uppercase"> {{ "sites.selector.sitesLabel" | transloco }} </label>
                @if (!shareService.isShareMode()) {
                    <button
                        type="button"
                        (click)="addClicked.emit()"
                        class="cursor-pointer flex items-center justify-center size-6 rounded-md border border-surface-200 dark:border-surface-700 text-muted-color hover:text-[var(--p-text-color)] hover:bg-surface-100 dark:hover:bg-surface-800 transition-colors focus:outline-none focus:ring-2 focus:ring-primary-500"
                        [attr.aria-label]="'sites.selector.addSiteAria' | transloco"
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
                            [formControl]="siteForm.selectedSite().control()"
                            [filter]="true"
                            filterBy="domain"
                            dataKey="id"
                            (onChange)="onSiteChange($event.value)"
                            optionLabel="domain"
                            [placeholder]="'sites.selector.selectPlaceholder' | transloco"
                            class="w-full text-sm"
                            [attr.aria-label]="'sites.selector.selectSiteAria' | transloco"
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
                                <p-button icon="pi pi-cog" [ariaLabel]="'sites.selector.siteSettingsAria' | transloco" [text]="true" size="small" (onClick)="settingsClicked.emit()" />
                                <p-button icon="pi pi-code" [ariaLabel]="'sites.selector.trackingCodeAria' | transloco" [text]="true" size="small" (onClick)="trackingClicked.emit()" />
                                <p-button icon="pi pi-share-alt" [ariaLabel]="'sites.selector.shareDashboardAria' | transloco" [text]="true" size="small" (onClick)="openShareDialog()" [disabled]="!current()" />
                            </p-buttonGroup>
                            @defer (when shareDialogLoaded()) {
                                <app-share-dashboard-link #shareDialog />
                            }
                        }
                    }
                </div>
            }
        </div>
    `
})
export class SiteSelector {
    protected shareService = inject(ShareService);
    private readonly shareDialog = viewChild<ShareDashboardLink>("shareDialog");
    private readonly siteFormModel = signal({
        selectedSite: new FormControl<Site | null>(null)
    });
    protected readonly siteForm = compatForm(this.siteFormModel);
    protected readonly shareDialogLoaded = signal(false);
    private readonly pendingShareDialogOpen = signal(false);

    sites = input.required<Site[]>();
    current = input<Site | null>(null);
    loading = input<boolean>(false);
    siteSelected = output<Site>();
    addClicked = output<void>();
    settingsClicked = output<void>();
    trackingClicked = output<void>();

    constructor() {
        effect(() => {
            this.siteForm.selectedSite().control().setValue(this.current(), { emitEvent: false });
        });

        effect(() => {
            if (!this.pendingShareDialogOpen()) {
                return;
            }

            const dialog = this.shareDialog();
            if (!dialog) {
                return;
            }

            dialog.open();
            this.pendingShareDialogOpen.set(false);
        });
    }

    protected onSiteChange(site: Site | null): void {
        if (!site) {
            return;
        }
        this.siteSelected.emit(site);
    }

    protected openShareDialog() {
        this.shareDialogLoaded.set(true);
        this.pendingShareDialogOpen.set(true);
    }
}
