import { ChangeDetectionStrategy, Component, computed, input, signal } from '@angular/core';

import { FormControl, ReactiveFormsModule } from '@angular/forms';
import { compatForm } from '@angular/forms/signals/compat';
import { Site } from '@models/analytics.types';
import { TranslocoPipe } from '@jsverse/transloco';
import { ButtonModule } from 'primeng/button';
import { ToggleSwitchModule } from 'primeng/toggleswitch';

@Component({
    selector: 'app-site-tracking-settings',
    standalone: true,
    imports: [ReactiveFormsModule, ButtonModule, ToggleSwitchModule, TranslocoPipe],
    template: `
        <div class="site-settings-stack">
            <section class="site-settings-card">
                <header class="site-settings-card__header">
                    <div class="site-settings-card__title-row">
                        <span class="site-settings-card__icon"><i class="pi pi-sliders-h" aria-hidden="true"></i></span>
                        <div>
                            <h3>{{ "sites.tracking.trackingCodeConfiguration" | transloco }}</h3>
                            <p>{{ "sites.tracking.description" | transloco }}</p>
                        </div>
                    </div>
                </header>
                <div class="site-settings-card__body">
                    <div class="site-settings-toggle-list">
                        <div class="site-settings-toggle-row">
                            <div class="site-settings-toggle-row__text">
                                <label id="collect-dnt-label" for="collect-dnt-switch" class="site-settings-toggle-row__title">{{ "sites.tracking.collectDntLabel" | transloco }}</label>
                                <span class="site-settings-field-hint">{{ "sites.tracking.collectDntDescription" | transloco }}</span>
                            </div>
                            <p-toggleswitch inputId="collect-dnt-switch" ariaLabelledBy="collect-dnt-label" styleClass="shrink-0" [formControl]="trackingForm.collectDnt().control()"></p-toggleswitch>
                        </div>

                        <div class="site-settings-toggle-row">
                            <div class="site-settings-toggle-row__text">
                                <label id="disable-beacon-label" for="disable-beacon-switch" class="site-settings-toggle-row__title">{{ "sites.tracking.disableBeaconLabel" | transloco }}</label>
                                <span class="site-settings-field-hint">{{ "sites.tracking.disableBeaconDescription" | transloco }}</span>
                            </div>
                            <p-toggleswitch inputId="disable-beacon-switch" ariaLabelledBy="disable-beacon-label" styleClass="shrink-0" [formControl]="trackingForm.disableBeacon().control()"></p-toggleswitch>
                        </div>

                        <div class="site-settings-subsection">
                            <h4>{{ "sites.tracking.autoTrackingTitle" | transloco }}</h4>
                            <p>{{ "sites.tracking.autoTrackingDescription" | transloco }}</p>
                        </div>

                        <div class="site-settings-toggle-row">
                            <div class="site-settings-toggle-row__text">
                                <label id="outbound-tracking-label" for="outbound-tracking-switch" class="site-settings-toggle-row__title">{{ "sites.tracking.outboundTrackingLabel" | transloco }}</label>
                                <span class="site-settings-field-hint">{{ "sites.tracking.outboundTrackingDescription" | transloco }}</span>
                            </div>
                            <p-toggleswitch inputId="outbound-tracking-switch" ariaLabelledBy="outbound-tracking-label" styleClass="shrink-0" [formControl]="trackingForm.trackOutbound().control()"></p-toggleswitch>
                        </div>

                        <div class="site-settings-toggle-row">
                            <div class="site-settings-toggle-row__text">
                                <label id="download-tracking-label" for="download-tracking-switch" class="site-settings-toggle-row__title">{{ "sites.tracking.downloadTrackingLabel" | transloco }}</label>
                                <span class="site-settings-field-hint">{{ "sites.tracking.downloadTrackingDescription" | transloco }}</span>
                            </div>
                            <p-toggleswitch inputId="download-tracking-switch" ariaLabelledBy="download-tracking-label" styleClass="shrink-0" [formControl]="trackingForm.trackDownloads().control()"></p-toggleswitch>
                        </div>

                        <div class="site-settings-toggle-row">
                            <div class="site-settings-toggle-row__text">
                                <label id="form-tracking-label" for="form-tracking-switch" class="site-settings-toggle-row__title">{{ "sites.tracking.formTrackingLabel" | transloco }}</label>
                                <span class="site-settings-field-hint">{{ "sites.tracking.formTrackingDescription" | transloco }}</span>
                            </div>
                            <p-toggleswitch inputId="form-tracking-switch" ariaLabelledBy="form-tracking-label" styleClass="shrink-0" [formControl]="trackingForm.trackForms().control()"></p-toggleswitch>
                        </div>
                    </div>
                </div>
            </section>

            <section class="site-settings-card">
                <header class="site-settings-card__header">
                    <div class="site-settings-card__title-row">
                        <span class="site-settings-card__icon"><i class="pi pi-code" aria-hidden="true"></i></span>
                        <div>
                            <h3>{{ "sites.tracking.htmlLabel" | transloco }}</h3>
                            <p>{{ "sites.tracking.trackingCodeConfiguration" | transloco }}</p>
                        </div>
                    </div>
                    <p-button [label]="copyButtonLabel() | transloco" [icon]="copyButtonIcon()" [text]="true" [type]="'button'" size="small" (onClick)="copySnippet()" />
                </header>
                <div class="site-settings-card__body">
                    <pre class="site-settings-code-panel">{{ snippetCode() }}</pre>
                </div>
            </section>
        </div>
    `,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class SiteTrackingSettings {
    site = input.required<Site | null>();
    private readonly trackingFormModel = signal({
        collectDnt: new FormControl(false, { nonNullable: true }),
        disableBeacon: new FormControl(false, { nonNullable: true }),
        trackOutbound: new FormControl(true, { nonNullable: true }),
        trackDownloads: new FormControl(true, { nonNullable: true }),
        trackForms: new FormControl(true, { nonNullable: true })
    });
    protected readonly trackingForm = compatForm(this.trackingFormModel);
    protected copyButtonLabel = signal('sites.tracking.copyCode');
    protected copyButtonIcon = signal('pi pi-copy');

    protected snippetCode = computed(() => {
        const origin = window.location.origin;

        let attrs = '';
        if (this.trackingForm.collectDnt().value()) attrs += ' data-collect-dnt="true"';
        if (this.trackingForm.disableBeacon().value()) attrs += ' data-disable-beacon="true"';
        if (!this.trackingForm.trackOutbound().value()) attrs += ' data-disable-outbound-tracking="true"';
        if (!this.trackingForm.trackDownloads().value()) attrs += ' data-disable-download-tracking="true"';
        if (!this.trackingForm.trackForms().value()) attrs += ' data-disable-form-tracking="true"';

        return `<script async src="${origin}/hk.js"${attrs}></script>`;
    });

    copySnippet() {
        navigator.clipboard.writeText(this.snippetCode()).then(() => {
            this.copyButtonLabel.set('common.copied');
            this.copyButtonIcon.set('pi pi-check');
            setTimeout(() => this.resetCopyButton(), 2000);
        });
    }

    private resetCopyButton() {
        this.copyButtonLabel.set('sites.tracking.copyCode');
        this.copyButtonIcon.set('pi pi-copy');
    }
}
