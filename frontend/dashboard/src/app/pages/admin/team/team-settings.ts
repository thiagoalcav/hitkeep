import { ChangeDetectionStrategy, Component, ElementRef, computed, effect, inject, signal, viewChild } from '@angular/core';
import { NgOptimizedImage } from '@angular/common';
import { HttpErrorResponse } from '@angular/common/http';
import { FormControl, FormGroup, ReactiveFormsModule, Validators } from '@angular/forms';
import { TranslocoPipe, TranslocoService } from '@jsverse/transloco';
import { CardModule } from 'primeng/card';
import { InputTextModule } from 'primeng/inputtext';
import { ButtonModule } from 'primeng/button';
import { MessageModule } from 'primeng/message';
import { TEAM_CAPABILITIES } from '@core/access/capabilities';
import { AccessService } from '@services/access.service';
import { TeamActionErrorResponse, TeamService } from '@services/team.service';
import { SiteService } from '@features/sites/services/site.service';
import { PermissionService } from '@services/permission.service';
import { ActivatedRoute, Router } from '@angular/router';
import { ConfirmationService } from 'primeng/api';
import { ConfirmDialogModule } from 'primeng/confirmdialog';
import { finalize } from 'rxjs';
import { SettingsAPIClients } from '@features/settings/components/settings-api-clients';
import { dialogCancelButton, dialogDangerButton } from '@components/dialog-actions/dialog-actions';

@Component({
    selector: 'app-team-settings',
    imports: [CardModule, TranslocoPipe, ReactiveFormsModule, InputTextModule, ButtonModule, MessageModule, ConfirmDialogModule, SettingsAPIClients, NgOptimizedImage],
    templateUrl: './team-settings.html',
    styleUrl: './team-settings.css',
    changeDetection: ChangeDetectionStrategy.OnPush,
    providers: [ConfirmationService]
})
export class TeamSettingsPage {
    private readonly router = inject(Router);
    private readonly route = inject(ActivatedRoute);
    private readonly confirmationService = inject(ConfirmationService);
    private readonly transloco = inject(TranslocoService);
    private readonly access = inject(AccessService);
    protected readonly teamService = inject(TeamService);
    private readonly siteService = inject(SiteService);
    private readonly perms = inject(PermissionService);
    protected readonly team = this.teamService.activeTeam;
    private readonly apiClientsSection = viewChild<ElementRef<HTMLElement>>('apiClientsSection');

    protected readonly canEdit = computed(() => this.access.canActiveTeam(TEAM_CAPABILITIES.manageSettings));
    protected readonly canArchive = computed(() => this.access.canActiveTeam(TEAM_CAPABILITIES.archive));

    protected readonly isSaving = signal(false);
    protected readonly isLeaving = signal(false);
    protected readonly isArchiving = signal(false);
    protected readonly successKey = signal('');
    protected readonly errorKey = signal('');
    protected readonly leaveErrorKey = signal('');
    protected readonly leaveSuccessKey = signal('');
    protected readonly archiveErrorKey = signal('');
    protected readonly archiveSuccessKey = signal('');
    protected readonly form = new FormGroup({
        name: new FormControl('', { nonNullable: true, validators: [Validators.required, Validators.maxLength(120)] }),
        logo_url: new FormControl('', { nonNullable: true, validators: [Validators.maxLength(2048)] })
    });

    constructor() {
        effect(() => {
            const t = this.team();
            if (t) {
                this.form.patchValue({ name: t.name, logo_url: t.logo_url ?? '' }, { emitEvent: false });
            }
        });

        effect(() => {
            const section = this.route.snapshot.queryParamMap.get('section');
            const target = this.apiClientsSection()?.nativeElement;
            if (section === 'api-clients' && target) {
                queueMicrotask(() => {
                    target.scrollIntoView({ block: 'start', behavior: 'smooth' });
                    target.focus({ preventScroll: true });
                });
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

        this.successKey.set('');
        this.errorKey.set('');
        this.isSaving.set(true);

        const { name, logo_url } = this.form.getRawValue();
        this.teamService.updateTeam(t.id, { name, logo_url }).subscribe({
            next: () => {
                this.isSaving.set(false);
                this.successKey.set('admin.team.settings.saveSuccess');
            },
            error: () => {
                this.isSaving.set(false);
                this.errorKey.set('admin.team.settings.saveError');
            }
        });
    }

    protected confirmLeaveTeam(): void {
        if (this.isLeaving()) {
            return;
        }

        this.confirmationService.confirm({
            message: this.transloco.translate('admin.team.settings.leaveConfirm'),
            icon: 'pi pi-exclamation-triangle',
            rejectButtonProps: dialogCancelButton(this.transloco.translate('common.actions.cancel')),
            acceptButtonProps: dialogDangerButton(this.transloco.translate('admin.team.settings.leaveAction')),
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

        this.leaveErrorKey.set('');
        this.leaveSuccessKey.set('');
        this.isLeaving.set(true);

        this.teamService
            .leaveTeam(t.id)
            .pipe(finalize(() => this.isLeaving.set(false)))
            .subscribe({
                next: () => {
                    this.leaveSuccessKey.set('admin.team.settings.leaveSuccess');
                    this.refreshTeamContext();
                },
                error: (error: unknown) => {
                    const errorCode = this.extractTeamErrorCode(error);
                    if (errorCode === 'team_last_owner') {
                        this.leaveErrorKey.set('admin.team.settings.leaveErrors.lastOwner');
                        return;
                    }
                    if (errorCode === 'user_only_team') {
                        this.leaveErrorKey.set('admin.team.settings.leaveErrors.onlyTeam');
                        return;
                    }
                    if (error instanceof HttpErrorResponse && error.status === 403) {
                        this.leaveErrorKey.set('teams.management.errors.forbidden');
                        return;
                    }
                    this.leaveErrorKey.set('admin.team.settings.leaveErrors.generic');
                }
            });
    }

    protected confirmArchiveTeam(): void {
        if (this.isArchiving() || !this.canArchive()) {
            return;
        }

        this.confirmationService.confirm({
            message: this.transloco.translate('admin.team.settings.archiveConfirm'),
            icon: 'pi pi-exclamation-triangle',
            rejectButtonProps: dialogCancelButton(this.transloco.translate('common.actions.cancel')),
            acceptButtonProps: dialogDangerButton(this.transloco.translate('admin.team.settings.archiveAction')),
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

        this.archiveErrorKey.set('');
        this.archiveSuccessKey.set('');
        this.isArchiving.set(true);

        this.teamService
            .archiveTeam(t.id)
            .pipe(finalize(() => this.isArchiving.set(false)))
            .subscribe({
                next: () => {
                    this.archiveSuccessKey.set('admin.team.settings.archiveSuccess');
                    this.refreshTeamContext();
                },
                error: (error: unknown) => {
                    const errorCode = this.extractTeamErrorCode(error);
                    if (errorCode === 'team_archive_has_sites') {
                        this.archiveErrorKey.set('admin.team.settings.archiveErrors.hasSites');
                        return;
                    }
                    if (errorCode === 'team_archive_default_forbidden') {
                        this.archiveErrorKey.set('admin.team.settings.archiveErrors.defaultTeam');
                        return;
                    }
                    if (errorCode === 'team_archive_forbidden') {
                        this.archiveErrorKey.set('admin.team.settings.archiveErrors.forbidden');
                        return;
                    }
                    this.archiveErrorKey.set('admin.team.settings.archiveErrors.generic');
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
                this.router.navigateByUrl('/dashboard');
            },
            error: () => this.router.navigateByUrl('/dashboard')
        });
    }

    private extractTeamErrorCode(error: unknown): string | null {
        if (!(error instanceof HttpErrorResponse)) {
            return null;
        }
        const body = error.error;
        if (!body || typeof body !== 'object') {
            return null;
        }
        const actionError = body as Partial<TeamActionErrorResponse>;
        return typeof actionError.code === 'string' ? actionError.code : null;
    }
}
