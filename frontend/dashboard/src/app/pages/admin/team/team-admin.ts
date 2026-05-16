import { ChangeDetectionStrategy, Component, computed, effect, inject, signal } from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { ActivatedRoute, Router } from '@angular/router';
import { TranslocoService } from '@jsverse/transloco';
import { TabsModule } from 'primeng/tabs';
import { PageBreadcrumbItem } from '@components/page-breadcrumb/page-breadcrumb';
import { TEAM_CAPABILITIES } from '@core/access/capabilities';
import { AccessService } from '@services/access.service';
import { TeamService } from '@services/team.service';
import { AdminPageFrame } from '../components/admin-page-frame';
import { TeamOverviewPage } from './team-overview';
import { TeamMembersPage } from './team-members';
import { TeamSettingsPage } from './team-settings';
import { TeamAuditPage } from './team-audit';

interface TeamAdminTab {
    label: string;
    value: string;
    visible: boolean;
}

@Component({
    selector: 'app-team-admin',
    imports: [TabsModule, AdminPageFrame, TeamOverviewPage, TeamMembersPage, TeamSettingsPage, TeamAuditPage],
    templateUrl: './team-admin.html',
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class TeamAdminPage {
    private readonly transloco = inject(TranslocoService);
    private readonly teamService = inject(TeamService);
    private readonly access = inject(AccessService);
    private readonly route = inject(ActivatedRoute);
    private readonly router = inject(Router);
    private readonly activeLanguage = toSignal(this.transloco.langChanges$, { initialValue: this.transloco.getActiveLang() });
    private readonly queryParamMap = toSignal(this.route.queryParamMap, { initialValue: this.route.snapshot.queryParamMap });

    protected readonly activeTab = signal('0');
    protected readonly activeTeam = this.teamService.activeTeam;
    protected readonly canViewAudit = computed(() => this.access.canActiveTeam(TEAM_CAPABILITIES.viewAudit));
    protected readonly tabs = computed<TeamAdminTab[]>(() => {
        this.activeLanguage();
        return [
            { label: this.transloco.translate('admin.team.tabs.overview'), value: '0', visible: true },
            { label: this.transloco.translate('admin.team.tabs.members'), value: '1', visible: true },
            { label: this.transloco.translate('admin.team.tabs.settings'), value: '2', visible: true },
            { label: this.transloco.translate('admin.team.tabs.activity'), value: '3', visible: this.canViewAudit() }
        ].filter((tab) => tab.visible);
    });

    protected readonly breadcrumbItems = computed<PageBreadcrumbItem[]>(() => {
        this.activeLanguage();
        return [{ label: this.transloco.translate('nav.administration') }, { label: this.transloco.translate('nav.team'), isCurrent: true }];
    });

    constructor() {
        effect(() => {
            const requestedTab = this.tabValueFromQuery(this.queryParamMap().get('tab'));
            const visibleTabs = this.tabs();
            if (requestedTab && visibleTabs.some((tab) => tab.value === requestedTab) && this.activeTab() !== requestedTab) {
                this.activeTab.set(requestedTab);
            }
        });

        effect(() => {
            const activeTab = this.activeTab();
            const visibleTabs = this.tabs();
            if (!visibleTabs.some((tab) => tab.value === activeTab)) {
                this.activeTab.set(visibleTabs[0]?.value ?? '0');
            }
        });
    }

    protected onTabChange(value: string | number | undefined): void {
        if (value === undefined) {
            return;
        }
        const tabValue = String(value);
        this.activeTab.set(tabValue);
        const tab = this.tabQueryFromValue(tabValue);
        void this.router.navigate([], {
            relativeTo: this.route,
            queryParams: tab ? { tab, section: null } : { tab: null, section: null },
            queryParamsHandling: 'merge',
            replaceUrl: true
        });
    }

    private tabValueFromQuery(tab: string | null): string | null {
        switch ((tab ?? '').trim().toLowerCase()) {
            case 'overview':
            case '0':
                return '0';
            case 'members':
            case '1':
                return '1';
            case 'settings':
            case '2':
                return '2';
            case 'activity':
            case 'audit':
            case '3':
                return '3';
            default:
                return null;
        }
    }

    private tabQueryFromValue(value: string): string | null {
        switch (value) {
            case '0':
                return 'overview';
            case '1':
                return 'members';
            case '2':
                return 'settings';
            case '3':
                return 'activity';
            default:
                return null;
        }
    }
}
