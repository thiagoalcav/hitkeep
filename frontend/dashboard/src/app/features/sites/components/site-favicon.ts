import { Component, computed, input, signal, ChangeDetectionStrategy } from '@angular/core';
import {CommonModule, NgOptimizedImage} from '@angular/common';
import { Site } from '../../../core/models/analytics.types';
@Component({
  selector: 'app-site-favicon',
  standalone: true,
  imports: [CommonModule, NgOptimizedImage],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <img [ngSrc]="faviconUrl()" class="size-5 max-w-5" [width]="20" [height]="20" loading="lazy" [alt]="site()?.domain">
  `,
})
export class SiteFavicon {
  site = input.required<Site|null>();
  protected faviconUrl = computed(() => `/api/sites/${this.site()?.id}/favicon`);

}
