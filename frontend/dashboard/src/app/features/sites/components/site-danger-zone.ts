import { ChangeDetectionStrategy, Component, computed, effect, inject, input, signal } from '@angular/core';
import { finalize } from 'rxjs';
import { TranslocoPipe, TranslocoService } from '@jsverse/transloco';

import { Site } from '@models/analytics.types';
import { PermissionService } from '@services/permission.service';
import { UserProfileService } from '@services/user-profile.service';
import { SiteService } from '@features/sites/services/site.service';
import { ButtonModule } from 'primeng/button';
import { InputTextModule } from 'primeng/inputtext';

@Component({
    selector: 'app-site-danger-zone',
    standalone: true,
    imports: [ButtonModule, InputTextModule, TranslocoPipe],
    template: `
        <div class="site-settings-stack">
            @if (canDeleteSite()) {
                <section class="site-settings-card site-settings-card--danger">
                    <header class="site-settings-card__header">
                        <div class="site-settings-card__title-row">
                            <span class="site-settings-card__icon site-settings-card__icon--danger"><i class="pi pi-trash" aria-hidden="true"></i></span>
                            <div>
                                <h3>{{ 'sites.danger.deleteTitle' | transloco }}</h3>
                                <p>{{ 'sites.danger.deleteDescription' | transloco }}</p>
                            </div>
                        </div>
                    </header>
                    <div class="site-settings-card__body">
                        <div class="site-settings-field">
                            <label for="delete-site-confirm">{{ 'sites.danger.confirmLabel' | transloco: { domain: site()?.domain } }}</label>
                            <input id="delete-site-confirm" pInputText class="w-full" [value]="confirmValue()" #confirmInput (input)="confirmValue.set(confirmInput.value)" [placeholder]="'sites.danger.confirmPlaceholder' | transloco" />
                        </div>
                    </div>
                    <footer class="site-settings-card__footer">
                        <p-button styleClass="site-settings-danger-action" [label]="'sites.danger.deleteAction' | transloco" icon="pi pi-trash" severity="danger" [disabled]="!canConfirmDelete()" [loading]="isDeleting()" (onClick)="deleteSite()" />
                    </footer>
                </section>
            }
        </div>
    `,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class SiteDangerZone {
    private perms = inject(PermissionService);
    private profile = inject(UserProfileService);
    private siteService = inject(SiteService);
    private transloco = inject(TranslocoService);

    site = input.required<Site | null>();
    protected isDeleting = signal(false);
    protected confirmValue = signal('');
    protected canDeleteSite = computed(() => {
        const site = this.site();
        if (!site) return false;

        const perms = this.perms.permissions();
        if (perms?.permissions?.[site.id] === 'owner') {
            return true;
        }

        const profile = this.profile.profile();
        return !!profile && profile.id === site.user_id;
    });
    protected canConfirmDelete = computed(() => {
        const site = this.site();
        if (!site) return false;
        return this.confirmValue().trim().toLowerCase() === site.domain.toLowerCase();
    });

    constructor() {
        effect(() => {
            const site = this.site();
            if (site) {
                this.confirmValue.set('');
            }
        });
    }

    deleteSite() {
        const site = this.site();
        if (!site || this.isDeleting()) return;
        if (!this.canConfirmDelete()) return;

        this.isDeleting.set(true);
        this.siteService
            .deleteSite(site.id)
            .pipe(finalize(() => this.isDeleting.set(false)))
            .subscribe({
                error: (err) => {
                    console.error('Failed to delete site', err);
                    alert(this.transloco.translate('sites.danger.deleteFailed'));
                }
            });
    }
}
