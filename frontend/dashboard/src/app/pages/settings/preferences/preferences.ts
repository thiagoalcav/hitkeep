import { Component } from '@angular/core';
import { PageHeader } from '@components/page-header/page-header';
import { PageBreadcrumb, PageBreadcrumbItem } from '@components/page-breadcrumb/page-breadcrumb';

@Component({
    selector: 'app-preferences',
    standalone: true,
    imports: [PageHeader, PageBreadcrumb],
    templateUrl: './preferences.html'
})
export class Preferences {
    protected readonly breadcrumbItems: PageBreadcrumbItem[] = [{ label: 'Preferences', isCurrent: true }];
}
