import { Component, inject, signal, effect, input } from '@angular/core';

import { FormControl, ReactiveFormsModule } from '@angular/forms';
import { compatForm } from '@angular/forms/signals/compat';
import { TranslocoPipe } from '@jsverse/transloco';
import { ButtonModule } from 'primeng/button';
import { InputNumberModule } from 'primeng/inputnumber';
import { MessageModule } from 'primeng/message';
import { SiteService } from '@features/sites/services/site.service';
import { AnalyticsService } from '@services/analytics.service';
import { Site } from '@models/analytics.types';

@Component({
    selector: 'app-site-retention-settings',
    standalone: true,
    imports: [ReactiveFormsModule, ButtonModule, InputNumberModule, MessageModule, TranslocoPipe],
    template: `
        <div class="flex flex-col gap-4">
            <div class="bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 p-4 rounded-lg flex gap-3">
                <i class="pi pi-info-circle text-blue-600 dark:text-blue-400 mt-0.5"></i>
                <div class="text-sm text-blue-800 dark:text-blue-300">
                    <p class="font-semibold mb-1">{{ 'sites.retention.infoTitle' | transloco }}</p>
                    <p class="opacity-90">{{ 'sites.retention.infoDescription' | transloco }}</p>
                </div>
            </div>

            <div class="flex flex-col gap-2 mt-2">
                <label for="retentionDays" class="font-medium">{{ 'sites.retention.periodLabel' | transloco }}</label>
                <div class="flex items-center gap-4">
                    <p-inputnumber
                        inputId="retentionDays"
                        [formControl]="retentionForm.retentionDays().control()"
                        [min]="1"
                        [max]="3650"
                        [showButtons]="true"
                        buttonLayout="horizontal"
                        spinnerMode="horizontal"
                        decrementButtonClass="p-button-secondary"
                        incrementButtonClass="p-button-secondary"
                        incrementButtonIcon="pi pi-plus"
                        decrementButtonIcon="pi pi-minus"
                        class="w-48"
                    >
                    </p-inputnumber>
                    <span class="text-sm text-muted-color">{{ 'sites.retention.hotDataSuffix' | transloco }}</span>
                </div>
                <small class="text-xs text-muted-color">{{ 'sites.retention.defaultHint' | transloco }}</small>
            </div>

            <div class="flex justify-end mt-4 pt-4 border-t border-surface-200 dark:border-surface-700">
                <p-button [label]="'sites.retention.savePolicy' | transloco" icon="pi pi-check" (onClick)="savePolicy()" [loading]="saving()" [disabled]="!hasChanged()"> </p-button>
            </div>
        </div>
    `
})
export class SiteRetentionSettings {
    site = input.required<Site | null>();
    private siteService = inject(SiteService);
    private analyticsService = inject(AnalyticsService);

    private readonly retentionFormModel = signal({
        retentionDays: new FormControl(365, { nonNullable: true })
    });
    protected readonly retentionForm = compatForm(this.retentionFormModel);
    saving = signal(false);
    originalDays = signal<number>(365);

    constructor() {
        effect(() => {
            const site = this.site();
            if (site) {
                const days = site.data_retention_days ?? 365;
                this.retentionForm.retentionDays().control().setValue(days, { emitEvent: false });
                this.originalDays.set(days);
            }
        });
    }

    hasChanged() {
        return this.retentionForm.retentionDays().value() !== this.originalDays();
    }

    savePolicy() {
        const site = this.site();
        if (!site) return;
        const retentionDays = this.retentionForm.retentionDays().value();

        this.saving.set(true);
        this.analyticsService.updateSiteRetention(site.id, retentionDays).subscribe({
            next: () => {
                this.saving.set(false);
                this.originalDays.set(retentionDays);
                // Optionally refresh site data
                this.siteService.loadSites();
            },
            error: () => this.saving.set(false)
        });
    }
}
