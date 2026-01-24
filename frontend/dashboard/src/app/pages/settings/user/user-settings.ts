import { Component } from '@angular/core';

import { ButtonModule } from 'primeng/button';
import { SettingsSecurity } from '../../../features/settings/components/settings-security';
import { PageHeader } from '../../../core/components/page-header/page-header';
import { PageBreadcrumb, PageBreadcrumbItem } from '../../../core/components/page-breadcrumb/page-breadcrumb';

@Component({
  selector: 'app-user-settings',
  standalone: true,
  imports: [SettingsSecurity, ButtonModule, PageHeader, PageBreadcrumb],
  templateUrl: './user-settings.html'
})
export class UserSettings {
  protected readonly breadcrumbItems: PageBreadcrumbItem[] = [{ label: 'User Settings', isCurrent: true }];

  downloadData() {
    window.open('/api/user/takeout', '_blank');
  }
}
