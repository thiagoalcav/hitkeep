import { DOCUMENT } from '@angular/common';
import { ChangeDetectionStrategy, Component, DestroyRef, computed, inject, signal } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { RouterLink } from '@angular/router';
import { TranslocoPipe } from '@jsverse/transloco';
import { ButtonModule } from 'primeng/button';
import { finalize } from 'rxjs';

import { injectActiveLang } from '@core/i18n/active-lang';
import { CloudService } from '@services/cloud.service';
import { DashboardBootstrapService } from '@services/dashboard-bootstrap.service';
import { ShareService } from '@services/share.service';
import { TeamService } from '@services/team.service';

@Component({
    selector: 'app-free-plan-retention-notice',
    imports: [ButtonModule, RouterLink, TranslocoPipe],
    templateUrl: './free-plan-retention-notice.html',
    styleUrl: './free-plan-retention-notice.css',
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class FreePlanRetentionNotice {
    private static readonly dismissedValue = 'dismissed';

    private readonly bootstrap = inject(DashboardBootstrapService);
    private readonly cloud = inject(CloudService);
    private readonly destroyRef = inject(DestroyRef);
    private readonly document = inject(DOCUMENT);
    private readonly share = inject(ShareService);
    private readonly teamService = inject(TeamService);
    private readonly activeLanguage = injectActiveLang();

    private readonly dismissalRevision = signal(0);

    protected readonly checkoutPending = signal(false);
    protected readonly checkoutErrorKey = signal<string | null>(null);
    protected readonly team = this.teamService.activeTeam;
    protected readonly retentionDays = computed(() => this.team()?.entitlements?.max_retention_days || 60);
    protected readonly dismissalKey = computed(() => {
        const team = this.team();
        return team?.id ? `hitkeep.freeRetentionNotice.dismissed.${team.id}` : '';
    });
    protected readonly visible = computed(() => {
        this.dismissalRevision();

        const team = this.team();
        return Boolean(this.bootstrap.cloudHosted() && !this.share.isShareMode() && team?.plan?.code === 'free' && !this.isDismissed(this.dismissalKey()));
    });

    protected startUpgrade(): void {
        if (this.checkoutPending()) {
            return;
        }

        this.checkoutErrorKey.set(null);
        this.checkoutPending.set(true);
        this.cloud
            .createBillingCheckoutSession({
                plan_code: 'pro',
                locale: this.activeLanguage()
            })
            .pipe(
                finalize(() => this.checkoutPending.set(false)),
                takeUntilDestroyed(this.destroyRef)
            )
            .subscribe({
                next: ({ url }) => this.redirectTo(url),
                error: () => this.checkoutErrorKey.set('cloud.retentionNotice.checkoutError')
            });
    }

    protected dismiss(): void {
        const key = this.dismissalKey();
        if (!key) {
            return;
        }

        try {
            this.document.defaultView?.localStorage.setItem(key, FreePlanRetentionNotice.dismissedValue);
        } catch {
            // Browsers can deny localStorage in restricted contexts; dismissal is best-effort.
        }
        this.dismissalRevision.update((value) => value + 1);
    }

    protected redirectTo(url: string): void {
        this.document.defaultView?.location.assign(url);
    }

    private isDismissed(key: string): boolean {
        if (!key) {
            return false;
        }

        try {
            return this.document.defaultView?.localStorage.getItem(key) === FreePlanRetentionNotice.dismissedValue;
        } catch {
            return false;
        }
    }
}
