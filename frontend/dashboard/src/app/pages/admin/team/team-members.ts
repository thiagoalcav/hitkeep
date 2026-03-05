import { HttpErrorResponse } from "@angular/common/http";
import { ChangeDetectionStrategy, Component, computed, inject, signal } from "@angular/core";
import { toSignal } from "@angular/core/rxjs-interop";
import { FormControl, ReactiveFormsModule, Validators } from "@angular/forms";
import { compatForm } from "@angular/forms/signals/compat";
import { TranslocoPipe, TranslocoService } from "@jsverse/transloco";
import { RelativeDateTime } from "@components/relative-date-time/relative-date-time";
import { TeamMember, TeamRole } from "@models/analytics.types";
import { TeamService } from "@services/team.service";
import { ConfirmationService } from "primeng/api";
import { ButtonModule } from "primeng/button";
import { ConfirmPopupModule } from "primeng/confirmpopup";
import { InputTextModule } from "primeng/inputtext";
import { SelectModule } from "primeng/select";
import { TableModule } from "primeng/table";
import { TagModule } from "primeng/tag";
import { finalize } from "rxjs";

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
    protected readonly isLoading = signal(false);
    protected readonly isInviting = signal(false);
    protected readonly removingUserID = signal<string | null>(null);
    protected readonly updatingUserIDs = signal<Record<string, boolean>>({});
    protected readonly errorKey = signal<string | null>(null);
    protected readonly successKey = signal<string | null>(null);

    protected readonly confirmKey = "team-member-remove";

    protected readonly canManageMembers = computed(() => {
        const role = this.team()?.role;
        return role === "owner" || role === "admin";
    });
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
        const teamID = this.team()?.id;
        if (teamID) {
            this.loadMembers(teamID);
        }
    }

    protected refreshMembers() {
        const teamID = this.team()?.id;
        if (!teamID) return;
        this.loadMembers(teamID);
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
                    this.loadMembers(teamID);
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
                    this.loadMembers(teamID);
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

    private loadMembers(teamID: string) {
        this.errorKey.set(null);
        this.isLoading.set(true);
        this.teamService
            .listTeamMembers(teamID)
            .pipe(finalize(() => this.isLoading.set(false)))
            .subscribe({
                next: (members) => {
                    this.members.set(members);
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
        if (actorRole === "owner") return ["owner", "admin", "member"];
        if (actorRole === "admin") return ["admin", "member"];
        return ["member"];
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
        if (error.status === 403) return "teams.management.errors.forbidden";
        if (error.status === 409) return "teams.management.errors.alreadyMember";
        if (error.status === 400 && typeof error.error === "string" && error.error.toLowerCase().includes("last owner")) {
            return "teams.management.errors.lastOwner";
        }
        return fallback;
    }
}
