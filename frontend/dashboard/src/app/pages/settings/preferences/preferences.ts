import { Component } from '@angular/core';
import { PageHeader } from '../../../core/components/page-header/page-header';
import { PageBreadcrumb, PageBreadcrumbItem } from '../../../core/components/page-breadcrumb/page-breadcrumb';


@Component({
  selector: 'app-preferences',
  standalone: true,
  imports: [PageHeader, PageBreadcrumb],
  templateUrl: './preferences.html'
})
export class Preferences {
  protected readonly breadcrumbItems: PageBreadcrumbItem[] = [{ label: 'Preferences', isCurrent: true }];
}
