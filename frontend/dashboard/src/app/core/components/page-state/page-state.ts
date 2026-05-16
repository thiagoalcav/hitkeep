import { ChangeDetectionStrategy, Component, input } from '@angular/core';
import { TranslocoPipe } from '@jsverse/transloco';

@Component({
    selector: 'app-page-state',
    imports: [TranslocoPipe],
    template: `
        <section class="page-state" [attr.aria-labelledby]="titleId()">
            <span class="page-state__icon"><i [class]="icon()" aria-hidden="true"></i></span>
            <h2 [id]="titleId()">{{ titleKey() | transloco }}</h2>
            <p>{{ messageKey() | transloco }}</p>
        </section>
    `,
    styleUrl: './page-state.css',
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class PageState {
    private static nextID = 0;
    private readonly defaultTitleID = `page-state-${PageState.nextID++}`;

    titleKey = input.required<string>();
    messageKey = input.required<string>();
    icon = input('pi pi-info-circle');
    titleId = input(this.defaultTitleID);
}
