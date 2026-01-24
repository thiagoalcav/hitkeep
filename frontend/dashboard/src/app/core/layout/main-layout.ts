import { Component, inject, signal } from '@angular/core';
import { Router, RouterOutlet, RouterLink, RouterLinkActive } from '@angular/router';

import { FormsModule } from '@angular/forms';
import { Brand } from '../components/brand/brand';
import { SiteSelector } from '../../features/sites/components/site-selector';
import { AddSiteDialog } from '../../features/sites/components/add-site-dialog';
import { SiteSettingsDrawer } from '../../features/sites/components/site-settings-drawer';
import { SiteService } from '../../features/sites/services/site.service';
import { PermissionService } from '../services/permission.service';
import { UserProfileService } from '../services/user-profile.service';
import { UserControls } from '../components/user-controls/user-controls';
import { ShareService } from '../services/share.service';
// PrimeNG
import { DrawerModule } from 'primeng/drawer';

@Component({
  selector: 'app-main-layout',
  standalone: true,
  host: {
    '(document:keydown)': 'handleKeyboard($event)'
  },
  imports: [
    FormsModule,
    RouterOutlet,
    RouterLink,
    RouterLinkActive,
    Brand,
    SiteSelector,
    AddSiteDialog,
    SiteSettingsDrawer,
    UserControls,
    DrawerModule
  ],
  template: `
    <div class="flex h-screen w-full bg-[var(--p-surface-ground)]">
      <!-- Sidebar (Desktop) -->
      <aside class="hidden md:flex w-64 flex-col bg-[var(--p-surface-card)] border-r border-surface-200 dark:border-surface-700 p-4 gap-6" aria-label="Main Sidebar">
        <app-brand size="small" class="px-2" />

        <div class="flex items-center gap-2">
          <app-site-selector
            class="flex-1"
            [sites]="siteService.sites()"
            [current]="siteService.activeSite()"
            [loading]="siteService.isLoading()"
            (siteSelected)="siteService.selectSite($event)"
            (addClicked)="isAddSiteVisible.set(true)"
            (settingsClicked)="openSiteSettings('0')"
            (trackingClicked)="openSiteSettings('1')" />
        </div>

        <nav class="flex-1 flex flex-col gap-1" aria-label="Primary Navigation">
          <div class="text-xs font-semibold text-muted-color uppercase px-2 mb-2" role="presentation">Analytics</div>

          <a routerLink="/dashboard"
             routerLinkActive="bg-primary-50 text-primary-700 dark:bg-primary-900/30 dark:text-primary-400"
             class="flex items-center gap-3 px-3 py-2 rounded-md font-medium transition-colors hover:bg-surface-100 dark:hover:bg-surface-800 cursor-pointer focus:outline-none focus:ring-2 focus:ring-primary-500"
             aria-label="Go to Dashboard">
            <i class="pi pi-chart-bar" aria-hidden="true"></i> <span>Dashboard</span>
          </a>
          <a routerLink="/goals"
             routerLinkActive="bg-primary-50 text-primary-700 dark:bg-primary-900/30 dark:text-primary-400"
             class="flex items-center gap-3 px-3 py-2 rounded-md font-medium transition-colors hover:bg-surface-100 dark:hover:bg-surface-800 cursor-pointer focus:outline-none focus:ring-2 focus:ring-primary-500"
             aria-label="Go to Goals">
            <i class="pi pi-flag" aria-hidden="true"></i> <span>Goals</span>
          </a>
          <a routerLink="/funnels"
             routerLinkActive="bg-primary-50 text-primary-700 dark:bg-primary-900/30 dark:text-primary-400"
             class="flex items-center gap-3 px-3 py-2 rounded-md font-medium transition-colors hover:bg-surface-100 dark:hover:bg-surface-800 cursor-pointer focus:outline-none focus:ring-2 focus:ring-primary-500"
             aria-label="Go to Funnels">
            <i class="pi pi-filter" aria-hidden="true"></i> <span>Funnels</span>
          </a>
        </nav>

      </aside>

      <!-- Main Content -->
      <main class="flex-1 flex flex-col h-full overflow-hidden relative" role="main" aria-label="Content Area">
        <!-- Mobile Header -->
        <div class="md:hidden flex items-center justify-between p-4 bg-surface-card border-b border-surface-200 dark:border-surface-700">
          <app-brand size="small" />
          <div class="flex items-center gap-2">
            <app-user-controls [showMenu]="!shareService.isShareMode()" />
            <button (click)="isMobileDrawerOpen.set(true)" class="p-2 rounded hover:bg-surface-100 dark:hover:bg-surface-800">
              <i class="pi pi-bars text-xl"></i>
            </button>
          </div>
        </div>

        <div class="flex-1 overflow-y-auto p-4 md:p-8 md:pt-4 relative">
          <router-outlet />
        </div>
      </main>

      <!-- Mobile Drawer -->
      <p-drawer [(visible)]="isMobileDrawerOpen">
        <div class="flex flex-col gap-6 h-full">
          <app-brand size="small" class="px-2" />

          <app-site-selector
            [sites]="siteService.sites()"
            [current]="siteService.activeSite()"
            [loading]="siteService.isLoading()"
            (siteSelected)="siteService.selectSite($event); isMobileDrawerOpen.set(false)"
            (addClicked)="isAddSiteVisible.set(true)" />

          <nav class="flex-1">
            <a routerLink="/dashboard" (click)="isMobileDrawerOpen.set(false)"
               routerLinkActive="bg-primary-50 text-primary-700"
               class="flex items-center gap-3 px-3 py-2 rounded-md font-medium">
              <i class="pi pi-chart-bar"></i> <span>Dashboard</span>
            </a>
            <a routerLink="/goals" (click)="isMobileDrawerOpen.set(false)"
               routerLinkActive="bg-primary-50 text-primary-700"
               class="flex items-center gap-3 px-3 py-2 rounded-md font-medium">
              <i class="pi pi-flag"></i> <span>Goals</span>
            </a>
            <a routerLink="/funnels" (click)="isMobileDrawerOpen.set(false)"
               routerLinkActive="bg-primary-50 text-primary-700"
               class="flex items-center gap-3 px-3 py-2 rounded-md font-medium">
              <i class="pi pi-filter"></i> <span>Funnels</span>
            </a>
          </nav>

        </div>
      </p-drawer>

      <!-- Add Site Dialog -->
      <app-add-site-dialog [(visible)]="isAddSiteVisible" />

      <!-- Site Settings Drawer -->
      <app-site-settings-drawer
        [(visible)]="isSiteSettingsVisible"
        [(activeTab)]="siteSettingsTab"
        [site]="siteService.activeSite()" />
    </div>
  `
})
export class MainLayout {
  private router = inject(Router);
  protected siteService = inject(SiteService);
  protected shareService = inject(ShareService);
  protected perms = inject(PermissionService);
  protected profile = inject(UserProfileService);

  // UI State
  protected isMobileDrawerOpen = signal(false);
  protected isAddSiteVisible = signal(false);
  protected isSiteSettingsVisible = signal(false);
  protected siteSettingsTab = signal('0');

  handleKeyboard(event: KeyboardEvent) {
    // Cmd/Ctrl + K opens site settings
    if ((event.metaKey || event.ctrlKey) && event.key === 'k') {
      event.preventDefault();
      this.openSiteSettings();
    }
  }

  openSiteSettings(tab = '0') {
    if (this.siteService.activeSite()) {
      this.siteSettingsTab.set(tab);
      this.isSiteSettingsVisible.set(true);
    }
  }

  constructor() {
    const currentUrl = this.router.routerState.snapshot.url;
    if (currentUrl.startsWith('/share') || this.shareService.isShareMode()) {
      return;
    }
    this.siteService.loadSites();
    this.perms.loadPermissions().subscribe();
    this.profile.loadProfile().subscribe();
  }
}
