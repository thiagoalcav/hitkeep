import { Component } from '@angular/core';

import { SplitButtonModule } from 'primeng/splitbutton';
import { MenuItem } from 'primeng/api';
import { SettingsSecurity } from '@features/settings/components/settings-security';
import { PageHeader } from '@components/page-header/page-header';
import { PageBreadcrumb, PageBreadcrumbItem } from '@components/page-breadcrumb/page-breadcrumb';

type ExportFormat = 'csv' | 'xlsx' | 'parquet';

@Component({
    selector: 'app-user-settings',
    standalone: true,
    imports: [SettingsSecurity, SplitButtonModule, PageHeader, PageBreadcrumb],
    templateUrl: './user-settings.html'
})
export class UserSettings {
    protected readonly breadcrumbItems: PageBreadcrumbItem[] = [{ label: 'User Settings', isCurrent: true }];
    protected readonly exportMenuItems: MenuItem[] = [
        { label: 'CSV', icon: 'pi pi-file', command: () => this.downloadData('csv') },
        { label: 'XLSX', icon: 'pi pi-file-excel', command: () => this.downloadData('xlsx') },
        { label: 'Parquet', icon: 'pi pi-database', command: () => this.downloadData('parquet') }
    ];

    downloadData(format: ExportFormat = 'xlsx') {
        window.open(`/api/user/takeout?format=${format}`, '_blank');
    }
}
