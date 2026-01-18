import { Component, inject, signal, computed, HostListener } from '@angular/core';
import { RouterOutlet, RouterLink, RouterLinkActive, Router } from '@angular/router';

import { FormsModule } from '@angular/forms';
import { Brand } from '../components/brand/brand';
import { SiteSelector } from '../../features/sites/components/site-selector';
import { AddSiteDialog } from '../../features/sites/components/add-site-dialog';
import { SiteSettingsDrawer } from '../../features/sites/components/site-settings-drawer';
import { PreferencesService } from '../services/preferences.service';
import { SiteService } from '../../features/sites/services/site.service';
import { PermissionService } from '../services/permission.service';
// PrimeNG
import { AvatarModule } from 'primeng/avatar';
import { ButtonModule } from 'primeng/button';
import { MenuModule } from 'primeng/menu';
import { MenuItem } from 'primeng/api';
import { DrawerModule } from 'primeng/drawer';

@Component({
  selector: 'app-main-layout',
  standalone: true,
  imports: [
    FormsModule,
    RouterOutlet,
    RouterLink,
    RouterLinkActive,
    Brand,
    SiteSelector,
    AddSiteDialog,
    SiteSettingsDrawer,
    AvatarModule,
    ButtonModule,
    MenuModule,
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

        <!-- Footer -->
        <div class="border-t border-surface-200 dark:border-surface-700 pt-4 flex items-center justify-between px-2">
          <div class="flex items-center gap-2">
            <p-avatar icon="pi pi-user" shape="circle" styleClass="bg-surface-200 dark:bg-surface-700" aria-hidden="true" />
            <div class="flex flex-col">
              <span class="text-sm font-medium">Admin</span>
            </div>
          </div>

          <div class="flex gap-1">
            <button (click)="prefs.toggleTheme()"
                    class="p-2 rounded-full hover:cursor-pointer hover:bg-surface-200 dark:hover:bg-surface-800 text-muted-color focus:outline-none focus:ring-2 focus:ring-primary-500"
                    [attr.aria-label]="prefs.isDarkMode() ? 'Switch to Light Mode' : 'Switch to Dark Mode'">
              <i class="pi" [class]="prefs.isDarkMode() ? 'pi-moon' : 'pi-sun'" aria-hidden="true"></i>
            </button>
            <button (click)="userMenu.toggle($event)"
                    class="p-2 rounded-full hover:cursor-pointer hover:bg-surface-200 dark:hover:bg-surface-800 text-muted-color focus:outline-none focus:ring-2 focus:ring-primary-500"
                    aria-haspopup="true" aria-label="User menu">
              <i class="pi pi-ellipsis-v" aria-hidden="true"></i>
            </button>
            <p-menu #userMenu [model]="userItems()" [popup]="true" appendTo="body" />
          </div>
        </div>
      </aside>

      <!-- Main Content -->
      <main class="flex-1 flex flex-col h-full overflow-hidden relative" role="main" aria-label="Content Area">
        <!-- Mobile Header -->
        <div class="md:hidden flex items-center justify-between p-4 bg-surface-card border-b border-surface-200 dark:border-surface-700">
          <app-brand size="small" />
          <button (click)="isMobileDrawerOpen.set(true)" class="p-2 rounded hover:bg-surface-100 dark:hover:bg-surface-800">
            <i class="pi pi-bars text-xl"></i>
          </button>
        </div>

        <div class="flex-1 overflow-y-auto p-4 md:p-8">
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

          <!-- Mobile Footer (Replicating Desktop Sidebar Footer) -->
          <div class="border-t border-surface-200 dark:border-surface-700 pt-6 pb-4">
            <div class="flex items-center justify-between px-2 mb-4">
              <div class="flex items-center gap-3">
                <p-avatar icon="pi pi-user" shape="circle" styleClass="bg-surface-200 dark:bg-surface-700" />
                <span class="text-sm font-medium">Admin</span>
              </div>
              <button (click)="prefs.toggleTheme()"
                      class="p-2 rounded-full hover:bg-surface-100 dark:hover:bg-surface-800">
                <i class="pi" [class]="prefs.isDarkMode() ? 'pi-moon' : 'pi-sun'"></i>
              </button>
            </div>
            <div class="flex flex-col gap-1">
              <a routerLink="/settings/user" (click)="isMobileDrawerOpen.set(false)"
                 class="flex items-center gap-3 px-3 py-2 rounded-md font-medium hover:bg-surface-100 dark:hover:bg-surface-800">
                <i class="pi pi-user"></i> <span>User Settings</span>
              </a>
              <a routerLink="/settings/preferences" (click)="isMobileDrawerOpen.set(false)"
                 class="flex items-center gap-3 px-3 py-2 rounded-md font-medium hover:bg-surface-100 dark:hover:bg-surface-800">
                <i class="pi pi-cog"></i> <span>Preferences</span>
              </a>
              <button (click)="signOut()"
                      class="flex items-center gap-3 px-3 py-2 rounded-md font-medium hover:bg-surface-100 dark:hover:bg-surface-800 w-full text-left text-red-500">
                <i class="pi pi-sign-out"></i> <span>Sign Out</span>
              </button>
            </div>
          </div>
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
  protected prefs = inject(PreferencesService);
  protected siteService = inject(SiteService);
  protected perms = inject(PermissionService);
  private router = inject(Router);

  // UI State
  protected isMobileDrawerOpen = signal(false);
  protected isAddSiteVisible = signal(false);
  protected isSiteSettingsVisible = signal(false);
  protected siteSettingsTab = signal('0');

  @HostListener('document:keydown', ['$event'])
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

  protected userItems = computed<MenuItem[]>(() => [
    {
      label: 'Administration',
      icon: 'pi pi-shield',

      visible: this.perms.isInstanceAdmin(),
      command: () => this.router.navigate(['/admin'])
    },
    {
      label: 'User Settings',
      icon: 'pi pi-user',
      command: () => this.router.navigate(['/settings/user'])
    },
    {
      label: 'Preferences',
      icon: 'pi pi-cog',
      command: () => this.router.navigate(['/settings/preferences'])
    },
    { separator: true },
    {
      label: 'Sign Out',
      icon: 'pi pi-sign-out',
      command: () => this.signOut()
    }
  ]);

  constructor() {
    this.siteService.loadSites();
    this.perms.loadPermissions().subscribe();
  }

  signOut() {
    document.cookie = 'hk_token=; Max-Age=0; path=/;';
    this.router.navigate(['/login']);
    this.isMobileDrawerOpen.set(false);
  }
}
