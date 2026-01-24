import { Component, input } from '@angular/core';

import { Site } from '../../../core/models/analytics.types';

@Component({
  selector: 'app-site-danger-zone',
  standalone: true,
  imports: [],
  template: `
    <div>
      <h3 class="font-bold">Danger Zone</h3>
      <p>Delete Site, etc.</p>
      <p>Site: {{ site()?.domain }}</p>
    </div>
  `
})
export class SiteDangerZone {
  site = input.required<Site | null>();
}