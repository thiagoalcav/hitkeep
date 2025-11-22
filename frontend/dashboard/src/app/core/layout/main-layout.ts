import { Component, inject, signal } from '@angular/core';
import { RouterOutlet, RouterLink, RouterLinkActive, Router } from '@angular/router';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { Brand } from '../components/brand/brand';
import { SiteSelector } from '../../features/sites/components/site-selector';
import { AddSiteDialog } from '../../features/sites/components/add-site-dialog';
import { PreferencesService } from '../services/preferences.service';
import { SiteService } from '../../features/sites/services/site.service';
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
    CommonModule, FormsModule, RouterOutlet, RouterLink, RouterLinkActive,
    Brand, SiteSelector, AddSiteDialog,
    AvatarModule, ButtonModule, MenuModule, DrawerModule
  ],
  template: `
<div class="flex h-screen w-full bg-[var(--p-surface-ground)]">
<!-- Sidebar (Desktop) -->
<aside class="hidden md:flex w-64 flex-col bg-[var(--p-surface-card)] border-r border-surface-200 dark:border-surface-700 p-4 gap-6" aria-label="Main Sidebar">
<app-brand size="small" class="px-2" />

<!-- Site Selector -->
    <app-site-selector
        [sites]="siteService.sites()"
        [current]="siteService.activeSite()"
        [loading]="siteService.isLoading()"
        (siteSelected)="siteService.selectSite($event)"
        (addClicked)="isAddSiteVisible.set(true)" />

    <!-- TODO: Refactor to Menu Service -->
    <nav class="flex-1 flex flex-col gap-1" aria-label="Primary Navigation">
      <div class="text-xs font-semibold text-muted-color uppercase px-2 mb-2" role="presentation">Analytics</div>

      <a routerLink="/dashboard"
         routerLinkActive="bg-primary-50 text-primary-700 dark:bg-primary-900/30 dark:text-primary-400"
         class="flex items-center gap-3 px-3 py-2 rounded-md font-medium transition-colors hover:bg-surface-100 dark:hover:bg-surface-800 cursor-pointer focus:outline-none focus:ring-2 focus:ring-primary-500"
         aria-label="Go to Dashboard">
         <i class="pi pi-chart-bar" aria-hidden="true"></i> <span>Dashboard</span>
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
         <p-menu #userMenu [model]="userItems" [popup]="true" appendTo="body" />
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
         </nav>
    </div>
  </p-drawer>

  <!-- Add Site Dialog (Refactored) -->
  <app-add-site-dialog [(visible)]="isAddSiteVisible" />
</div>
`
})
export class MainLayout {
  protected prefs = inject(PreferencesService);
  protected siteService = inject(SiteService);
  private router = inject(Router);
// UI State
  protected isMobileDrawerOpen = signal(false);
  protected isAddSiteVisible = signal(false);
  protected userItems: MenuItem[] = [
    {
      label: 'Sign Out',
      icon: 'pi pi-sign-out',
      command: () => {
        document.cookie = 'hk_token=; Max-Age=0; path=/;';
        this.router.navigate(['/login']);
      }
    }
  ];
  constructor() {
    this.siteService.loadSites();
  }
}
