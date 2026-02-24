import { Component, input, computed, signal } from "@angular/core";

import { FormControl, ReactiveFormsModule } from "@angular/forms";
import { compatForm } from "@angular/forms/signals/compat";
import { Site } from "@models/analytics.types";
import { TranslocoPipe } from "@jsverse/transloco";
import { ToggleSwitchModule } from "primeng/toggleswitch";

@Component({
    selector: "app-site-tracking-settings",
    standalone: true,
    imports: [ReactiveFormsModule, ToggleSwitchModule, TranslocoPipe],
    template: `
        <div class="flex flex-col gap-6">
            <div class="text-[var(--p-text-muted-color)] leading-relaxed">
                <p>{{ "sites.tracking.description" | transloco }}</p>
            </div>

            <div class="flex flex-col gap-4 py-4">
                <h4 class="sr-only">{{ "sites.tracking.trackingCodeConfiguration" | transloco }}</h4>

                <div class="flex items-center justify-between">
                    <div class="flex flex-col">
                        <span class="font-medium">{{ "sites.tracking.collectDntLabel" | transloco }}</span>
                        <span class="text-xs text-[var(--p-text-muted-color)]">{{ "sites.tracking.collectDntDescription" | transloco }}</span>
                    </div>
                    <p-toggleswitch [formControl]="trackingForm.collectDnt().control()"></p-toggleswitch>
                </div>

                <div class="flex items-center justify-between">
                    <div class="flex flex-col">
                        <span class="font-medium">{{ "sites.tracking.disableBeaconLabel" | transloco }}</span>
                        <span class="text-xs text-[var(--p-text-muted-color)]">{{ "sites.tracking.disableBeaconDescription" | transloco }}</span>
                    </div>
                    <p-toggleswitch [formControl]="trackingForm.disableBeacon().control()"></p-toggleswitch>
                </div>
            </div>

            <div class="rounded-md border border-[var(--p-surface-border)] bg-[var(--p-surface-50)] dark:bg-[var(--p-surface-900)] overflow-hidden">
                <div class="flex justify-between items-center px-3 py-2 border-b border-[var(--p-surface-border)] bg-[var(--p-surface-100)] dark:bg-[var(--p-surface-800)]">
                    <span class="text-xs font-mono font-medium text-[var(--p-text-muted-color)]">{{ "sites.tracking.htmlLabel" | transloco }}</span>
                    <button
                        class="flex items-center gap-2 px-3 py-1.5 rounded hover:bg-[var(--p-surface-200)] dark:hover:bg-[var(--p-surface-700)] transition-colors text-xs font-medium text-[var(--p-text-color)] cursor-pointer focus:outline-none focus:ring-2 focus:ring-[var(--p-primary-color)]"
                        (click)="copySnippet()"
                        [attr.aria-label]="copyButtonLabel() | transloco"
                    >
                        <i [class]="copyButtonIcon()"></i>
                        <span>{{ copyButtonLabel() | transloco }}</span>
                    </button>
                </div>

                <pre class="p-4 m-0 text-sm overflow-x-auto font-mono whitespace-pre-wrap break-all text-[var(--p-text-color)]">{{ snippetCode() }}</pre>
            </div>
        </div>
    `
})
export class SiteTrackingSettings {
    site = input.required<Site | null>();
    private readonly trackingFormModel = signal({
        collectDnt: new FormControl(false, { nonNullable: true }),
        disableBeacon: new FormControl(false, { nonNullable: true })
    });
    protected readonly trackingForm = compatForm(this.trackingFormModel);
    protected copyButtonLabel = signal("sites.tracking.copyCode");
    protected copyButtonIcon = signal("pi pi-copy");

    protected snippetCode = computed(() => {
        const origin = window.location.origin;

        let attrs = "";
        if (this.trackingForm.collectDnt().value()) attrs += ' data-collect-dnt="true"';
        if (this.trackingForm.disableBeacon().value()) attrs += ' data-disable-beacon="true"';

        return `<script async src="${origin}/hk.js"${attrs}></script>`;
    });

    copySnippet() {
        navigator.clipboard.writeText(this.snippetCode()).then(() => {
            this.copyButtonLabel.set("common.copied");
            this.copyButtonIcon.set("pi pi-check");
            setTimeout(() => this.resetCopyButton(), 2000);
        });
    }

    private resetCopyButton() {
        this.copyButtonLabel.set("sites.tracking.copyCode");
        this.copyButtonIcon.set("pi pi-copy");
    }
}
