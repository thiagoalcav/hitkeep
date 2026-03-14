import { ChangeDetectionStrategy, Component, computed, effect, inject, signal } from "@angular/core";
import { NgOptimizedImage } from "@angular/common";
import { HttpErrorResponse } from "@angular/common/http";
import { FormControl, FormGroup, ReactiveFormsModule, Validators } from "@angular/forms";
import { TranslocoPipe, TranslocoService } from "@jsverse/transloco";
import { CardModule } from "primeng/card";
import { InputTextModule } from "primeng/inputtext";
import { ButtonModule } from "primeng/button";
import { MessageModule } from "primeng/message";
import { TeamActionErrorResponse, TeamService } from "@services/team.service";
import { SiteService } from "@features/sites/services/site.service";
import { PermissionService } from "@services/permission.service";
import { Router } from "@angular/router";
import { ConfirmationService } from "primeng/api";
import { ConfirmPopupModule } from "primeng/confirmpopup";
import { finalize } from "rxjs";
import { SettingsAPIClients } from "@features/settings/components/settings-api-clients";

@Component({
    selector: "app-team-settings",
    imports: [CardModule, TranslocoPipe, ReactiveFormsModule, InputTextModule, ButtonModule, MessageModule, ConfirmPopupModule, SettingsAPIClients, NgOptimizedImage],
    templateUrl: "./team-settings.html",
    styleUrl: "./team-settings.css",
    changeDetection: ChangeDetectionStrategy.OnPush,
    providers: [ConfirmationService]
})
export class TeamSettingsPage {
    private readonly router = inject(Router);
    private readonly confirmationService = inject(ConfirmationService);
    private readonly transloco = inject(TranslocoService);
    protected readonly teamService = inject(TeamService);
    private readonly siteService = inject(SiteService);
    private readonly perms = inject(PermissionService);
    protected readonly team = this.teamService.activeTeam;

    protected readonly canEdit = computed(() => {
        const role = this.team()?.role;
        return role === "owner" || role === "admin";
    });
    protected readonly canArchive = computed(() => this.team()?.role === "owner");

    protected readonly isSaving = signal(false);
    protected readonly isLeaving = signal(false);
    protected readonly isArchiving = signal(false);
    protected readonly successKey = signal("");
    protected readonly errorKey = signal("");
    protected readonly leaveErrorKey = signal("");
    protected readonly leaveSuccessKey = signal("");
    protected readonly archiveErrorKey = signal("");
    protected readonly archiveSuccessKey = signal("");
    protected readonly leaveConfirmKey = "team-settings-leave";
    protected readonly archiveConfirmKey = "team-settings-archive";

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

    protected confirmLeaveTeam(event: Event): void {
        const target = event.currentTarget;
        if (!(target instanceof HTMLElement) || this.isLeaving()) {
            return;
        }

        this.confirmationService.confirm({
            key: this.leaveConfirmKey,
            target,
            message: this.transloco.translate("admin.team.settings.leaveConfirm"),
            icon: "pi pi-exclamation-triangle",
            rejectButtonProps: {
                label: this.transloco.translate("common.actions.cancel"),
                severity: "secondary",
                outlined: true
            },
            acceptButtonProps: {
                label: this.transloco.translate("admin.team.settings.leaveAction"),
                severity: "danger"
            },
            accept: () => this.leaveTeam()
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
                    this.refreshTeamContext();
                },
                error: (error: unknown) => {
                    const errorCode = this.extractTeamErrorCode(error);
                    if (errorCode === "team_last_owner") {
                        this.leaveErrorKey.set("admin.team.settings.leaveErrors.lastOwner");
                        return;
                    }
                    if (errorCode === "user_only_team") {
                        this.leaveErrorKey.set("admin.team.settings.leaveErrors.onlyTeam");
                        return;
                    }
                    if (error instanceof HttpErrorResponse && error.status === 403) {
                        this.leaveErrorKey.set("teams.management.errors.forbidden");
                        return;
                    }
                    this.leaveErrorKey.set("admin.team.settings.leaveErrors.generic");
                }
            });
    }

    protected confirmArchiveTeam(event: Event): void {
        const target = event.currentTarget;
        if (!(target instanceof HTMLElement) || this.isArchiving() || !this.canArchive()) {
            return;
        }

        this.confirmationService.confirm({
            key: this.archiveConfirmKey,
            target,
            message: this.transloco.translate("admin.team.settings.archiveConfirm"),
            icon: "pi pi-exclamation-triangle",
            rejectButtonProps: {
                label: this.transloco.translate("common.actions.cancel"),
                severity: "secondary",
                outlined: true
            },
            acceptButtonProps: {
                label: this.transloco.translate("admin.team.settings.archiveAction"),
                severity: "danger"
            },
            accept: () => this.archiveTeam()
        });
    }

    protected archiveTeam(): void {
        if (this.isArchiving()) {
            return;
        }

        const t = this.team();
        if (!t || !this.canArchive()) {
            return;
        }

        this.archiveErrorKey.set("");
        this.archiveSuccessKey.set("");
        this.isArchiving.set(true);

        this.teamService
            .archiveTeam(t.id)
            .pipe(finalize(() => this.isArchiving.set(false)))
            .subscribe({
                next: () => {
                    this.archiveSuccessKey.set("admin.team.settings.archiveSuccess");
                    this.refreshTeamContext();
                },
                error: (error: unknown) => {
                    const errorCode = this.extractTeamErrorCode(error);
                    if (errorCode === "team_archive_has_sites") {
                        this.archiveErrorKey.set("admin.team.settings.archiveErrors.hasSites");
                        return;
                    }
                    if (errorCode === "team_archive_default_forbidden") {
                        this.archiveErrorKey.set("admin.team.settings.archiveErrors.defaultTeam");
                        return;
                    }
                    if (errorCode === "team_archive_forbidden") {
                        this.archiveErrorKey.set("admin.team.settings.archiveErrors.forbidden");
                        return;
                    }
                    this.archiveErrorKey.set("admin.team.settings.archiveErrors.generic");
                }
            });
    }

    private refreshTeamContext(): void {
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
    }

    private extractTeamErrorCode(error: unknown): string | null {
        if (!(error instanceof HttpErrorResponse)) {
            return null;
        }
        const body = error.error;
        if (!body || typeof body !== "object") {
            return null;
        }
        const actionError = body as Partial<TeamActionErrorResponse>;
        return typeof actionError.code === "string" ? actionError.code : null;
    }
}
