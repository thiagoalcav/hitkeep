import { ChangeDetectionStrategy, Component, computed, inject, input } from '@angular/core';
import { DOCUMENT, NgOptimizedImage } from '@angular/common';
import { browserAppUrl } from '@core/interceptors/base-path.interceptor';

export interface SiteFaviconSource {
    domain: string;
}

@Component({
    selector: 'app-site-favicon',
    standalone: true,
    imports: [NgOptimizedImage],
    changeDetection: ChangeDetectionStrategy.OnPush,
    template: ` <img [ngSrc]="faviconUrl()" class="size-5 max-w-5 rounded-full" [width]="20" [height]="20" loading="lazy" [alt]="domain()" /> `
})
export class SiteFavicon {
    private document = inject(DOCUMENT);
    site = input.required<SiteFaviconSource | null>();
    protected domain = computed(() => this.site()?.domain ?? '');
    protected faviconUrl = computed(() => {
        const domain = this.domain();
        return domain ? browserAppUrl(this.document, `/api/favicon/${encodeURIComponent(domain)}`) : '';
    });
}
