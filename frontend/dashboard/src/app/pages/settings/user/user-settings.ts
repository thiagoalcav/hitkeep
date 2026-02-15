import { Component, computed, inject } from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { TranslocoPipe, TranslocoService } from '@jsverse/transloco';

import { SplitButtonModule } from 'primeng/splitbutton';
import { MenuItem } from 'primeng/api';
import { SettingsSecurity } from '@features/settings/components/settings-security';
import { PageHeader } from '@components/page-header/page-header';
import { PageBreadcrumb, PageBreadcrumbItem } from '@components/page-breadcrumb/page-breadcrumb';

type ExportFormat = 'csv' | 'xlsx' | 'parquet';

@Component({
    selector: 'app-user-settings',
    standalone: true,
    imports: [SettingsSecurity, SplitButtonModule, PageHeader, PageBreadcrumb, TranslocoPipe],
    templateUrl: './user-settings.html'
})
export class UserSettings {
    private transloco = inject(TranslocoService);
    private activeLanguage = toSignal(this.transloco.langChanges$, { initialValue: this.transloco.getActiveLang() });
    protected readonly breadcrumbItems = computed<PageBreadcrumbItem[]>(() => {
        this.activeLanguage();
        return [{ label: this.transloco.translate('settings.user.breadcrumb'), isCurrent: true }];
    });
    protected readonly exportMenuItems = computed<MenuItem[]>(() => {
        this.activeLanguage();
        return [
            { label: this.transloco.translate('common.exportFormats.csv'), icon: 'pi pi-file', command: () => this.downloadData('csv') },
            { label: this.transloco.translate('common.exportFormats.xlsx'), icon: 'pi pi-file-excel', command: () => this.downloadData('xlsx') },
            { label: this.transloco.translate('common.exportFormats.parquet'), icon: 'pi pi-database', command: () => this.downloadData('parquet') }
        ];
    });

    downloadData(format: ExportFormat = 'xlsx') {
        window.open(`/api/user/takeout?format=${format}`, '_blank');
    }
}
