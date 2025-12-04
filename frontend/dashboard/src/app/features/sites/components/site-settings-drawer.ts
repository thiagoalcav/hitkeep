import { Component, input, model } from '@angular/core';
import { CommonModule } from '@angular/common';
import { Site } from '../../../core/models/analytics.types';

// PrimeNG
import { DrawerModule } from 'primeng/drawer';
import { TabsModule } from 'primeng/tabs';
import { ButtonModule } from 'primeng/button';

// Components
import { SiteGeneralSettings } from './site-general-settings';
import { SiteTrackingSettings } from './site-tracking-settings';
import { SiteDangerZone } from './site-danger-zone';
import { SiteRetentionSettings } from './site-retention-settings';
import { SiteTeamSettings } from './site-team-settings';

@Component({
  selector: 'app-site-settings-drawer',
  standalone: true,
  imports: [
    CommonModule,
    DrawerModule,
    TabsModule,
    ButtonModule,
    SiteGeneralSettings,
    SiteTrackingSettings,
    SiteDangerZone,
    SiteRetentionSettings,
    SiteTeamSettings
  ],
  template: `
    <p-drawer
      [visible]="visible()"
      (visibleChange)="onVisibleChange($event)"
      position="right"
      [style]="{ width: '600px', maxWidth: '90vw' }">

      <div class="flex flex-col h-full">
        <div class="flex items-center gap-2 text-sm text-muted-color mb-4">
          <span>Sites</span>
          <i class="pi pi-angle-right"></i>
          <span class="font-medium text-primary">{{ site()?.domain }}</span>
          <i class="pi pi-angle-right"></i>
          <span>Settings</span>
        </div>

        <div class="flex items-center justify-between mb-6">
          <div>
            <h2 class="text-xl font-bold">Site Settings</h2>
          </div>
        </div>

        <p-tabs [(value)]="activeTab" styleClass="flex-1">
          <p-tablist>
            <p-tab value="0">
              <div class="flex items-center gap-2">
                <i class="pi pi-cog"></i>
                <span>General</span>
              </div>
            </p-tab>
            <p-tab value="1">
              <div class="flex items-center gap-2">
                <i class="pi pi-code"></i>
                <span>Tracking</span>
              </div>
            </p-tab>
            <p-tab value="2">
              <div class="flex items-center gap-2">
                <i class="pi pi-history"></i>
                <span>Retention</span>
              </div>
            </p-tab>
            <p-tab value="3">
              <div class="flex items-center gap-2">
                <i class="pi pi-users"></i>
                <span>Team</span>
              </div>
            </p-tab>
            <p-tab value="4">
              <div class="flex items-center gap-2">
                <i class="pi pi-flag"></i>
                <span>Goals</span>
              </div>
            </p-tab>
            <p-tab value="5">
              <div class="flex items-center gap-2">
                <i class="pi pi-bell"></i>
                <span>Alerts</span>
              </div>
            </p-tab>
            <p-tab value="6">
              <div class="flex items-center gap-2">
                <i class="pi pi-exclamation-triangle"></i>
                <span>Danger Zone</span>
              </div>
            </p-tab>
          </p-tablist>
          <p-tabpanels>
            <p-tabpanel value="0">
              <app-site-general-settings [site]="site()" />
            </p-tabpanel>
            <p-tabpanel value="1">
              <app-site-tracking-settings [site]="site()" />
            </p-tabpanel>
            <p-tabpanel value="2">
                <app-site-retention-settings [site]="site()" />
              </p-tabpanel>
              <p-tabpanel value="3">
                <app-site-team-settings [site]="site()" />
              </p-tabpanel>
              <p-tabpanel value="4">
                <div class="flex flex-col items-center justify-center py-12 text-center">
                  <i class="pi pi-flag text-4xl text-muted-color mb-4"></i>
                  <h3 class="font-semibold mb-2">No goals yet</h3>
                  <p class="text-sm text-muted-color mb-4">
                    Track conversions by defining goals for important actions
                  </p>
                  <p-button label="Create First Goal" icon="pi pi-plus" />
                </div>
              </p-tabpanel>
              <p-tabpanel value="5">
                <p>Alerts settings will be here.</p>
              </p-tabpanel>
              <p-tabpanel value="6">
                <app-site-danger-zone [site]="site()" />
              </p-tabpanel>
          </p-tabpanels>
        </p-tabs>
      </div>
    </p-drawer>
  `
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