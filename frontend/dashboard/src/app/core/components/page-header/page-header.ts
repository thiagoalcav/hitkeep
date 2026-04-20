import { ChangeDetectionStrategy, Component, DestroyRef, Directive, TemplateRef, contentChild, effect, inject } from '@angular/core';
import { MainLayoutContextService } from '@layout/main-layout-context.service';

@Directive({
    selector: 'ng-template[appPageHeaderLeft]',
    standalone: true
})
export class PageHeaderLeft {}

@Directive({
    selector: 'ng-template[appPageHeaderRight]',
    standalone: true
})
export class PageHeaderRight {}

@Component({
    selector: 'app-page-header',
    standalone: true,
    changeDetection: ChangeDetectionStrategy.OnPush,
    template: ''
})
export class PageHeader {
    private readonly context = inject(MainLayoutContextService, { optional: true });
    private readonly destroyRef = inject(DestroyRef);
    private readonly owner = Symbol('page-header');
    private readonly leftTemplate = contentChild(PageHeaderLeft, { read: TemplateRef });
    private readonly rightTemplate = contentChild(PageHeaderRight, { read: TemplateRef });

    constructor() {
        effect(() => {
            this.context?.registerPageHeader(this.owner, this.leftTemplate() ?? null, this.rightTemplate() ?? null);
        });

        this.destroyRef.onDestroy(() => {
            this.context?.clearPageHeader(this.owner);
        });
    }
}
