import {Component, computed, model, signal, ChangeDetectionStrategy} from '@angular/core';
import {CommonModule} from '@angular/common';
import {FormsModule} from '@angular/forms';
import {DialogModule} from 'primeng/dialog';
import {ButtonModule} from 'primeng/button';
import {ToggleSwitchModule} from 'primeng/toggleswitch';

@Component({
  selector: 'app-tracking-code',
  standalone: true,
  imports: [CommonModule, FormsModule, DialogModule, ButtonModule, ToggleSwitchModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <p-dialog
      header="Tracking Code"
      [(visible)]="visible"
      [modal]="true"
      [style]="{ width: '700px', maxWidth: '90vw' }"
      [draggable]="false"
      (onShow)="resetState()">

      <div class="flex flex-col gap-6 pt-2">

        <div class="text-[var(--p-text-muted-color)] leading-relaxed">
          <p>Copy and paste the code below into your website's HTML. We recommend placing it just before the closing
            <code
              class="text-[var(--p-text-color)] bg-[var(--p-surface-100)] dark:bg-[var(--p-surface-800)] px-1 rounded">&lt;/body&gt;</code>
            tag.</p>
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

        <!-- TODO: Move to own component -->
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
      <ng-template pTemplate="footer">
        <p-button label="Close" (onClick)="visible.set(false)" styleClass="p-button-text"/>
      </ng-template>
    </p-dialog>
  `
})
export class TrackingCode {
  visible = model<boolean>(false);
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

  resetState() {
    this.collectDnt.set(false);
    this.disableBeacon.set(false);
    this.resetCopyButton();
  }

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
