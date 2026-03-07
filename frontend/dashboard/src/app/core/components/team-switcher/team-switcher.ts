import { ChangeDetectionStrategy, Component, computed, effect, inject, input, output, signal } from "@angular/core";
import { FormControl, ReactiveFormsModule } from "@angular/forms";
import { compatForm } from "@angular/forms/signals/compat";
import { TranslocoPipe, TranslocoService } from "@jsverse/transloco";
import { SelectModule } from "primeng/select";
import { SkeletonModule } from "primeng/skeleton";
import { TagModule } from "primeng/tag";
import { Team, TeamRole } from "@models/analytics.types";

@Component({
    selector: "app-team-switcher",
    imports: [ReactiveFormsModule, SelectModule, SkeletonModule, TagModule, TranslocoPipe],
    changeDetection: ChangeDetectionStrategy.OnPush,
    template: `
        <div
            [class]="
                compact()
                    ? 'flex w-full flex-col gap-2 rounded-2xl border border-surface-200/80 bg-linear-to-b from-white to-surface-50 p-2 shadow-sm dark:border-surface-700/80 dark:from-surface-900 dark:to-surface-800'
                    : 'flex w-full flex-col gap-2'
            "
            role="region"
            [attr.aria-label]="'teams.switcher.regionAria' | transloco"
        >
            @if (showBrand()) {
                <div class="mb-1 flex select-none items-center gap-3 px-2">
                    <div class="flex size-9 items-center justify-center rounded-2xl bg-linear-to-br from-primary-500 to-emerald-500 shadow-sm">
                        <img src="/icon.png" alt="HitKeep Logo" class="h-6 w-6 object-cover" />
                    </div>
                    <div class="min-w-0">
                        <div class="truncate text-sm font-semibold uppercase tracking-[0.24em] text-muted-color">Workspace</div>
                        <span class="block truncate text-xl font-bold tracking-tight text-[var(--p-text-color)]">HitKeep</span>
                    </div>
                </div>
            }

            <div class="flex items-center" [class.justify-between]="!compact()" [class.justify-end]="compact()">
                @if (!compact()) {
                    <label [for]="selectId" class="flex items-center gap-2 text-xs font-semibold uppercase tracking-[0.18em] text-[var(--p-text-muted-color)]">
                        <span class="inline-block size-2 rounded-full bg-emerald-500"></span>
                        {{ "teams.switcher.label" | transloco }}
                    </label>
                }
                @if (showAdd()) {
                    <button
                        type="button"
                        (click)="addClicked.emit()"
                        class="flex h-8 min-w-8 cursor-pointer items-center justify-center rounded-xl border border-surface-200 bg-white/80 px-2 text-muted-color shadow-sm transition-colors hover:bg-surface-100 hover:text-[var(--p-text-color)] focus:outline-none focus:ring-2 focus:ring-primary-500 dark:border-surface-700 dark:bg-surface-900/80 dark:hover:bg-surface-800"
                        [attr.aria-label]="'teams.switcher.addTeamAria' | transloco"
                    >
                        <i class="pi pi-plus text-xs" aria-hidden="true"></i>
                    </button>
                }
            </div>

            @if (loading()) {
                <p-skeleton height="40px" class="rounded-md" />
            } @else if (isSingleTeam()) {
                <div
                    [class]="
                        compact()
                            ? 'flex min-w-0 items-center gap-3 rounded-2xl border border-surface-200/80 bg-white/70 px-3 py-2 shadow-sm dark:border-surface-700/80 dark:bg-surface-900/70'
                            : 'flex min-w-0 items-center gap-3 rounded-2xl border border-surface-200/80 bg-[var(--p-surface-ground)] px-3 py-2 shadow-sm dark:border-surface-700/80'
                    "
                    [attr.aria-label]="'teams.switcher.currentTeamAria' | transloco: { name: activeTeam().name }"
                >
                    <img [src]="teamLogoUrl(activeTeam())" [alt]="activeTeam().name" class="size-9 max-w-9 rounded-2xl object-cover shadow-sm" />
                    <div class="min-w-0">
                        <div class="truncate text-sm font-semibold">{{ activeTeam().name }}</div>
                        <div class="text-xs uppercase tracking-[0.18em] text-muted-color">{{ roleLabel(activeTeam().role) }}</div>
                    </div>
                </div>
            } @else {
                <p-select
                    [inputId]="selectId"
                    [options]="teams()"
                    [formControl]="teamForm.selectedTeam().control()"
                    [filter]="true"
                    filterBy="name"
                    dataKey="id"
                    optionLabel="name"
                    [loading]="switching()"
                    [placeholder]="'teams.switcher.placeholder' | transloco"
                    class="w-full text-sm"
                    [fluid]="true"
                    [attr.aria-label]="'teams.switcher.selectAria' | transloco"
                    (onChange)="onTeamChange($event.value)"
                >
                    <ng-template pTemplate="selectedItem" let-selected>
                        @if (selected) {
                            <div class="flex min-w-0 grow items-center gap-3 py-0.5">
                                <img [src]="teamLogoUrl(selected)" [alt]="selected.name" class="size-9 max-w-9 rounded-2xl object-cover shadow-sm" />
                                <div class="min-w-0">
                                    <div class="truncate text-sm font-semibold">{{ selected.name }}</div>
                                    <div class="text-xs uppercase tracking-[0.18em] text-muted-color">{{ roleLabel(selected.role) }}</div>
                                </div>
                            </div>
                        }
                    </ng-template>

                    <ng-template pTemplate="item" let-team>
                        <div class="flex w-full min-w-0 items-center justify-between gap-3 rounded-xl px-1 py-1">
                            <div class="flex min-w-0 items-center gap-3">
                                <img [src]="teamLogoUrl(team)" [alt]="team.name" class="size-9 max-w-9 shrink-0 rounded-2xl object-cover shadow-sm" />
                                <div class="min-w-0">
                                    <div class="truncate font-semibold">{{ team.name }}</div>
                                    <div class="text-xs uppercase tracking-[0.18em] text-muted-color">{{ roleLabel(team.role) }}</div>
                                </div>
                            </div>
                            <p-tag [value]="roleLabel(team.role)" [severity]="roleSeverity(team.role)" [rounded]="true" />
                        </div>
                    </ng-template>
                </p-select>
            }
        </div>
    `
})
export class TeamSwitcher {
    private static nextId = 0;

    private transloco = inject(TranslocoService);
    protected readonly selectId = `team-dropdown-${TeamSwitcher.nextId++}`;

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

    protected readonly isSingleTeam = computed(() => !this.loading() && this.teams().length <= 1);
    protected readonly activeTeam = computed(() => this.teams().find((t) => t.id === this.currentTeamId()) ?? this.teams()[0] ?? null);

    private readonly teamFormModel = signal({
        selectedTeam: new FormControl<Team | null>(null)
    });
    protected readonly teamForm = compatForm(this.teamFormModel);

    constructor() {
        effect(() => {
            if (this.switching()) {
                return;
            }
            const currentTeam = this.teams().find((team) => team.id === this.currentTeamId()) ?? this.teams()[0] ?? null;
            this.teamForm.selectedTeam().control().setValue(currentTeam, { emitEvent: false });
        });
    }

    protected async onTeamChange(team: Team | null) {
        if (!team || team.id === this.currentTeamId()) {
            return;
        }

        const proceed = await this.canSwitchTeam(team);
        if (!proceed) {
            const currentTeam = this.activeTeam();
            this.teamForm.selectedTeam().control().setValue(currentTeam, { emitEvent: false });
            return;
        }
        this.teamSelected.emit(team);
    }

    private async canSwitchTeam(nextTeam: Team): Promise<boolean> {
        const checker = this.beforeSwitch();
        if (!checker) {
            return true;
        }
        const allowed = checker(nextTeam);
        return typeof (allowed as Promise<boolean>)?.then === "function" ? await allowed : Boolean(allowed);
    }

    protected teamLogoUrl(team: Team): string {
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
