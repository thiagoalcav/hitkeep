import { Component, computed, inject, signal, effect } from "@angular/core";
import { Router, RouterOutlet, RouterLink, RouterLinkActive } from "@angular/router";

import { Brand } from "@components/brand/brand";
import { SiteSelector } from "@features/sites/components/site-selector";
import { AddSiteDialog } from "@features/sites/components/add-site-dialog";
import { SiteSettingsDrawer } from "@features/sites/components/site-settings-drawer";
import { SiteService } from "@features/sites/services/site.service";
import { Team } from "@models/analytics.types";
import { PermissionService } from "@services/permission.service";
import { UserProfileService } from "@services/user-profile.service";
import { UserPreferencesService } from "@services/user-preferences.service";
import { TeamService } from "@services/team.service";
import { UserControls } from "@components/user-controls/user-controls";
import { TeamSwitcher } from "@components/team-switcher/team-switcher";
import { CreateTeamDialog } from "@components/create-team-dialog/create-team-dialog";
import { ShareService } from "@services/share.service";
import { SiteSettingsService } from "@services/site-settings.service";
import { TranslocoPipe, TranslocoService } from "@jsverse/transloco";
// PrimeNG
import { DrawerModule } from "primeng/drawer";

@Component({
    selector: "app-main-layout",
    standalone: true,
    host: {
        "(document:keydown)": "handleKeyboard($event)"
    },
    imports: [RouterOutlet, RouterLink, RouterLinkActive, Brand, SiteSelector, TeamSwitcher, AddSiteDialog, CreateTeamDialog, SiteSettingsDrawer, UserControls, DrawerModule, TranslocoPipe],
    template: `
        <div class="flex h-screen w-full bg-[var(--p-surface-ground)]">
            <!-- Sidebar (Desktop) -->
            <aside class="hidden md:flex w-64 flex-col bg-[var(--p-surface-card)] border-r border-surface-200 dark:border-surface-700 p-4 gap-6 overflow-y-auto" [attr.aria-label]="'nav.mainSidebarAria' | transloco">
                <div class="px-1">
                    <app-brand size="small" />
                </div>

                <div class="flex items-center gap-2">
                    <app-site-selector
                        class="flex-1"
                        [sites]="siteService.sites()"
                        [current]="siteService.activeSite()"
                        [loading]="siteService.isLoading()"
                        (siteSelected)="siteService.selectSite($event)"
                        (addClicked)="isAddSiteVisible.set(true)"
                        (settingsClicked)="openSiteSettings('0')"
                        (trackingClicked)="openSiteSettings('1')"
                    />
                </div>

                <nav class="flex-1 flex flex-col gap-1" [attr.aria-label]="'nav.primaryNavigationAria' | transloco">
                    <div class="text-xs font-semibold text-muted-color uppercase px-2 mb-2" role="presentation">{{ "nav.analytics" | transloco }}</div>

                    <a
                        routerLink="/dashboard"
                        routerLinkActive="bg-primary-50 text-primary-700 dark:bg-primary-900/30 dark:text-primary-400"
                        class="flex items-center gap-3 px-3 py-2 rounded-md font-medium transition-colors hover:bg-surface-100 dark:hover:bg-surface-800 cursor-pointer focus:outline-none focus:ring-2 focus:ring-primary-500"
                        [attr.aria-label]="'nav.dashboardAria' | transloco"
                    >
                        <i class="pi pi-chart-bar" aria-hidden="true"></i> <span>{{ "nav.dashboard" | transloco }}</span>
                    </a>
                    <a
                        routerLink="/goals"
                        routerLinkActive="bg-primary-50 text-primary-700 dark:bg-primary-900/30 dark:text-primary-400"
                        class="flex items-center gap-3 px-3 py-2 rounded-md font-medium transition-colors hover:bg-surface-100 dark:hover:bg-surface-800 cursor-pointer focus:outline-none focus:ring-2 focus:ring-primary-500"
                        [attr.aria-label]="'nav.goalsAria' | transloco"
                    >
                        <i class="pi pi-flag" aria-hidden="true"></i> <span>{{ "nav.goals" | transloco }}</span>
                    </a>
                    <a
                        routerLink="/funnels"
                        routerLinkActive="bg-primary-50 text-primary-700 dark:bg-primary-900/30 dark:text-primary-400"
                        class="flex items-center gap-3 px-3 py-2 rounded-md font-medium transition-colors hover:bg-surface-100 dark:hover:bg-surface-800 cursor-pointer focus:outline-none focus:ring-2 focus:ring-primary-500"
                        [attr.aria-label]="'nav.funnelsAria' | transloco"
                    >
                        <i class="pi pi-filter" aria-hidden="true"></i> <span>{{ "nav.funnels" | transloco }}</span>
                    </a>
                    <a
                        routerLink="/events"
                        routerLinkActive="bg-primary-50 text-primary-700 dark:bg-primary-900/30 dark:text-primary-400"
                        class="flex items-center gap-3 px-3 py-2 rounded-md font-medium transition-colors hover:bg-surface-100 dark:hover:bg-surface-800 cursor-pointer focus:outline-none focus:ring-2 focus:ring-primary-500"
                        [attr.aria-label]="'nav.eventsAria' | transloco"
                    >
                        <i class="pi pi-bolt" aria-hidden="true"></i> <span>{{ "nav.events" | transloco }}</span>
                    </a>
                    <a
                        routerLink="/utm"
                        routerLinkActive="bg-primary-50 text-primary-700 dark:bg-primary-900/30 dark:text-primary-400"
                        [routerLinkActiveOptions]="{ exact: true }"
                        class="flex items-center gap-3 px-3 py-2 rounded-md font-medium transition-colors hover:bg-surface-100 dark:hover:bg-surface-800 cursor-pointer focus:outline-none focus:ring-2 focus:ring-primary-500"
                        [attr.aria-label]="'nav.utmAria' | transloco"
                    >
                        <i class="pi pi-tags" aria-hidden="true"></i> <span>{{ "nav.utm" | transloco }}</span>
                    </a>
                    <a
                        routerLink="/utm/builder"
                        routerLinkActive="bg-primary-50 text-primary-700 dark:bg-primary-900/30 dark:text-primary-400"
                        class="flex items-center gap-3 pl-8 pr-3 py-2 rounded-md text-sm font-medium transition-colors hover:bg-surface-100 dark:hover:bg-surface-800 cursor-pointer focus:outline-none focus:ring-2 focus:ring-primary-500"
                        [attr.aria-label]="'nav.utmBuilderAria' | transloco"
                    >
                        <i class="pi pi-link" aria-hidden="true"></i> <span>{{ "nav.utmBuilder" | transloco }}</span>
                    </a>

                    <div class="text-xs font-semibold text-muted-color uppercase px-2 mt-4 mb-2" role="presentation">{{ "nav.integration" | transloco }}</div>
                    <a
                        routerLink="/integration/api-clients"
                        routerLinkActive="bg-primary-50 text-primary-700 dark:bg-primary-900/30 dark:text-primary-400"
                        class="flex items-center gap-3 px-3 py-2 rounded-md font-medium transition-colors hover:bg-surface-100 dark:hover:bg-surface-800 cursor-pointer focus:outline-none focus:ring-2 focus:ring-primary-500"
                        [attr.aria-label]="'nav.apiClientsAria' | transloco"
                    >
                        <i class="pi pi-key" aria-hidden="true"></i> <span>{{ "nav.apiClients" | transloco }}</span>
                    </a>
                    <a
                        routerLink="/integration/api-reference"
                        routerLinkActive="bg-primary-50 text-primary-700 dark:bg-primary-900/30 dark:text-primary-400"
                        class="flex items-center gap-3 px-3 py-2 rounded-md font-medium transition-colors hover:bg-surface-100 dark:hover:bg-surface-800 cursor-pointer focus:outline-none focus:ring-2 focus:ring-primary-500"
                        [attr.aria-label]="'nav.apiReferenceAria' | transloco"
                    >
                        <i class="pi pi-book" aria-hidden="true"></i> <span>{{ "nav.apiReference" | transloco }}</span>
                    </a>

                    <div class="text-xs font-semibold text-muted-color uppercase px-2 mt-4 mb-2" role="presentation">{{ "nav.account" | transloco }}</div>
                    <a
                        routerLink="/settings/reports"
                        routerLinkActive="bg-primary-50 text-primary-700 dark:bg-primary-900/30 dark:text-primary-400"
                        class="flex items-center gap-3 px-3 py-2 rounded-md font-medium transition-colors hover:bg-surface-100 dark:hover:bg-surface-800 cursor-pointer focus:outline-none focus:ring-2 focus:ring-primary-500"
                        [attr.aria-label]="'nav.emailReportsAria' | transloco"
                    >
                        <i class="pi pi-envelope" aria-hidden="true"></i> <span>{{ "nav.emailReports" | transloco }}</span>
                    </a>

                    @if (isTeamAdmin()) {
                        <div class="text-xs font-semibold text-muted-color uppercase px-2 mt-4 mb-2" role="presentation">{{ "nav.administration" | transloco }}</div>
                        @if (perms.isInstanceAdmin()) {
                            <a
                                routerLink="/admin/system"
                                routerLinkActive="bg-primary-50 text-primary-700 dark:bg-primary-900/30 dark:text-primary-400"
                                class="flex items-center gap-3 px-3 py-2 rounded-md font-medium transition-colors hover:bg-surface-100 dark:hover:bg-surface-800 cursor-pointer focus:outline-none focus:ring-2 focus:ring-primary-500"
                                [attr.aria-label]="'nav.systemSettingsAria' | transloco"
                            >
                                <i class="pi pi-shield" aria-hidden="true"></i> <span>{{ "nav.systemSettings" | transloco }}</span>
                            </a>
                        }
                        <a
                            routerLink="/admin/team"
                            routerLinkActive="bg-primary-50 text-primary-700 dark:bg-primary-900/30 dark:text-primary-400"
                            class="flex items-center gap-3 px-3 py-2 rounded-md font-medium transition-colors hover:bg-surface-100 dark:hover:bg-surface-800 cursor-pointer focus:outline-none focus:ring-2 focus:ring-primary-500"
                            [attr.aria-label]="'nav.teamAria' | transloco"
                        >
                            <i class="pi pi-users" aria-hidden="true"></i> <span>{{ "nav.team" | transloco }}</span>
                        </a>
                    }
                </nav>
            </aside>

            <!-- Main Content -->
            <main class="flex-1 flex flex-col h-full overflow-hidden relative" role="main" [attr.aria-label]="'nav.contentAreaAria' | transloco">
                <!-- Desktop Header -->
                <div class="hidden md:flex items-center justify-between gap-4 px-6 py-3 bg-surface-card border-b border-surface-200 dark:border-surface-700">
                    <div class="w-[24rem] max-w-full">
                        <app-team-switcher
                            [teams]="teamService.teams()"
                            [currentTeamId]="teamService.activeTeamId()"
                            [loading]="teamService.isLoading()"
                            [switching]="teamService.isSwitching()"
                            [showBrand]="false"
                            [showAdd]="true"
                            [compact]="true"
                            [beforeSwitch]="beforeTeamSwitch"
                            (teamSelected)="onTeamSelected($event)"
                            (addClicked)="isCreateTeamVisible.set(true)"
                        />
                    </div>
                    <app-user-controls [showMenu]="!shareService.isShareMode()" />
                </div>

                <!-- Mobile Header -->
                <div class="md:hidden flex flex-col gap-3 p-4 bg-surface-card border-b border-surface-200 dark:border-surface-700">
                    <div class="flex items-center justify-between">
                        <app-brand size="small" />
                        <div class="flex items-center gap-2">
                            <app-user-controls [showMenu]="!shareService.isShareMode()" />
                            <button (click)="isMobileDrawerOpen.set(true)" class="p-2 rounded hover:bg-surface-100 dark:hover:bg-surface-800">
                                <i class="pi pi-bars text-xl"></i>
                            </button>
                        </div>
                    </div>
                    <app-team-switcher
                        [teams]="teamService.teams()"
                        [currentTeamId]="teamService.activeTeamId()"
                        [loading]="teamService.isLoading()"
                        [switching]="teamService.isSwitching()"
                        [showBrand]="false"
                        [showAdd]="true"
                        [compact]="true"
                        [beforeSwitch]="beforeTeamSwitch"
                        (teamSelected)="onTeamSelected($event)"
                        (addClicked)="isCreateTeamVisible.set(true)"
                    />
                </div>

                <div class="flex-1 overflow-y-auto p-4 md:p-8 md:pt-4 relative">
                    <router-outlet />
                </div>
            </main>

            <!-- Mobile Drawer -->
            <p-drawer [visible]="isMobileDrawerOpen()" (visibleChange)="isMobileDrawerOpen.set($event)">
                <div class="flex flex-col gap-6 h-full">
                    <app-team-switcher
                        [teams]="teamService.teams()"
                        [currentTeamId]="teamService.activeTeamId()"
                        [loading]="teamService.isLoading()"
                        [switching]="teamService.isSwitching()"
                        [showAdd]="true"
                        [compact]="true"
                        [beforeSwitch]="beforeTeamSwitch"
                        (teamSelected)="onTeamSelected($event)"
                        (addClicked)="isCreateTeamVisible.set(true)"
                    />

                    <app-site-selector
                        [sites]="siteService.sites()"
                        [current]="siteService.activeSite()"
                        [loading]="siteService.isLoading()"
                        (siteSelected)="siteService.selectSite($event); isMobileDrawerOpen.set(false)"
                        (addClicked)="isAddSiteVisible.set(true)"
                        (settingsClicked)="openSiteSettings('0'); isMobileDrawerOpen.set(false)"
                        (trackingClicked)="openSiteSettings('1'); isMobileDrawerOpen.set(false)"
                    />

                    <nav class="flex-1">
                        <a routerLink="/dashboard" (click)="isMobileDrawerOpen.set(false)" routerLinkActive="bg-primary-50 text-primary-700" class="flex items-center gap-3 px-3 py-2 rounded-md font-medium" [attr.aria-label]="'nav.dashboardAria' | transloco">
                            <i class="pi pi-chart-bar"></i> <span>{{ "nav.dashboard" | transloco }}</span>
                        </a>
                        <a routerLink="/goals" (click)="isMobileDrawerOpen.set(false)" routerLinkActive="bg-primary-50 text-primary-700" class="flex items-center gap-3 px-3 py-2 rounded-md font-medium" [attr.aria-label]="'nav.goalsAria' | transloco">
                            <i class="pi pi-flag"></i> <span>{{ "nav.goals" | transloco }}</span>
                        </a>
                        <a routerLink="/funnels" (click)="isMobileDrawerOpen.set(false)" routerLinkActive="bg-primary-50 text-primary-700" class="flex items-center gap-3 px-3 py-2 rounded-md font-medium" [attr.aria-label]="'nav.funnelsAria' | transloco">
                            <i class="pi pi-filter"></i> <span>{{ "nav.funnels" | transloco }}</span>
                        </a>
                        <a routerLink="/events" (click)="isMobileDrawerOpen.set(false)" routerLinkActive="bg-primary-50 text-primary-700" class="flex items-center gap-3 px-3 py-2 rounded-md font-medium" [attr.aria-label]="'nav.eventsAria' | transloco">
                            <i class="pi pi-bolt"></i> <span>{{ "nav.events" | transloco }}</span>
                        </a>
                        <a
                            routerLink="/utm"
                            [routerLinkActiveOptions]="{ exact: true }"
                            (click)="isMobileDrawerOpen.set(false)"
                            routerLinkActive="bg-primary-50 text-primary-700"
                            class="flex items-center gap-3 px-3 py-2 rounded-md font-medium"
                            [attr.aria-label]="'nav.utmAria' | transloco"
                        >
                            <i class="pi pi-tags"></i> <span>{{ "nav.utm" | transloco }}</span>
                        </a>
                        <a
                            routerLink="/utm/builder"
                            (click)="isMobileDrawerOpen.set(false)"
                            routerLinkActive="bg-primary-50 text-primary-700"
                            class="flex items-center gap-3 pl-8 pr-3 py-2 rounded-md text-sm font-medium"
                            [attr.aria-label]="'nav.utmBuilderAria' | transloco"
                        >
                            <i class="pi pi-link"></i> <span>{{ "nav.utmBuilder" | transloco }}</span>
                        </a>
                        <div class="text-xs font-semibold text-muted-color uppercase px-3 mt-4 mb-2" role="presentation">{{ "nav.integration" | transloco }}</div>
                        <a
                            routerLink="/integration/api-clients"
                            (click)="isMobileDrawerOpen.set(false)"
                            routerLinkActive="bg-primary-50 text-primary-700"
                            class="flex items-center gap-3 px-3 py-2 rounded-md font-medium"
                            [attr.aria-label]="'nav.apiClientsAria' | transloco"
                        >
                            <i class="pi pi-key"></i> <span>{{ "nav.apiClients" | transloco }}</span>
                        </a>
                        <a
                            routerLink="/integration/api-reference"
                            (click)="isMobileDrawerOpen.set(false)"
                            routerLinkActive="bg-primary-50 text-primary-700"
                            class="flex items-center gap-3 px-3 py-2 rounded-md font-medium"
                            [attr.aria-label]="'nav.apiReferenceAria' | transloco"
                        >
                            <i class="pi pi-book"></i> <span>{{ "nav.apiReference" | transloco }}</span>
                        </a>
                        <div class="text-xs font-semibold text-muted-color uppercase px-3 mt-4 mb-2" role="presentation">{{ "nav.account" | transloco }}</div>
                        <a
                            routerLink="/settings/reports"
                            (click)="isMobileDrawerOpen.set(false)"
                            routerLinkActive="bg-primary-50 text-primary-700"
                            class="flex items-center gap-3 px-3 py-2 rounded-md font-medium"
                            [attr.aria-label]="'nav.emailReportsAria' | transloco"
                        >
                            <i class="pi pi-envelope"></i> <span>{{ "nav.emailReports" | transloco }}</span>
                        </a>

                        @if (isTeamAdmin()) {
                            <div class="text-xs font-semibold text-muted-color uppercase px-3 mt-4 mb-2" role="presentation">{{ "nav.administration" | transloco }}</div>
                            @if (perms.isInstanceAdmin()) {
                                <a
                                    routerLink="/admin/system"
                                    (click)="isMobileDrawerOpen.set(false)"
                                    routerLinkActive="bg-primary-50 text-primary-700"
                                    class="flex items-center gap-3 px-3 py-2 rounded-md font-medium"
                                    [attr.aria-label]="'nav.systemSettingsAria' | transloco"
                                >
                                    <i class="pi pi-shield"></i> <span>{{ "nav.systemSettings" | transloco }}</span>
                                </a>
                            }
                            <a routerLink="/admin/team" (click)="isMobileDrawerOpen.set(false)" routerLinkActive="bg-primary-50 text-primary-700" class="flex items-center gap-3 px-3 py-2 rounded-md font-medium" [attr.aria-label]="'nav.teamAria' | transloco">
                                <i class="pi pi-users"></i> <span>{{ "nav.team" | transloco }}</span>
                            </a>
                        }
                    </nav>
                </div>
            </p-drawer>

            <!-- Add Site Dialog -->
            @defer (when isAddSiteVisible()) {
                <app-add-site-dialog [visible]="isAddSiteVisible()" (visibleChange)="isAddSiteVisible.set($event)" />
            }

            <!-- Site Settings Drawer -->
            @defer (when isSiteSettingsVisible()) {
                <app-site-settings-drawer [visible]="isSiteSettingsVisible()" (visibleChange)="isSiteSettingsVisible.set($event)" [activeTab]="siteSettingsTab()" (activeTabChange)="siteSettingsTab.set($event)" [site]="siteService.activeSite()" />
            }

            <!-- Create Team Dialog -->
            @defer (when isCreateTeamVisible()) {
                <app-create-team-dialog [(visible)]="isCreateTeamVisible" />
            }
        </div>
    `
})
export class MainLayout {
    private router = inject(Router);
    protected siteService = inject(SiteService);
    protected shareService = inject(ShareService);
    private siteSettings = inject(SiteSettingsService);
    protected teamService = inject(TeamService);
    protected perms = inject(PermissionService);
    protected profile = inject(UserProfileService);
    protected preferences = inject(UserPreferencesService);
    private transloco = inject(TranslocoService);

    protected readonly isTeamAdmin = computed(() => {
        const role = this.teamService.activeTeam()?.role;
        return role === "owner" || role === "admin";
    });

    // UI State
    protected isMobileDrawerOpen = signal(false);
    protected isAddSiteVisible = signal(false);
    protected isCreateTeamVisible = signal(false);
    protected isSiteSettingsVisible = signal(false);
    protected siteSettingsTab = signal("0");
    protected readonly beforeTeamSwitch = () => {
        if (!this.isSiteSettingsVisible()) {
            return true;
        }
        const proceed = window.confirm(this.transloco.translate("sites.settings.unsavedChangesConfirm"));
        if (!proceed) {
            return false;
        }
        this.isSiteSettingsVisible.set(false);
        return true;
    };

    handleKeyboard(event: KeyboardEvent) {
        // Cmd/Ctrl + K opens site settings
        if ((event.metaKey || event.ctrlKey) && event.key === "k") {
            event.preventDefault();
            this.openSiteSettings();
        }
    }

    openSiteSettings(tab = "0") {
        if (this.siteService.activeSite()) {
            this.siteSettingsTab.set(tab);
            this.isSiteSettingsVisible.set(true);
        }
    }

    protected onTeamSelected(team: Team) {
        this.teamService.setActiveTeam(team.id).subscribe({
            next: () => {
                this.siteService.sites.set([]);
                this.siteService.activeSite.set(null);
                this.teamService.loadTeams().subscribe({
                    next: () => {
                        this.siteService.loadSites();
                        this.perms.loadPermissions().subscribe({
                            next: () => this.redirectIfTeamAdminAccessWasLost(),
                            error: () => this.redirectIfTeamAdminAccessWasLost()
                        });
                    },
                    error: () => {
                        this.siteService.loadSites();
                        this.perms.loadPermissions().subscribe({
                            next: () => this.redirectIfTeamAdminAccessWasLost(),
                            error: () => this.redirectIfTeamAdminAccessWasLost()
                        });
                    }
                });
            },
            error: () => undefined
        });
    }

    private redirectIfTeamAdminAccessWasLost() {
        if (this.router.routerState.snapshot.url.startsWith("/admin/team") && !this.isTeamAdmin()) {
            this.router.navigateByUrl("/dashboard");
        }
    }

    constructor() {
        const currentUrl = this.router.routerState.snapshot.url;
        if (currentUrl.startsWith("/share") || this.shareService.isShareMode()) {
            return;
        }
        this.teamService.loadTeams().subscribe({ error: () => undefined });
        this.siteService.loadSites();
        this.perms.loadPermissions().subscribe({ error: () => undefined });
        this.profile.loadProfile().subscribe({ error: () => undefined });
        this.preferences.load().subscribe({ error: () => undefined });

        effect(() => {
            const tab = this.siteSettings.request();
            if (!tab) return;
            this.openSiteSettings(tab);
            this.siteSettings.clear();
        });
    }
}
