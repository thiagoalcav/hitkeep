import { ChangeDetectionStrategy, Component, ElementRef, computed, effect, input, model, inject } from '@angular/core';
import { TranslocoPipe, TranslocoService } from '@jsverse/transloco';

import { INSTANCE_CAPABILITIES, SITE_CAPABILITIES } from '@core/access/capabilities';
import { Site } from '@models/analytics.types';
import { AccessService } from '@services/access.service';

// PrimeNG
import { DrawerModule } from 'primeng/drawer';
import { TabsModule } from 'primeng/tabs';

// Components
import { SiteGeneralSettings } from '@features/sites/components/site-general-settings';
import { SiteTrackingSettings } from '@features/sites/components/site-tracking-settings';
import { SiteDangerZone } from '@features/sites/components/site-danger-zone';
import { SiteRetentionSettings } from '@features/sites/components/site-retention-settings';
import { SiteTeamSettings } from '@features/sites/components/site-team-settings';
import { SiteExclusionSettings } from '@features/sites/components/site-exclusion-settings';

@Component({
    selector: 'app-site-settings-drawer',
    standalone: true,
    imports: [DrawerModule, TabsModule, SiteGeneralSettings, SiteTrackingSettings, SiteExclusionSettings, SiteDangerZone, SiteRetentionSettings, SiteTeamSettings, TranslocoPipe],
    templateUrl: './site-settings-drawer.html',
    styleUrl: './site-settings-drawer.css',
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class SiteSettingsDrawer {
    private transloco = inject(TranslocoService);
    private elementRef = inject(ElementRef<HTMLElement>);
    private access = inject(AccessService);

    visible = model<boolean>(false);
    site = input.required<Site | null>();
    activeTab = model<string>('0');

    protected readonly canManageData = computed(() => {
        const site = this.site();
        return !!site && this.access.canSite(site.id, SITE_CAPABILITIES.manageData);
    });
    protected readonly canManageFiltering = computed(() => {
        const site = this.site();
        return !!site && (this.access.canSite(site.id, SITE_CAPABILITIES.manageData) || this.access.hasInstance(INSTANCE_CAPABILITIES.manageSiteExclusions));
    });
    protected readonly canManageTeam = computed(() => {
        const site = this.site();
        return !!site && this.access.canSite(site.id, SITE_CAPABILITIES.manageTeam);
    });
    protected readonly canDeleteSite = computed(() => {
        const site = this.site();
        return !!site && this.access.canSite(site.id, SITE_CAPABILITIES.delete);
    });

    constructor() {
        effect(() => {
            if (!this.isTabAvailable(this.activeTab())) {
                this.activeTab.set('0');
            }
        });
    }

    onActiveTabChange(value: string | number | undefined) {
        const nextTab = value == null ? '0' : String(value);
        this.activeTab.set(this.isTabAvailable(nextTab) ? nextTab : '0');
        this.schedulePanelScrollReset();
    }

    private isTabAvailable(value: string): boolean {
        switch (value) {
            case '2':
                return this.canManageFiltering();
            case '3':
                return this.canManageData();
            case '4':
                return this.canManageTeam();
            case '5':
                return this.canDeleteSite();
            default:
                return true;
        }
    }

    private schedulePanelScrollReset(): void {
        [0, 50, 150].forEach((delay) => setTimeout(() => this.resetPanelScroll(), delay));
    }

    private resetPanelScroll(): void {
        const host = this.elementRef.nativeElement as HTMLElement;
        host.querySelectorAll('.p-drawer-content, .p-tabpanels, .p-tabpanel').forEach((panel) => {
            (panel as HTMLElement).scrollTo({ top: 0 });
        });
    }

    onVisibleChange(isVisible: boolean) {
        if (!isVisible) {
            // Attempting to close
            if (this.canDeactivate()) {
                this.visible.set(false);
            }
        } else {
            this.visible.set(true);
            this.schedulePanelScrollReset();
        }
    }

    canDeactivate(): boolean {
        if (this.hasUnsavedChanges()) {
            return confirm(this.transloco.translate('sites.settings.unsavedChangesConfirm'));
        }
        return true;
    }

    hasUnsavedChanges(): boolean {
        // TODO: Implement actual check against child components or form state
        return false;
    }
}
