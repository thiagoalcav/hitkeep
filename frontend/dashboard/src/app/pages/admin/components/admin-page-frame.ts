import { ChangeDetectionStrategy, Component, input } from '@angular/core';
import { PageBreadcrumbItem } from '@components/page-breadcrumb/page-breadcrumb';
import { PageFrame } from '@components/page-frame/page-frame';

@Component({
    selector: 'app-admin-page-frame',
    imports: [PageFrame],
    templateUrl: './admin-page-frame.html',
    styleUrl: './admin-page-frame.css',
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class AdminPageFrame {
    breadcrumbItems = input.required<PageBreadcrumbItem[]>();
}
