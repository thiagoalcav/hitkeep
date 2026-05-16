import { ChangeDetectionStrategy, Component, computed, inject } from '@angular/core';
import { TranslocoPipe } from '@jsverse/transloco';
import { Brand } from '@components/brand/brand';
import { TeamSwitcher } from '@components/team-switcher/team-switcher';
import { UserControls } from '@components/user-controls/user-controls';
import { MainLayoutContextService } from '@layout/main-layout-context.service';

@Component({
    selector: 'app-layout-mobile-header',
    imports: [Brand, TeamSwitcher, UserControls, TranslocoPipe],
    templateUrl: './layout-mobile-header.html',
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class LayoutMobileHeader {
    protected readonly context = inject(MainLayoutContextService);
    protected readonly shareService = this.context.shareService;
    protected readonly teamService = this.context.teamService;
    protected readonly isMobileDrawerOpen = this.context.isMobileDrawerOpen;
    protected readonly isCreateTeamVisible = this.context.isCreateTeamVisible;
    protected readonly beforeTeamSwitch = this.context.beforeTeamSwitch;
    protected readonly canCreateTeams = computed(() => !this.context.cloudHosted());
}
