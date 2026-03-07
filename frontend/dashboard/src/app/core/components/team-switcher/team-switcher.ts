import { ChangeDetectionStrategy, Component, computed, inject, input, output, signal } from "@angular/core";
import { TranslocoPipe, TranslocoService } from "@jsverse/transloco";
import { Menu } from "primeng/menu";
import { MenuItem } from "primeng/api";
import { MenuModule } from "primeng/menu";
import { SkeletonModule } from "primeng/skeleton";
import { TagModule } from "primeng/tag";
import { Team, TeamRole } from "@models/analytics.types";

interface TeamSwitcherMenuItem extends MenuItem {
    team?: Team;
    active?: boolean;
    kind?: "team" | "action";
}

@Component({
    selector: "app-team-switcher",
    imports: [MenuModule, SkeletonModule, TagModule, TranslocoPipe],
    templateUrl: "./team-switcher.html",
    styleUrl: "./team-switcher.css",
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class TeamSwitcher {
    private static nextId = 0;

    private readonly transloco = inject(TranslocoService);

    protected readonly triggerId = `team-menu-trigger-${TeamSwitcher.nextId++}`;

    teams = input.required<Team[]>();
    currentTeamId = input<string>("");
    loading = input<boolean>(false);
    switching = input<boolean>(false);
    showBrand = input<boolean>(true);
    showAdd = input<boolean>(false);
    compact = input<boolean>(false);
    beforeSwitch = input<((nextTeam: Team) => boolean | Promise<boolean>) | undefined>(undefined);

    teamSelected = output<Team>();
    addClicked = output<void>();

    protected readonly isMenuOpen = signal(false);
    protected readonly activeTeam = computed(() => this.teams().find((team) => team.id === this.currentTeamId()) ?? this.teams()[0] ?? null);
    protected readonly activeTeamName = computed(() => this.activeTeam()?.name ?? this.transloco.translate("teams.switcher.placeholder"));
    protected readonly menuItems = computed<TeamSwitcherMenuItem[]>(() => {
        const items: TeamSwitcherMenuItem[] = this.teams().map((team) => ({
            id: team.id,
            label: team.name,
            team,
            active: team.id === this.activeTeam()?.id,
            kind: "team"
        }));

        if (this.showAdd()) {
            items.push({ separator: true });
            items.push({
                id: "create-team",
                label: "create-team",
                icon: "pi pi-plus",
                kind: "action"
            });
        }

        return items;
    });
    protected readonly menuStyle = computed(() => ({
        width: this.compact() ? "19rem" : "20rem"
    }));

    protected async onTeamOptionSelected(team: Team | undefined, menu: Menu) {
        if (!team || this.switching() || team.id === this.currentTeamId()) {
            return;
        }

        const proceed = await this.canSwitchTeam(team);
        if (!proceed) {
            menu.hide();
            return;
        }

        menu.hide();
        this.teamSelected.emit(team);
    }

    protected onCreateTeam(menu: Menu) {
        if (!this.showAdd()) {
            return;
        }
        menu.hide();
        this.addClicked.emit();
    }

    protected teamLogoUrl(team: Team | null): string {
        if (!team) {
            return this.generateInitialsAvatar({ id: "team", name: "Team", logo_url: "", role: "member", created_at: "" });
        }
        if (team.logo_url) {
            return team.logo_url;
        }
        return this.generateInitialsAvatar(team);
    }

    protected roleLabel(role: string): string {
        return this.transloco.translate(`teams.roles.${role}`);
    }

    protected roleSeverity(role: TeamRole | string): "danger" | "info" | "secondary" {
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

    private async canSwitchTeam(nextTeam: Team): Promise<boolean> {
        const checker = this.beforeSwitch();
        if (!checker) {
            return true;
        }
        const allowed = checker(nextTeam);
        return typeof (allowed as Promise<boolean>)?.then === "function" ? await allowed : Boolean(allowed);
    }

    private generateInitialsAvatar(team: Team): string {
        const initials = this.teamInitials(team.name);
        const palette = this.paletteFromSeed(team.id || team.name || "team");
        const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="40" height="40" viewBox="0 0 40 40" role="img" aria-label="${this.escapeAttr(team.name || "Team")}"><rect width="40" height="40" rx="20" fill="${palette.background}"/><text x="20" y="25" text-anchor="middle" font-family="Inter,Segoe UI,Arial,sans-serif" font-size="14" font-weight="700" fill="${palette.foreground}">${this.escapeText(initials)}</text></svg>`;
        return `data:image/svg+xml;utf8,${encodeURIComponent(svg)}`;
    }

    private teamInitials(name: string): string {
        const trimmed = (name || "").trim();
        if (!trimmed) {
            return "T";
        }
        const parts = trimmed.split(/\s+/).filter(Boolean);
        if (parts.length === 1) {
            return parts[0].slice(0, 2).toUpperCase();
        }
        return `${parts[0].charAt(0)}${parts[1].charAt(0)}`.toUpperCase();
    }

    private paletteFromSeed(seed: string): { background: string; foreground: string } {
        const palettes: { background: string; foreground: string }[] = [
            { background: "#DCFCE7", foreground: "#065F46" },
            { background: "#DBEAFE", foreground: "#1E3A8A" },
            { background: "#FCE7F3", foreground: "#9D174D" },
            { background: "#FEF3C7", foreground: "#92400E" },
            { background: "#E0E7FF", foreground: "#3730A3" }
        ];
        let hash = 0;
        for (let i = 0; i < seed.length; i++) {
            hash = (hash << 5) - hash + seed.charCodeAt(i);
            hash |= 0;
        }
        const index = Math.abs(hash) % palettes.length;
        return palettes[index];
    }

    private escapeText(value: string): string {
        return value.replace(/[&<>"']/g, (ch) => {
            switch (ch) {
                case "&":
                    return "&amp;";
                case "<":
                    return "&lt;";
                case ">":
                    return "&gt;";
                case '"':
                    return "&quot;";
                case "'":
                    return "&#39;";
                default:
                    return ch;
            }
        });
    }

    private escapeAttr(value: string): string {
        return this.escapeText(value);
    }
}
