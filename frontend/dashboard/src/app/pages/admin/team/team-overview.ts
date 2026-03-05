import { ChangeDetectionStrategy, Component, inject } from "@angular/core";
import { TranslocoPipe, TranslocoService } from "@jsverse/transloco";
import { CardModule } from "primeng/card";
import { TagModule } from "primeng/tag";
import { TeamService } from "@services/team.service";
import { TeamRole } from "@models/analytics.types";

@Component({
    selector: "app-team-overview",
    imports: [CardModule, TagModule, TranslocoPipe],
    templateUrl: "./team-overview.html",
    styleUrl: "./team-overview.css",
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class TeamOverviewPage {
    private readonly transloco = inject(TranslocoService);
    protected readonly teamService = inject(TeamService);

    protected readonly team = this.teamService.activeTeam;

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
}
