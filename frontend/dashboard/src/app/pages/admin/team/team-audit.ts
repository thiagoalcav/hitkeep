import { HttpErrorResponse } from "@angular/common/http";
import { ChangeDetectionStrategy, Component, computed, effect, inject, signal } from "@angular/core";
import { finalize } from "rxjs";
import { TranslocoPipe } from "@jsverse/transloco";
import { RelativeDateTime } from "@components/relative-date-time/relative-date-time";
import { TeamAuditEntry } from "@models/analytics.types";
import { TeamService } from "@services/team.service";
import { ButtonModule } from "primeng/button";
import { TableModule } from "primeng/table";
import { TagModule } from "primeng/tag";

@Component({
    selector: "app-team-audit",
    imports: [ButtonModule, TableModule, TagModule, RelativeDateTime, TranslocoPipe],
    templateUrl: "./team-audit.html",
    styleUrl: "./team-audit.css",
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class TeamAuditPage {
    private readonly teamService = inject(TeamService);

    protected readonly team = this.teamService.activeTeam;
    protected readonly rows = signal<TeamAuditEntry[]>([]);
    protected readonly isLoading = signal(false);
    protected readonly errorKey = signal<string | null>(null);

    protected readonly canViewAudit = computed(() => {
        const role = this.team()?.role;
        return role === "owner" || role === "admin";
    });

    constructor() {
        effect(() => {
            this.team()?.id;
            this.refresh();
        });
    }

    protected refresh() {
        const teamID = this.team()?.id;
        if (!teamID || !this.canViewAudit()) {
            this.rows.set([]);
            return;
        }

        this.errorKey.set(null);
        this.isLoading.set(true);
        this.teamService
            .listTeamAudit(teamID)
            .pipe(finalize(() => this.isLoading.set(false)))
            .subscribe({
                next: (entries) => this.rows.set(entries),
                error: (error: unknown) => {
                    if (error instanceof HttpErrorResponse && error.status === 403) {
                        this.errorKey.set("admin.team.audit.forbidden");
                        return;
                    }
                    this.errorKey.set("admin.team.audit.loadError");
                }
            });
    }

    protected actionSeverity(action: string): "danger" | "info" | "success" | "secondary" {
        if (action.startsWith("member.remove") || action.startsWith("member.left")) {
            return "danger";
        }
        if (action.startsWith("member.role")) {
            return "info";
        }
        if (action.startsWith("member.add") || action.startsWith("member.invited")) {
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
            case "member.added":
                return "admin.team.audit.actions.memberAdded";
            case "member.role_updated":
                return "admin.team.audit.actions.memberRoleUpdated";
            case "member.removed":
                return "admin.team.audit.actions.memberRemoved";
            case "member.left":
                return "admin.team.audit.actions.memberLeft";
            default:
                return "admin.team.audit.actions.unknown";
        }
    }
}
