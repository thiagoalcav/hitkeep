import { Component, input, model, inject } from '@angular/core';
import { TranslocoPipe, TranslocoService } from '@jsverse/transloco';

import { Site } from '@models/analytics.types';

// PrimeNG
import { DrawerModule } from 'primeng/drawer';
import { TabsModule } from 'primeng/tabs';
import { ButtonModule } from 'primeng/button';

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
    imports: [DrawerModule, TabsModule, ButtonModule, SiteGeneralSettings, SiteTrackingSettings, SiteExclusionSettings, SiteDangerZone, SiteRetentionSettings, SiteTeamSettings, TranslocoPipe],
    templateUrl: './site-settings-drawer.html',
    styleUrl: './site-settings-drawer.css'
})
export class SiteSettingsDrawer {
    private transloco = inject(TranslocoService);

    visible = model<boolean>(false);
    site = input.required<Site | null>();
    activeTab = model<string>('0');

    onActiveTabChange(value: string | number | undefined) {
        this.activeTab.set(value == null ? '0' : String(value));
    }

    onVisibleChange(isVisible: boolean) {
        if (!isVisible) {
            // Attempting to close
            if (this.canDeactivate()) {
                this.visible.set(false);
            }
        } else {
            this.visible.set(true);
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
