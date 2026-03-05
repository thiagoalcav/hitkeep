import { ChangeDetectionStrategy, Component, computed, effect, inject, signal } from "@angular/core";
import { HttpErrorResponse } from "@angular/common/http";
import { FormControl, FormGroup, ReactiveFormsModule, Validators } from "@angular/forms";
import { TranslocoPipe } from "@jsverse/transloco";
import { CardModule } from "primeng/card";
import { InputTextModule } from "primeng/inputtext";
import { ButtonModule } from "primeng/button";
import { MessageModule } from "primeng/message";
import { TeamService } from "@services/team.service";
import { SiteService } from "@features/sites/services/site.service";
import { PermissionService } from "@services/permission.service";
import { Router } from "@angular/router";
import { finalize } from "rxjs";

@Component({
    selector: "app-team-settings",
    imports: [CardModule, TranslocoPipe, ReactiveFormsModule, InputTextModule, ButtonModule, MessageModule],
    templateUrl: "./team-settings.html",
    styleUrl: "./team-settings.css",
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class TeamSettingsPage {
    private readonly router = inject(Router);
    protected readonly teamService = inject(TeamService);
    private readonly siteService = inject(SiteService);
    private readonly perms = inject(PermissionService);
    protected readonly team = this.teamService.activeTeam;

    protected readonly canEdit = computed(() => {
        const role = this.team()?.role;
        return role === "owner" || role === "admin";
    });

    protected readonly isSaving = signal(false);
    protected readonly isLeaving = signal(false);
    protected readonly successKey = signal("");
    protected readonly errorKey = signal("");
    protected readonly leaveErrorKey = signal("");
    protected readonly leaveSuccessKey = signal("");

    protected readonly form = new FormGroup({
        name: new FormControl("", { nonNullable: true, validators: [Validators.required, Validators.maxLength(120)] }),
        logo_url: new FormControl("", { nonNullable: true, validators: [Validators.maxLength(2048)] })
    });

    constructor() {
        effect(() => {
            const t = this.team();
            if (t) {
                this.form.patchValue({ name: t.name, logo_url: t.logo_url ?? "" }, { emitEvent: false });
            }
        });
    }

    protected saveSettings(): void {
        if (this.form.invalid || this.isSaving()) {
            return;
        }

        const t = this.team();
        if (!t) {
            return;
        }

        this.successKey.set("");
        this.errorKey.set("");
        this.isSaving.set(true);

        const { name, logo_url } = this.form.getRawValue();
        this.teamService.updateTeam(t.id, { name, logo_url }).subscribe({
            next: () => {
                this.isSaving.set(false);
                this.successKey.set("admin.team.settings.saveSuccess");
            },
            error: () => {
                this.isSaving.set(false);
                this.errorKey.set("admin.team.settings.saveError");
            }
        });
    }

    protected leaveTeam(): void {
        if (this.isLeaving()) {
            return;
        }

        const t = this.team();
        if (!t) {
            return;
        }

        this.leaveErrorKey.set("");
        this.leaveSuccessKey.set("");
        this.isLeaving.set(true);

        this.teamService
            .leaveTeam(t.id)
            .pipe(finalize(() => this.isLeaving.set(false)))
            .subscribe({
                next: () => {
                    this.leaveSuccessKey.set("admin.team.settings.leaveSuccess");
                    this.siteService.sites.set([]);
                    this.siteService.activeSite.set(null);
                    this.siteService.loadSites();
                    this.perms.loadPermissions().subscribe({ error: () => undefined });
                    this.teamService.loadTeams().subscribe({
                        next: () => {
                            this.router.navigateByUrl("/dashboard");
                        },
                        error: () => this.router.navigateByUrl("/dashboard")
                    });
                },
                error: (error: unknown) => {
                    if (error instanceof HttpErrorResponse && error.status === 400) {
                        const errorText = typeof error.error === "string" ? error.error.toLowerCase() : "";
                        if (errorText.includes("last owner")) {
                            this.leaveErrorKey.set("admin.team.settings.leaveErrors.lastOwner");
                            return;
                        }
                        if (errorText.includes("only team")) {
                            this.leaveErrorKey.set("admin.team.settings.leaveErrors.onlyTeam");
                            return;
                        }
                    }
                    if (error instanceof HttpErrorResponse && error.status === 403) {
                        this.leaveErrorKey.set("teams.management.errors.forbidden");
                        return;
                    }
                    this.leaveErrorKey.set("admin.team.settings.leaveErrors.generic");
                }
            });
    }
}
