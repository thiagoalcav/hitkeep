import { ChangeDetectionStrategy, Component, computed, effect, inject, signal } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ButtonModule } from 'primeng/button';
import { DialogModule } from 'primeng/dialog';
import { InputTextModule } from 'primeng/inputtext';
import { ShareService } from '../../../core/services/share.service';
import { SiteService } from '../../sites/services/site.service';

@Component({
  selector: 'app-share-dashboard-link',
  imports: [CommonModule, ButtonModule, DialogModule, InputTextModule],
  templateUrl: './share-dashboard-link.html',
  styleUrl: './share-dashboard-link.css',
  changeDetection: ChangeDetectionStrategy.OnPush
})
export class ShareDashboardLink {
  private shareService = inject(ShareService);
  protected siteService = inject(SiteService);

  protected isShareMode = computed(() => this.shareService.isShareMode());
  protected showShareDialog = signal(false);
  protected shareLinkUrl = signal<string | null>(null);
  protected shareLoading = signal(false);
  protected shareError = signal<string | null>(null);
  protected shareCopyLabel = signal('Copy Link');
  protected shareCopyIcon = signal('pi pi-copy');
  private shareSiteId = signal<string | null>(null);

  constructor() {
    effect(() => {
      const siteId = this.siteService.activeSite()?.id ?? null;
      if (this.shareSiteId() !== siteId) {
        this.shareSiteId.set(siteId);
        this.resetShareState();
      }
    });
  }

  protected openShareDialog() {
    this.shareError.set(null);
    this.resetShareCopyState();
    this.showShareDialog.set(true);
  }

  open() {
    if (this.isShareMode()) return;
    this.openShareDialog();
  }

  protected generateShareLink() {
    const siteId = this.siteService.activeSite()?.id;
    if (!siteId || this.shareLoading()) return;

    this.shareLoading.set(true);
    this.shareError.set(null);

    this.shareService.createShareLink(siteId).subscribe({
      next: (res) => {
        this.shareLinkUrl.set(res.url);
        this.shareLoading.set(false);
        this.resetShareCopyState();
      },
      error: () => {
        this.shareError.set('Unable to create a share link right now. Please try again.');
        this.shareLoading.set(false);
      }
    });
  }

  protected copyShareLink() {
    const url = this.shareLinkUrl();
    if (!url) return;

    navigator.clipboard.writeText(url).then(() => {
      this.shareCopyLabel.set('Copied!');
      this.shareCopyIcon.set('pi pi-check');
      setTimeout(() => this.resetShareCopyState(), 2000);
    });
  }

  protected resetShareDialog() {
    this.showShareDialog.set(false);
    this.shareError.set(null);
  }

  private resetShareState() {
    this.shareLinkUrl.set(null);
    this.shareLoading.set(false);
    this.shareError.set(null);
    this.resetShareCopyState();
  }

  private resetShareCopyState() {
    this.shareCopyLabel.set('Copy Link');
    this.shareCopyIcon.set('pi pi-copy');
  }
}
