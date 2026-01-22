import { ChangeDetectionStrategy, Component } from '@angular/core';
import { UserControls } from '../user-controls/user-controls';

@Component({
  selector: 'app-page-header',
  standalone: true,
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [UserControls],
  template: `
    <div class="flex flex-col gap-2 mb-8 pb-4 pt-1 border-b border-surface-200 dark:border-surface-700 -mx-4 md:-mx-8 px-4 md:px-8">
      <div class="flex justify-between items-center">
        <ng-content select="[header-left]"></ng-content>
        <div class="hidden md:flex shrink-0">
          <app-user-controls />
        </div>
      </div>
    </div>
  `
})
export class PageHeader {}
