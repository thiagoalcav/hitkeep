import { ChangeDetectionStrategy, Component, computed, effect, inject, signal } from "@angular/core";
import { toSignal } from "@angular/core/rxjs-interop";
import { TranslocoService } from "@jsverse/transloco";
import { TabsModule } from "primeng/tabs";
import { PageHeader, PageHeaderLeft } from "@components/page-header/page-header";
import { PageBreadcrumb, PageBreadcrumbItem } from "@components/page-breadcrumb/page-breadcrumb";
import { TeamService } from "@services/team.service";
import { TeamOverviewPage } from "./team-overview";
import { TeamMembersPage } from "./team-members";
import { TeamSettingsPage } from "./team-settings";
import { TeamAuditPage } from "./team-audit";

interface TeamAdminTab {
    label: string;
    value: string;
    visible: boolean;
}

@Component({
    selector: "app-team-admin",
    imports: [TabsModule, PageHeader, PageHeaderLeft, PageBreadcrumb, TeamOverviewPage, TeamMembersPage, TeamSettingsPage, TeamAuditPage],
    templateUrl: "./team-admin.html",
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class TeamAdminPage {
    private readonly transloco = inject(TranslocoService);
    private readonly teamService = inject(TeamService);
    private readonly activeLanguage = toSignal(this.transloco.langChanges$, { initialValue: this.transloco.getActiveLang() });

    protected readonly activeTab = signal("0");
    protected readonly activeTeam = this.teamService.activeTeam;
    protected readonly canViewAudit = computed(() => {
        const role = this.activeTeam()?.role;
        return role === "owner" || role === "admin";
    });
    protected readonly tabs = computed<TeamAdminTab[]>(() => {
        this.activeLanguage();
        return [
            { label: this.transloco.translate("admin.team.tabs.overview"), value: "0", visible: true },
            { label: this.transloco.translate("admin.team.tabs.members"), value: "1", visible: true },
            { label: this.transloco.translate("admin.team.tabs.settings"), value: "2", visible: true },
            { label: this.transloco.translate("admin.team.tabs.activity"), value: "3", visible: this.canViewAudit() }
        ].filter((tab) => tab.visible);
    });

    protected readonly breadcrumbItems = computed<PageBreadcrumbItem[]>(() => {
        this.activeLanguage();
        return [{ label: this.transloco.translate("nav.administration") }, { label: this.transloco.translate("nav.team"), isCurrent: true }];
    });

    constructor() {
        effect(() => {
            const activeTab = this.activeTab();
            const visibleTabs = this.tabs();
            if (!visibleTabs.some((tab) => tab.value === activeTab)) {
                this.activeTab.set(visibleTabs[0]?.value ?? "0");
            }
        });
    }
}
