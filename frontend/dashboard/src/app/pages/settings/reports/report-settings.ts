import { ChangeDetectionStrategy, Component, computed, effect, inject, signal } from "@angular/core";
import { FormsModule } from "@angular/forms";
import { forkJoin } from "rxjs";
import { toSignal } from "@angular/core/rxjs-interop";
import { TranslocoPipe, TranslocoService } from "@jsverse/transloco";
import { ButtonModule } from "primeng/button";
import { DividerModule } from "primeng/divider";
import { ToggleSwitchModule } from "primeng/toggleswitch";
import { PageHeader, PageHeaderLeft } from "@components/page-header/page-header";
import { PageBreadcrumb, PageBreadcrumbItem } from "@components/page-breadcrumb/page-breadcrumb";
import { SettingsCard } from "@features/settings/components/settings-card";
import { ReportSubscriptionsService } from "@services/report-subscriptions.service";
import { FrequencyPrefs, SiteReportSubscription } from "@core/models/analytics.types";

@Component({
    selector: "app-report-settings",
    imports: [FormsModule, ButtonModule, DividerModule, ToggleSwitchModule, SettingsCard, PageHeader, PageHeaderLeft, PageBreadcrumb, TranslocoPipe],
    templateUrl: "./report-settings.html",
    styleUrl: "./report-settings.css",
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class ReportSettings {
    private service = inject(ReportSubscriptionsService);
    private transloco = inject(TranslocoService);
    private activeLanguage = toSignal(this.transloco.langChanges$, { initialValue: this.transloco.getActiveLang() });

    protected readonly isLoading = this.service.isLoading;

    protected readonly digestPrefs = signal<FrequencyPrefs>({ daily: false, weekly: false, monthly: false });
    protected readonly sitePrefs = signal<SiteReportSubscription[]>([]);

    protected readonly digestSaveState = signal<"idle" | "saved" | "error">("idle");
    protected readonly sitesSaveState = signal<"idle" | "saved" | "error">("idle");
    protected readonly isDigestSaving = signal(false);
    protected readonly isSitesSaving = signal(false);

    protected readonly breadcrumbItems = computed<PageBreadcrumbItem[]>(() => {
        this.activeLanguage();
        return [{ label: this.transloco.translate("settings.reports.breadcrumb"), isCurrent: true }];
    });

    constructor() {
        this.service.load().subscribe({
            error: () => undefined
        });

        effect(() => {
            const subs = this.service.subscriptions();
            if (!subs) return;
            this.digestPrefs.set({ ...subs.digest });
            this.sitePrefs.set(subs.sites.map((s) => ({ ...s })));
            this.digestSaveState.set("idle");
            this.sitesSaveState.set("idle");
        });
    }

    protected setDigestFreq(freq: keyof FrequencyPrefs, value: boolean): void {
        this.digestPrefs.update((p) => ({ ...p, [freq]: value }));
        if (this.digestSaveState() !== "idle") this.digestSaveState.set("idle");
    }

    protected setSiteFreq(index: number, freq: keyof FrequencyPrefs, value: boolean): void {
        this.sitePrefs.update((sites) => {
            const copy = sites.map((s) => ({ ...s }));
            copy[index] = { ...copy[index], [freq]: value };
            return copy;
        });
        if (this.sitesSaveState() !== "idle") this.sitesSaveState.set("idle");
    }

    protected saveDigest(): void {
        this.isDigestSaving.set(true);
        this.digestSaveState.set("idle");
        this.service.updateDigestSubscription(this.digestPrefs()).subscribe({
            next: () => this.digestSaveState.set("saved"),
            error: () => this.digestSaveState.set("error"),
            complete: () => this.isDigestSaving.set(false)
        });
    }

    protected saveSites(): void {
        const sites = this.sitePrefs();
        if (sites.length === 0) return;
        this.isSitesSaving.set(true);
        this.sitesSaveState.set("idle");

        forkJoin(
            sites.map((site) =>
                this.service.updateSiteSubscription(site.site_id, {
                    daily: site.daily,
                    weekly: site.weekly,
                    monthly: site.monthly
                })
            )
        ).subscribe({
            next: () => this.sitesSaveState.set("saved"),
            error: () => this.sitesSaveState.set("error"),
            complete: () => this.isSitesSaving.set(false)
        });
    }
}
