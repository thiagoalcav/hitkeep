import { Component, computed, input, ChangeDetectionStrategy } from '@angular/core';
import { NgOptimizedImage } from '@angular/common';
import { Site } from '../../../core/models/analytics.types';
@Component({
  selector: 'app-site-favicon',
  standalone: true,
  imports: [NgOptimizedImage],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <img [ngSrc]="faviconUrl()" class="size-5 max-w-5 rounded-full" [width]="20" [height]="20" loading="lazy" [alt]="site()?.domain">
  `,
})
export class SiteFavicon {
  site = input.required<Site | null>();
  protected faviconUrl = computed(() => {
    const domain = this.site()?.domain;
    return domain ? `/api/favicon/${encodeURIComponent(domain)}` : '';
  });
}
