import { ChangeDetectionStrategy, Component, computed, inject, signal } from "@angular/core";
import { toSignal } from "@angular/core/rxjs-interop";
import { TranslocoPipe, TranslocoService } from "@jsverse/transloco";
import { TabsModule } from "primeng/tabs";
import { PageHeader } from "@components/page-header/page-header";
import { PageBreadcrumb, PageBreadcrumbItem } from "@components/page-breadcrumb/page-breadcrumb";
import { TeamOverviewPage } from "./team-overview";
import { TeamMembersPage } from "./team-members";
import { TeamSettingsPage } from "./team-settings";
import { TeamAuditPage } from "./team-audit";

@Component({
    selector: "app-team-admin",
    imports: [TabsModule, PageHeader, PageBreadcrumb, TeamOverviewPage, TeamMembersPage, TeamSettingsPage, TeamAuditPage, TranslocoPipe],
    templateUrl: "./team-admin.html",
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class TeamAdminPage {
    private readonly transloco = inject(TranslocoService);
    private readonly activeLanguage = toSignal(this.transloco.langChanges$, { initialValue: this.transloco.getActiveLang() });

    protected readonly activeTab = signal("0");

    protected readonly breadcrumbItems = computed<PageBreadcrumbItem[]>(() => {
        this.activeLanguage();
        return [{ label: this.transloco.translate("nav.administration") }, { label: this.transloco.translate("nav.team"), isCurrent: true }];
    });
}
