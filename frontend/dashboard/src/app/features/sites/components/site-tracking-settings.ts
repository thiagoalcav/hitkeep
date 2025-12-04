import { Component, input, computed, signal } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { Site } from '../../../core/models/analytics.types';
import { ToggleSwitchModule } from 'primeng/toggleswitch';

@Component({
  selector: 'app-site-tracking-settings',
  standalone: true,
  imports: [CommonModule, FormsModule, ToggleSwitchModule],
  template: `
    <div class="flex flex-col gap-6">
      <div class="text-[var(--p-text-muted-color)] leading-relaxed">
        <p>Copy and paste the code below into your website's HTML.</p>
      </div>

      <div class="flex flex-col gap-4 py-4">
        <h4 class="sr-only">Tracking CodeConfiguration</h4>

        <div class="flex items-center justify-between">
          <div class="flex flex-col">
            <span class="font-medium">Collect DNT</span>
            <span class="text-xs text-[var(--p-text-muted-color)]">Track users even if "Do Not Track" is enabled in their browser.</span>
          </div>
          <p-toggleswitch [(ngModel)]="collectDnt"></p-toggleswitch>
        </div>

        <div class="flex items-center justify-between">
          <div class="flex flex-col">
            <span class="font-medium">Disable sendBeacon</span>
            <span class="text-xs text-[var(--p-text-muted-color)]">Use standard fetch requests instead of navigator.sendBeacon.</span>
          </div>
          <p-toggleswitch [(ngModel)]="disableBeacon"></p-toggleswitch>
        </div>
      </div>

      <div
        class="rounded-md border border-[var(--p-surface-border)] bg-[var(--p-surface-50)] dark:bg-[var(--p-surface-900)] overflow-hidden">
        <div
          class="flex justify-between items-center px-3 py-2 border-b border-[var(--p-surface-border)] bg-[var(--p-surface-100)] dark:bg-[var(--p-surface-800)]">
          <span class="text-xs font-mono font-medium text-[var(--p-text-muted-color)]">HTML</span>
          <button
            class="flex items-center gap-2 px-3 py-1.5 rounded hover:bg-[var(--p-surface-200)] dark:hover:bg-[var(--p-surface-700)] transition-colors text-xs font-medium text-[var(--p-text-color)] cursor-pointer focus:outline-none focus:ring-2 focus:ring-[var(--p-primary-color)]"
            (click)="copySnippet()"
            [attr.aria-label]="copyButtonLabel()">
            <i [class]="copyButtonIcon()"></i>
            <span>{{ copyButtonLabel() }}</span>
          </button>
        </div>

        <pre
          class="p-4 m-0 text-sm overflow-x-auto font-mono whitespace-pre-wrap break-all text-[var(--p-text-color)]">{{ snippetCode() }}</pre>
      </div>
    </div>
  `
})
export class SiteTrackingSettings {
  site = input.required<Site | null>();
  protected collectDnt = signal(false);
  protected disableBeacon = signal(false);
  protected copyButtonLabel = signal('Copy Code');
  protected copyButtonIcon = signal('pi pi-copy');

  protected snippetCode = computed(() => {
    const origin = window.location.origin;

    let attrs = '';
    if (this.collectDnt()) attrs += ' data-collect-dnt="true"';
    if (this.disableBeacon()) attrs += ' data-disable-beacon="true"';

    return `<script async src="${origin}/hk.js"${attrs}></script>`;
  });

  copySnippet() {
    navigator.clipboard.writeText(this.snippetCode()).then(() => {
      this.copyButtonLabel.set('Copied!');
      this.copyButtonIcon.set('pi pi-check');
      setTimeout(() => this.resetCopyButton(), 2000);
    });
  }

  private resetCopyButton() {
    this.copyButtonLabel.set('Copy Code');
    this.copyButtonIcon.set('pi pi-copy');
  }
}