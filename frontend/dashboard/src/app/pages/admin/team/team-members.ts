import { HttpErrorResponse } from "@angular/common/http";
import { ChangeDetectionStrategy, Component, computed, effect, inject, signal } from "@angular/core";
import { toSignal } from "@angular/core/rxjs-interop";
import { FormControl, ReactiveFormsModule, Validators } from "@angular/forms";
import { compatForm } from "@angular/forms/signals/compat";
import { TranslocoPipe, TranslocoService } from "@jsverse/transloco";
import { RelativeDateTime } from "@components/relative-date-time/relative-date-time";
import { TeamInvite, TeamMember, TeamRole } from "@models/analytics.types";
import { TeamService } from "@services/team.service";
import { ConfirmationService } from "primeng/api";
import { ButtonModule } from "primeng/button";
import { ConfirmPopupModule } from "primeng/confirmpopup";
import { InputTextModule } from "primeng/inputtext";
import { SelectModule } from "primeng/select";
import { TableModule } from "primeng/table";
import { TagModule } from "primeng/tag";
import { finalize, forkJoin } from "rxjs";

interface TeamRoleOption {
    label: string;
    value: TeamRole;
}

@Component({
    selector: "app-team-members",
    imports: [ReactiveFormsModule, ButtonModule, ConfirmPopupModule, InputTextModule, SelectModule, TableModule, TagModule, RelativeDateTime, TranslocoPipe],
    templateUrl: "./team-members.html",
    styleUrl: "./team-members.css",
    changeDetection: ChangeDetectionStrategy.OnPush,
    providers: [ConfirmationService]
})
export class TeamMembersPage {
    private readonly transloco = inject(TranslocoService);
    private readonly teamService = inject(TeamService);
    private readonly confirmationService = inject(ConfirmationService);
    private readonly activeLanguage = toSignal(this.transloco.langChanges$, { initialValue: this.transloco.getActiveLang() });

    protected readonly team = this.teamService.activeTeam;
    protected readonly members = signal<TeamMember[]>([]);
    protected readonly pendingInvites = signal<TeamInvite[]>([]);
    protected readonly isLoading = signal(false);
    protected readonly isInviting = signal(false);
    protected readonly removingUserID = signal<string | null>(null);
    protected readonly transferringUserID = signal<string | null>(null);
    protected readonly updatingUserIDs = signal<Record<string, boolean>>({});
    protected readonly inviteActionIDs = signal<Record<string, boolean>>({});
    protected readonly errorKey = signal<string | null>(null);
    protected readonly successKey = signal<string | null>(null);

    protected readonly confirmKey = "team-member-remove";
    protected readonly inviteConfirmKey = "team-invite-action";
    protected readonly ownershipConfirmKey = "team-ownership-transfer";

    protected readonly canManageMembers = computed(() => {
        const role = this.team()?.role;
        return role === "owner" || role === "admin";
    });
    protected readonly canTransferOwnership = computed(() => this.team()?.role === "owner");
    protected readonly ownerCount = computed(() => this.members().filter((member) => member.role === "owner").length);

    protected readonly inviteRoleOptions = computed(() => {
        this.activeLanguage();
        return this.assignableRoles().map((role) => ({
            label: this.roleLabel(role),
            value: role
        }));
    });

    private readonly roleControls = new Map<string, FormControl<TeamRole>>();
    private readonly inviteFormModel = signal({
        email: new FormControl("", { nonNullable: true, validators: [Validators.required, Validators.email, Validators.maxLength(320)] }),
        role: new FormControl<TeamRole>("member", { nonNullable: true, validators: [Validators.required] })
    });
    protected readonly inviteForm = compatForm(this.inviteFormModel);

    constructor() {
        effect(() => {
            const team = this.team();
            if (!team?.id) {
                this.members.set([]);
                this.pendingInvites.set([]);
                return;
            }
            this.loadTeamState(team.id, team.role);
        });
    }

    protected refreshMembers() {
        const team = this.team();
        if (!team?.id) return;
        this.loadTeamState(team.id, team.role);
    }

    protected inviteMember() {
        if (!this.canManageMembers()) return;
        if (this.inviteForm().invalid()) {
            this.inviteForm.email().markAsTouched();
            this.inviteForm.role().markAsTouched();
            return;
        }

        const teamID = this.team()?.id;
        if (!teamID) return;

        this.errorKey.set(null);
        this.successKey.set(null);
        this.isInviting.set(true);

        const email = this.inviteForm.email().value().trim().toLowerCase();
        const role = this.inviteForm.role().value();

        this.teamService
            .upsertTeamMember(teamID, { email, role })
            .pipe(finalize(() => this.isInviting.set(false)))
            .subscribe({
                next: (response) => {
                    this.inviteForm.email().control().reset("");
                    this.successKey.set(response.is_invite ? "teams.management.status.inviteSent" : "teams.management.status.memberUpdated");
                    this.loadTeamState(teamID, this.team()?.role ?? "member");
                },
                error: (error: unknown) => {
                    this.errorKey.set(this.resolveErrorKey(error, "teams.management.errors.inviteFailed"));
                }
            });
    }

    protected confirmRemoveMember(event: Event, member: TeamMember) {
        if (!this.canRemoveMember(member)) return;

        this.confirmationService.confirm({
            key: this.confirmKey,
            target: event.currentTarget as EventTarget,
            message: this.transloco.translate("teams.management.confirmRemove", { email: member.email }),
            icon: "pi pi-exclamation-triangle",
            rejectButtonProps: {
                label: this.transloco.translate("common.actions.cancel"),
                severity: "secondary",
                outlined: true
            },
            acceptButtonProps: {
                label: this.transloco.translate("teams.management.removeAction"),
                severity: "danger"
            },
            accept: () => this.removeMember(member)
        });
    }

    protected canEditMember(member: TeamMember): boolean {
        if (!this.canManageMembers()) return false;
        const actorRole = this.team()?.role;
        if (!actorRole) return false;
        return this.roleRank(member.role) >= this.roleRank(actorRole);
    }

    protected canRemoveMember(member: TeamMember): boolean {
        if (!this.canEditMember(member)) return false;
        if (member.role === "owner" && this.ownerCount() <= 1) return false;
        return true;
    }

    protected canTransferMember(member: TeamMember): boolean {
        if (!this.canTransferOwnership()) return false;
        return member.role !== "owner";
    }

    protected roleControlFor(member: TeamMember): FormControl<TeamRole> {
        const existing = this.roleControls.get(member.user_id);
        if (existing) return existing;
        const control = new FormControl<TeamRole>(member.role, { nonNullable: true });
        this.roleControls.set(member.user_id, control);
        return control;
    }

    protected roleOptionsForMember(member: TeamMember): TeamRoleOption[] {
        if (!this.canEditMember(member)) {
            return [{ label: this.roleLabel(member.role), value: member.role }];
        }
        return this.inviteRoleOptions();
    }

    protected onRoleChange(member: TeamMember, role: TeamRole) {
        const teamID = this.team()?.id;
        if (!teamID || !this.canEditMember(member) || role === member.role) {
            this.roleControlFor(member).setValue(member.role, { emitEvent: false });
            return;
        }

        if (member.role === "owner" && role !== "owner" && this.ownerCount() <= 1) {
            this.errorKey.set("teams.management.errors.lastOwner");
            this.roleControlFor(member).setValue(member.role, { emitEvent: false });
            return;
        }

        this.errorKey.set(null);
        this.successKey.set(null);
        this.setRoleUpdateState(member.user_id, true);

        this.teamService
            .upsertTeamMember(teamID, { email: member.email, role })
            .pipe(finalize(() => this.setRoleUpdateState(member.user_id, false)))
            .subscribe({
                next: () => {
                    this.successKey.set("teams.management.status.roleUpdated");
                    this.loadTeamState(teamID, this.team()?.role ?? "member");
                },
                error: (error: unknown) => {
                    this.errorKey.set(this.resolveErrorKey(error, "teams.management.errors.roleUpdateFailed"));
                    this.roleControlFor(member).setValue(member.role, { emitEvent: false });
                }
            });
    }

    protected isRoleUpdating(member: TeamMember): boolean {
        return Boolean(this.updatingUserIDs()[member.user_id]);
    }

    protected roleLabel(role: TeamRole): string {
        return this.transloco.translate(`teams.roles.${role}`);
    }

    protected roleSeverity(role: TeamRole): "danger" | "info" | "secondary" {
        switch (role) {
            case "owner":
                return "danger";
            case "admin":
                return "info";
            case "member":
            default:
                return "secondary";
        }
    }

    protected confirmResendInvite(event: Event, invite: TeamInvite) {
        const target = event.currentTarget;
        if (!(target instanceof HTMLElement)) return;
        this.confirmationService.confirm({
            key: this.inviteConfirmKey,
            target,
            message: this.transloco.translate("teams.management.confirmResendInvite", { email: invite.email }),
            icon: "pi pi-envelope",
            rejectButtonProps: {
                label: this.transloco.translate("common.actions.cancel"),
                severity: "secondary",
                outlined: true
            },
            acceptButtonProps: {
                label: this.transloco.translate("teams.management.resendAction")
            },
            accept: () => this.resendInvite(invite)
        });
    }

    protected confirmRevokeInvite(event: Event, invite: TeamInvite) {
        const target = event.currentTarget;
        if (!(target instanceof HTMLElement)) return;
        this.confirmationService.confirm({
            key: this.inviteConfirmKey,
            target,
            message: this.transloco.translate("teams.management.confirmRevokeInvite", { email: invite.email }),
            icon: "pi pi-exclamation-triangle",
            rejectButtonProps: {
                label: this.transloco.translate("common.actions.cancel"),
                severity: "secondary",
                outlined: true
            },
            acceptButtonProps: {
                label: this.transloco.translate("teams.management.revokeInviteAction"),
                severity: "danger"
            },
            accept: () => this.revokeInvite(invite)
        });
    }

    protected confirmTransferOwnership(event: Event, member: TeamMember) {
        const target = event.currentTarget;
        if (!(target instanceof HTMLElement) || !this.canTransferMember(member)) return;
        this.confirmationService.confirm({
            key: this.ownershipConfirmKey,
            target,
            message: this.transloco.translate("teams.management.confirmTransferOwnership", { email: member.email }),
            icon: "pi pi-shield",
            rejectButtonProps: {
                label: this.transloco.translate("common.actions.cancel"),
                severity: "secondary",
                outlined: true
            },
            acceptButtonProps: {
                label: this.transloco.translate("teams.management.transferOwnershipAction"),
                severity: "contrast"
            },
            accept: () => this.transferOwnership(member)
        });
    }

    protected isInviteActionLoading(invite: TeamInvite): boolean {
        return Boolean(this.inviteActionIDs()[invite.id]);
    }

    protected inviteRoleLabel(role: TeamRole): string {
        return this.roleLabel(role);
    }

    private loadTeamState(teamID: string, role: TeamRole) {
        this.errorKey.set(null);
        this.isLoading.set(true);
        if (role === "owner" || role === "admin") {
            forkJoin({
                members: this.teamService.listTeamMembers(teamID),
                invites: this.teamService.listTeamInvites(teamID)
            })
                .pipe(finalize(() => this.isLoading.set(false)))
                .subscribe({
                    next: ({ members, invites }) => {
                        this.members.set(members);
                        this.pendingInvites.set(invites);
                        this.syncRoleControls(members);
                    },
                    error: (error: unknown) => {
                        this.errorKey.set(this.resolveErrorKey(error, "teams.management.errors.loadFailed"));
                    }
                });
            return;
        }

        this.teamService
            .listTeamMembers(teamID)
            .pipe(finalize(() => this.isLoading.set(false)))
            .subscribe({
                next: (members) => {
                    this.members.set(members);
                    this.pendingInvites.set([]);
                    this.syncRoleControls(members);
                },
                error: (error: unknown) => {
                    this.errorKey.set(this.resolveErrorKey(error, "teams.management.errors.loadFailed"));
                }
            });
    }

    private removeMember(member: TeamMember) {
        const teamID = this.team()?.id;
        if (!teamID) return;

        this.errorKey.set(null);
        this.successKey.set(null);
        this.removingUserID.set(member.user_id);
        this.teamService
            .removeTeamMember(teamID, member.user_id)
            .pipe(finalize(() => this.removingUserID.set(null)))
            .subscribe({
                next: () => {
                    this.members.update((current) => current.filter((entry) => entry.user_id !== member.user_id));
                    this.roleControls.delete(member.user_id);
                    this.successKey.set("teams.management.status.memberRemoved");
                },
                error: (error: unknown) => {
                    this.errorKey.set(this.resolveErrorKey(error, "teams.management.errors.removeFailed"));
                }
            });
    }

    private assignableRoles(): TeamRole[] {
        const actorRole = this.team()?.role;
        if (actorRole === "owner") return ["admin", "member"];
        if (actorRole === "admin") return ["admin", "member"];
        return ["member"];
    }

    private resendInvite(invite: TeamInvite) {
        const teamID = this.team()?.id;
        if (!teamID) return;

        this.setInviteActionState(invite.id, true);
        this.errorKey.set(null);
        this.successKey.set(null);
        this.teamService
            .resendTeamInvite(teamID, invite.id)
            .pipe(finalize(() => this.setInviteActionState(invite.id, false)))
            .subscribe({
                next: () => {
                    this.successKey.set("teams.management.status.inviteResent");
                    this.loadTeamState(teamID, this.team()?.role ?? "member");
                },
                error: (error: unknown) => {
                    this.errorKey.set(this.resolveErrorKey(error, "teams.management.errors.inviteResendFailed"));
                }
            });
    }

    private revokeInvite(invite: TeamInvite) {
        const teamID = this.team()?.id;
        if (!teamID) return;

        this.setInviteActionState(invite.id, true);
        this.errorKey.set(null);
        this.successKey.set(null);
        this.teamService
            .revokeTeamInvite(teamID, invite.id)
            .pipe(finalize(() => this.setInviteActionState(invite.id, false)))
            .subscribe({
                next: () => {
                    this.pendingInvites.update((current) => current.filter((entry) => entry.id !== invite.id));
                    this.successKey.set("teams.management.status.inviteRevoked");
                },
                error: (error: unknown) => {
                    this.errorKey.set(this.resolveErrorKey(error, "teams.management.errors.inviteRevokeFailed"));
                }
            });
    }

    private transferOwnership(member: TeamMember) {
        const teamID = this.team()?.id;
        if (!teamID) return;

        this.transferringUserID.set(member.user_id);
        this.errorKey.set(null);
        this.successKey.set(null);
        this.teamService
            .transferTeamOwnership(teamID, member.user_id)
            .pipe(finalize(() => this.transferringUserID.set(null)))
            .subscribe({
                next: () => {
                    this.successKey.set("teams.management.status.ownershipTransferred");
                    this.teamService.loadTeams().subscribe({
                        next: () => this.loadTeamState(teamID, this.team()?.role ?? "member"),
                        error: () => this.loadTeamState(teamID, this.team()?.role ?? "member")
                    });
                },
                error: (error: unknown) => {
                    this.errorKey.set(this.resolveErrorKey(error, "teams.management.errors.ownershipTransferFailed"));
                }
            });
    }

    private roleRank(role: TeamRole): number {
        switch (role) {
            case "owner":
                return 0;
            case "admin":
                return 1;
            case "member":
            default:
                return 2;
        }
    }

    private setRoleUpdateState(userID: string, value: boolean) {
        this.updatingUserIDs.update((current) => {
            if (value) return { ...current, [userID]: true };
            const next = { ...current };
            delete next[userID];
            return next;
        });
    }

    private setInviteActionState(inviteID: string, value: boolean) {
        this.inviteActionIDs.update((current) => {
            if (value) return { ...current, [inviteID]: true };
            const next = { ...current };
            delete next[inviteID];
            return next;
        });
    }

    private syncRoleControls(members: TeamMember[]) {
        const memberIDs = new Set(members.map((member) => member.user_id));
        for (const userID of this.roleControls.keys()) {
            if (!memberIDs.has(userID)) this.roleControls.delete(userID);
        }
        for (const member of members) {
            const control = this.roleControlFor(member);
            if (control.value !== member.role) {
                control.setValue(member.role, { emitEvent: false });
            }
        }
    }

    private resolveErrorKey(error: unknown, fallback: string): string {
        if (!(error instanceof HttpErrorResponse)) return fallback;
        if (error.error && typeof error.error === "object" && "code" in error.error && typeof error.error.code === "string") {
            switch (error.error.code) {
                case "ownership_transfer_required":
                    return "teams.management.errors.transferRequired";
                case "team_last_owner":
                    return "teams.management.errors.lastOwner";
                case "ownership_transfer_target_invalid":
                    return "teams.management.errors.transferTargetInvalid";
                case "ownership_transfer_self":
                case "ownership_transfer_already_owner":
                    return "teams.management.errors.ownershipTransferConflict";
            }
        }
        if (error.status === 403) return "teams.management.errors.forbidden";
        if (error.status === 409) return "teams.management.errors.alreadyMember";
        if (error.status === 400 && typeof error.error === "string" && error.error.toLowerCase().includes("last owner")) {
            return "teams.management.errors.lastOwner";
        }
        return fallback;
    }
}
