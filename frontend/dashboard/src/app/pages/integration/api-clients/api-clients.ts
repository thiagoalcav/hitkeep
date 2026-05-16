import { ChangeDetectionStrategy, Component, computed, inject } from '@angular/core';
import { TranslocoPipe, TranslocoService } from '@jsverse/transloco';
import { toSignal } from '@angular/core/rxjs-interop';

import { TEAM_CAPABILITIES } from '@core/access/capabilities';
import { PageHeader, PageHeaderLeft } from '@components/page-header/page-header';
import { PageBreadcrumb, PageBreadcrumbItem } from '@components/page-breadcrumb/page-breadcrumb';
import { SettingsAPIClients } from '@features/settings/components/settings-api-clients';
import { AccessService } from '@services/access.service';
import { TeamService } from '@services/team.service';

@Component({
    selector: 'app-api-clients-page',
    imports: [PageHeader, PageHeaderLeft, PageBreadcrumb, SettingsAPIClients, TranslocoPipe],
    templateUrl: './api-clients.html',
    styleUrl: './api-clients.css',
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class APIClientsPage {
    private transloco = inject(TranslocoService);
    private activeLanguage = toSignal(this.transloco.langChanges$, { initialValue: this.transloco.getActiveLang() });
    private readonly access = inject(AccessService);
    private readonly teams = inject(TeamService);

    protected readonly breadcrumbItems = computed<PageBreadcrumbItem[]>(() => {
        this.activeLanguage();
        return [
            { label: this.transloco.translate('nav.integration'), routerLink: '/integration/api-clients' },
            { label: this.transloco.translate('nav.apiClients'), isCurrent: true }
        ];
    });

    protected readonly canManageTeamAPIClients = computed(() => !!this.teams.activeTeam() && this.access.canActiveTeam(TEAM_CAPABILITIES.manageApiClients));
}
