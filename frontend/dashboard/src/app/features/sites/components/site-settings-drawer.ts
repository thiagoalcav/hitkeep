import { Component, input, model } from '@angular/core';

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

@Component({
    selector: 'app-site-settings-drawer',
    standalone: true,
    imports: [DrawerModule, TabsModule, ButtonModule, SiteGeneralSettings, SiteTrackingSettings, SiteDangerZone, SiteRetentionSettings, SiteTeamSettings],
    templateUrl: './site-settings-drawer.html',
    styleUrl: './site-settings-drawer.css'
})
export class SiteSettingsDrawer {
    visible = model<boolean>(false);
    site = input.required<Site | null>();
    activeTab = model<string>('0');

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
            return confirm('You have unsaved changes. Discard them?');
        }
        return true;
    }

    hasUnsavedChanges(): boolean {
        // TODO: Implement actual check against child components or form state
        return false;
    }
}
