import { Component, computed, effect, inject, input, signal } from '@angular/core';
import { finalize } from 'rxjs';

import { Site } from '@models/analytics.types';
import { PermissionService } from '@services/permission.service';
import { UserProfileService } from '@services/user-profile.service';
import { SiteService } from '@features/sites/services/site.service';
import { ButtonModule } from 'primeng/button';
import { InputTextModule } from 'primeng/inputtext';

@Component({
    selector: 'app-site-danger-zone',
    standalone: true,
    imports: [ButtonModule, InputTextModule],
    template: `
        <div class="flex flex-col gap-6">
            @if (canDeleteSite()) {
                <div class="border border-red-200 rounded-lg p-4 bg-red-50">
                    <div class="flex flex-col gap-4">
                        <div>
                            <h4 class="font-semibold text-red-700 mb-1">Delete Site</h4>
                            <p class="text-sm text-red-600">Permanently deletes this site and all associated analytics data.</p>
                        </div>
                        <div>
                            <label class="text-sm font-medium text-red-700 mb-2 block" for="delete-site-confirm"> Type "{{ site()?.domain }}" to confirm </label>
                            <input id="delete-site-confirm" pInputText class="w-full" [value]="confirmValue()" #confirmInput (input)="confirmValue.set(confirmInput.value)" placeholder="example.com" />
                        </div>
                        <div class="flex items-center justify-end">
                            <p-button label="Delete Site" icon="pi pi-trash" severity="danger" [disabled]="!canConfirmDelete()" [loading]="isDeleting()" (onClick)="deleteSite()" />
                        </div>
                    </div>
                </div>
            }
        </div>
    `
})
export class SiteDangerZone {
    private perms = inject(PermissionService);
    private profile = inject(UserProfileService);
    private siteService = inject(SiteService);

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
                    alert('Failed to delete site.');
                }
            });
    }
}
