import { ChangeDetectionStrategy, Component, computed, inject } from "@angular/core";
import { TranslocoPipe, TranslocoService } from "@jsverse/transloco";
import { toSignal } from "@angular/core/rxjs-interop";

import { PageHeader } from "@components/page-header/page-header";
import { PageBreadcrumb, PageBreadcrumbItem } from "@components/page-breadcrumb/page-breadcrumb";
import { SettingsAPIClients } from "@features/settings/components/settings-api-clients";

@Component({
    selector: "app-api-clients-page",
    imports: [PageHeader, PageBreadcrumb, SettingsAPIClients, TranslocoPipe],
    templateUrl: "./api-clients.html",
    styleUrl: "./api-clients.css",
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class APIClientsPage {
    private transloco = inject(TranslocoService);
    private activeLanguage = toSignal(this.transloco.langChanges$, { initialValue: this.transloco.getActiveLang() });

    protected readonly breadcrumbItems = computed<PageBreadcrumbItem[]>(() => {
        this.activeLanguage();
        return [
            { label: this.transloco.translate("nav.integration"), routerLink: "/integration/api-clients" },
            { label: this.transloco.translate("nav.apiClients"), isCurrent: true }
        ];
    });
}
