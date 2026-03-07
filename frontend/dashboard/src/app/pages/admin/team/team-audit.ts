import { HttpErrorResponse } from "@angular/common/http";
import { ChangeDetectionStrategy, Component, computed, effect, inject, signal } from "@angular/core";
import { toSignal } from "@angular/core/rxjs-interop";
import { FormControl, ReactiveFormsModule } from "@angular/forms";
import { finalize, startWith } from "rxjs";
import { TranslocoPipe } from "@jsverse/transloco";
import { RelativeDateTime } from "@components/relative-date-time/relative-date-time";
import { TeamAuditEntry } from "@models/analytics.types";
import { TeamService } from "@services/team.service";
import { ButtonModule } from "primeng/button";
import { SelectModule } from "primeng/select";
import { TableModule } from "primeng/table";
import { TagModule } from "primeng/tag";

@Component({
    selector: "app-team-audit",
    imports: [ReactiveFormsModule, ButtonModule, SelectModule, TableModule, TagModule, RelativeDateTime, TranslocoPipe],
    templateUrl: "./team-audit.html",
    styleUrl: "./team-audit.css",
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class TeamAuditPage {
    private readonly teamService = inject(TeamService);
    private readonly pageSize = 25;

    protected readonly team = this.teamService.activeTeam;
    protected readonly actionControl = new FormControl("", { nonNullable: true });
    protected readonly selectedAction = toSignal(this.actionControl.valueChanges.pipe(startWith(this.actionControl.value)), { initialValue: this.actionControl.value });
    protected readonly rows = signal<TeamAuditEntry[]>([]);
    protected readonly isLoading = signal(false);
    protected readonly errorKey = signal<string | null>(null);
    protected readonly total = signal(0);
    protected readonly hasMore = signal(false);

    protected readonly actionOptions = computed(() => [
        { label: "admin.team.audit.filters.all", value: "" },
        { label: "admin.team.audit.actions.teamCreated", value: "team.created" },
        { label: "admin.team.audit.actions.teamUpdated", value: "team.updated" },
        { label: "admin.team.audit.actions.memberInvited", value: "member.invited" },
        { label: "admin.team.audit.actions.memberInviteAccepted", value: "member.invite_accepted" },
        { label: "admin.team.audit.actions.memberInviteResent", value: "member.invite_resent" },
        { label: "admin.team.audit.actions.memberInviteRevoked", value: "member.invite_revoked" },
        { label: "admin.team.audit.actions.memberAdded", value: "member.added" },
        { label: "admin.team.audit.actions.memberRoleUpdated", value: "member.role_updated" },
        { label: "admin.team.audit.actions.memberRemoved", value: "member.removed" },
        { label: "admin.team.audit.actions.memberLeft", value: "member.left" },
        { label: "admin.team.audit.actions.ownershipTransferred", value: "ownership.transferred" },
        { label: "admin.team.audit.actions.teamArchived", value: "team.archived" },
        { label: "admin.team.audit.actions.siteTransferredIn", value: "site.transferred_in" },
        { label: "admin.team.audit.actions.siteTransferredOut", value: "site.transferred_out" }
    ]);

    protected readonly canViewAudit = computed(() => {
        const role = this.team()?.role;
        return role === "owner" || role === "admin";
    });

    protected readonly loadedSummaryKey = computed(() => {
        if (this.rows().length === 0) {
            return "admin.team.audit.summary.empty";
        }

        if (this.hasMore()) {
            return "admin.team.audit.summary.partial";
        }

        return "admin.team.audit.summary.complete";
    });

    constructor() {
        effect(() => {
            this.selectedAction();
            this.refresh(this.team()?.id ?? null, true);
        });
    }

    protected refresh(teamID: string | null = this.team()?.id ?? null, reset = true) {
        if (!teamID || !this.canViewAudit()) {
            this.rows.set([]);
            this.total.set(0);
            this.hasMore.set(false);
            return;
        }

        const offset = reset ? 0 : this.rows().length;
        this.errorKey.set(null);
        this.isLoading.set(true);
        this.teamService
            .listTeamAudit(teamID, {
                action: this.selectedAction() || undefined,
                limit: this.pageSize,
                offset
            })
            .pipe(finalize(() => this.isLoading.set(false)))
            .subscribe({
                next: (response) => {
                    this.rows.set(reset ? response.entries : [...this.rows(), ...response.entries]);
                    this.total.set(response.total);
                    this.hasMore.set(response.has_more);
                },
                error: (error: unknown) => {
                    if (error instanceof HttpErrorResponse && error.status === 403) {
                        this.errorKey.set("admin.team.audit.forbidden");
                        return;
                    }
                    this.errorKey.set("admin.team.audit.loadError");
                }
            });
    }

    protected loadMore() {
        this.refresh(this.team()?.id ?? null, false);
    }

    protected actionSeverity(action: string): "danger" | "info" | "success" | "secondary" {
        if (action.startsWith("member.remove") || action.startsWith("member.left") || action.startsWith("team.archived")) {
            return "danger";
        }
        if (action.startsWith("member.role") || action.startsWith("ownership.transferred")) {
            return "info";
        }
        if (action.startsWith("member.add") || action.startsWith("member.invite") || action.startsWith("site.transferred")) {
            return "success";
        }
        return "secondary";
    }

    protected actionLabel(action: string): string {
        return this.translateAction(action);
    }

    private translateAction(action: string): string {
        switch (action) {
            case "team.created":
                return "admin.team.audit.actions.teamCreated";
            case "team.updated":
                return "admin.team.audit.actions.teamUpdated";
            case "member.invited":
                return "admin.team.audit.actions.memberInvited";
            case "member.invite_accepted":
                return "admin.team.audit.actions.memberInviteAccepted";
            case "member.invite_resent":
                return "admin.team.audit.actions.memberInviteResent";
            case "member.invite_revoked":
                return "admin.team.audit.actions.memberInviteRevoked";
            case "member.added":
                return "admin.team.audit.actions.memberAdded";
            case "member.role_updated":
                return "admin.team.audit.actions.memberRoleUpdated";
            case "member.removed":
                return "admin.team.audit.actions.memberRemoved";
            case "member.left":
                return "admin.team.audit.actions.memberLeft";
            case "ownership.transferred":
                return "admin.team.audit.actions.ownershipTransferred";
            case "team.archived":
                return "admin.team.audit.actions.teamArchived";
            case "site.transferred_in":
                return "admin.team.audit.actions.siteTransferredIn";
            case "site.transferred_out":
                return "admin.team.audit.actions.siteTransferredOut";
            default:
                return "admin.team.audit.actions.unknown";
        }
    }
}
