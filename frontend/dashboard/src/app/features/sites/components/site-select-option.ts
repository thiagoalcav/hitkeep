import { ChangeDetectionStrategy, Component, input } from '@angular/core';

import { SiteFavicon, SiteFaviconSource } from '@features/sites/components/site-favicon';

@Component({
    selector: 'app-site-select-option',
    imports: [SiteFavicon],
    changeDetection: ChangeDetectionStrategy.OnPush,
    template: `
        <div class="site-select-option" [class.site-select-option--selected]="selected()">
            <app-site-favicon [site]="site()" />
            <span class="site-select-option__domain" [title]="site()?.domain">{{ site()?.domain }}</span>
        </div>
    `,
    styles: [
        `
            .site-select-option {
                display: flex;
                min-width: 0;
                align-items: center;
                gap: 0.5rem;
            }

            .site-select-option__domain {
                min-width: 0;
                overflow: hidden;
                text-overflow: ellipsis;
                white-space: nowrap;
            }

            .site-select-option--selected .site-select-option__domain {
                font-size: 0.875rem;
                font-weight: 600;
            }
        `
    ]
})
export class SiteSelectOption {
    readonly site = input.required<SiteFaviconSource | null>();
    readonly selected = input(false);
}
