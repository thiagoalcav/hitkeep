import { ChangeDetectionStrategy, Component, computed, inject } from '@angular/core';
import { NgTemplateOutlet } from '@angular/common';
import { TeamSwitcher } from '@components/team-switcher/team-switcher';
import { UserControls } from '@components/user-controls/user-controls';
import { MainLayoutContextService } from '@layout/main-layout-context.service';

@Component({
    selector: 'app-layout-page-bar',
    imports: [NgTemplateOutlet, TeamSwitcher, UserControls],
    templateUrl: './layout-page-bar.html',
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class LayoutPageBar {
    protected readonly context = inject(MainLayoutContextService);
    protected readonly shareService = this.context.shareService;
    protected readonly teamService = this.context.teamService;
    protected readonly isCreateTeamVisible = this.context.isCreateTeamVisible;
    protected readonly beforeTeamSwitch = this.context.beforeTeamSwitch;
    protected readonly canCreateTeams = computed(() => !this.context.cloudHosted());
}
