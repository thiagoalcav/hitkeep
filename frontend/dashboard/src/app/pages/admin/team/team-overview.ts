import { ChangeDetectionStrategy, Component, computed, inject } from "@angular/core";
import { TranslocoPipe, TranslocoService } from "@jsverse/transloco";
import { CardModule } from "primeng/card";
import { ProgressBarModule } from "primeng/progressbar";
import { TagModule } from "primeng/tag";
import { TeamService } from "@services/team.service";
import { TeamRole } from "@models/analytics.types";

@Component({
    selector: "app-team-overview",
    imports: [CardModule, ProgressBarModule, TagModule, TranslocoPipe],
    templateUrl: "./team-overview.html",
    styleUrl: "./team-overview.css",
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class TeamOverviewPage {
    private readonly transloco = inject(TranslocoService);
    protected readonly teamService = inject(TeamService);

    protected readonly team = this.teamService.activeTeam;
    protected readonly usageCards = computed(() => {
        const team = this.team();
        if (!team?.usage || !team.entitlements) {
            return [];
        }

        return [
            this.buildUsageCard("monthlyEvents", team.usage.current_monthly_events, team.entitlements.max_monthly_events),
            this.buildUsageCard("sites", team.usage.current_sites, team.entitlements.max_sites_per_team),
            this.buildUsageCard("members", team.usage.current_members, team.entitlements.max_team_members)
        ];
    });

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

    protected roleLabel(role: TeamRole): string {
        return this.transloco.translate(`teams.roles.${role}`);
    }

    protected usageDescription(current: number): string {
        return this.transloco.translate("admin.team.overview.usage.currentUsage", { count: current });
    }

    protected usageLimitLabel(limit: number): string {
        if (limit <= 0) {
            return this.transloco.translate("admin.team.overview.usage.unlimited");
        }
        return this.transloco.translate("admin.team.overview.usage.limitValue", { count: limit });
    }

    protected usageStateClass(percentage: number, limit: number): string {
        if (limit <= 0) {
            return "team-overview__usage-card--unlimited";
        }
        if (percentage >= 95) {
            return "team-overview__usage-card--critical";
        }
        if (percentage >= 80) {
            return "team-overview__usage-card--warning";
        }
        return "team-overview__usage-card--healthy";
    }

    protected pendingInviteLabel(count: number): string {
        if (count === 1) {
            return this.transloco.translate("admin.team.overview.usage.pendingInviteOne");
        }
        return this.transloco.translate("admin.team.overview.usage.pendingInviteMany", { count });
    }

    private buildUsageCard(key: string, current: number, limit: number) {
        const percentage = limit > 0 ? Math.min(100, Math.round((current / limit) * 100)) : 0;

        return {
            key,
            current,
            limit,
            displayValue: current.toLocaleString(),
            percentage,
            className: this.usageStateClass(percentage, limit),
            hasFiniteLimit: limit > 0,
            limitLabel: this.usageLimitLabel(limit),
            description: this.usageDescription(current)
        };
    }
}
