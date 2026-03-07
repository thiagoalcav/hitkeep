import { Component, computed, effect, inject, signal } from "@angular/core";
import { Router, RouterLink, RouterLinkActive, RouterOutlet } from "@angular/router";
import { TranslocoPipe, TranslocoService } from "@jsverse/transloco";
import { DrawerModule } from "primeng/drawer";
import { Brand } from "@components/brand/brand";
import { TeamSwitcher } from "@components/team-switcher/team-switcher";
import { UserControls } from "@components/user-controls/user-controls";
import { AddSiteDialog } from "@features/sites/components/add-site-dialog";
import { SiteSettingsDrawer } from "@features/sites/components/site-settings-drawer";
import { SiteSelector } from "@features/sites/components/site-selector";
import { Team } from "@models/analytics.types";
import { PermissionService } from "@services/permission.service";
import { ShareService } from "@services/share.service";
import { SiteSettingsService } from "@services/site-settings.service";
import { TeamService } from "@services/team.service";
import { UserPreferencesService } from "@services/user-preferences.service";
import { UserProfileService } from "@services/user-profile.service";
import { CreateTeamDialog } from "@components/create-team-dialog/create-team-dialog";
import { SiteService } from "@features/sites/services/site.service";

@Component({
    selector: "app-main-layout",
    host: {
        "(document:keydown)": "handleKeyboard($event)"
    },
    imports: [RouterOutlet, RouterLink, RouterLinkActive, Brand, SiteSelector, TeamSwitcher, AddSiteDialog, CreateTeamDialog, SiteSettingsDrawer, UserControls, DrawerModule, TranslocoPipe],
    templateUrl: "./main-layout.html",
    styleUrl: "./main-layout.css"
})
export class MainLayout {
    private readonly router = inject(Router);
    protected readonly siteService = inject(SiteService);
    protected readonly shareService = inject(ShareService);
    private readonly siteSettings = inject(SiteSettingsService);
    protected readonly teamService = inject(TeamService);
    protected readonly perms = inject(PermissionService);
    protected readonly profile = inject(UserProfileService);
    protected readonly preferences = inject(UserPreferencesService);
    private readonly transloco = inject(TranslocoService);

    protected readonly isTeamAdmin = computed(() => {
        const role = this.teamService.activeTeam()?.role;
        return role === "owner" || role === "admin";
    });

    protected readonly isMobileDrawerOpen = signal(false);
    protected readonly isAddSiteVisible = signal(false);
    protected readonly isCreateTeamVisible = signal(false);
    protected readonly isSiteSettingsVisible = signal(false);
    protected readonly siteSettingsTab = signal("0");
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
            if (!tab) {
                return;
            }
            this.openSiteSettings(tab);
            this.siteSettings.clear();
        });
    }
}
