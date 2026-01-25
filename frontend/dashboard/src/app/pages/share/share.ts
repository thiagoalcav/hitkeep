import { ChangeDetectionStrategy, Component, inject, signal } from '@angular/core';
import { ActivatedRoute } from '@angular/router';
import { CommonModule } from '@angular/common';
import { Dashboard } from '@pages/dashboard/dashboard';
import { ShareService } from '@services/share.service';
import { SiteService } from '@features/sites/services/site.service';

@Component({
    selector: 'app-share-dashboard',
    standalone: true,
    imports: [CommonModule, Dashboard],
    templateUrl: './share.html',
    styleUrl: './share.css',
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class ShareDashboard {
    private route = inject(ActivatedRoute);
    private shareService = inject(ShareService);
    private siteService = inject(SiteService);

    protected loading = signal(true);
    protected error = signal<string | null>(null);

    constructor() {
        const token = this.route.snapshot.paramMap.get('token');
        if (!token) {
            this.error.set('Missing share token.');
            this.loading.set(false);
            return;
        }

        this.shareService.loadShareSite(token).subscribe({
            next: (site) => {
                this.siteService.sites.set([site]);
                this.siteService.activeSite.set(site);
                this.loading.set(false);
            },
            error: () => {
                this.error.set('Share link is invalid or expired.');
                this.loading.set(false);
            }
        });
    }
}
