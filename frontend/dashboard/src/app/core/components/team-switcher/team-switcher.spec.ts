import { ComponentFixture, TestBed } from "@angular/core/testing";
import { provideHttpClient } from "@angular/common/http";
import { TranslocoTestingModule } from "@jsverse/transloco";
import { vi } from "vitest";
import { Team } from "@models/analytics.types";
import { TeamSwitcher } from "./team-switcher";

interface TeamSwitcherTestAccess {
    onTeamChange(team: Team | null): Promise<void>;
    onCreateTeam(): void;
    teamLogoUrl(team: Team): string;
}

describe("TeamSwitcher", () => {
    let component: TeamSwitcher;
    let fixture: ComponentFixture<TeamSwitcher>;

    const mockTeams: Team[] = [
        { id: "team-1", name: "Alpha Team", logo_url: "", role: "owner", created_at: "2026-01-01T00:00:00Z" },
        { id: "team-2", name: "Bravo Team", logo_url: "", role: "member", created_at: "2026-01-02T00:00:00Z" },
        { id: "team-3", name: "Charlie Team", logo_url: "", role: "admin", created_at: "2026-01-03T00:00:00Z" }
    ];

    beforeEach(async () => {
        await TestBed.configureTestingModule({
            imports: [
                TeamSwitcher,
                TranslocoTestingModule.forRoot({
                    langs: {
                        en: {
                            teams: {
                                switcher: {
                                    label: "Teams",
                                    regionAria: "Team switcher",
                                    selectAria: "Select team",
                                    placeholder: "Select a team",
                                    currentTeamAria: "Current team {{ name }}",
                                    addTeamAria: "Create a new team",
                                    openMenuAria: "Open team menu for {{ name }}",
                                    menuAria: "Team menu",
                                    currentLabel: "Current team"
                                },
                                createDialog: {
                                    createAction: "Create team"
                                },
                                roles: {
                                    owner: "Owner",
                                    admin: "Admin",
                                    member: "Member"
                                }
                            }
                        }
                    },
                    translocoConfig: {
                        availableLangs: ["en"],
                        defaultLang: "en"
                    },
                    preloadLangs: true
                })
            ],
            providers: [provideHttpClient()]
        }).compileComponents();

        fixture = TestBed.createComponent(TeamSwitcher);
        component = fixture.componentInstance;
        fixture.componentRef.setInput("teams", mockTeams);
        fixture.componentRef.setInput("currentTeamId", "team-1");
        fixture.detectChanges();
    });

    afterEach(() => {
        vi.restoreAllMocks();
    });

    it("should create", () => {
        expect(component).toBeTruthy();
    });

    it("should render the team select trigger", () => {
        const trigger = fixture.nativeElement.querySelector("[data-testid='team-switcher-trigger']");
        expect(trigger).toBeTruthy();
        expect(trigger.textContent).toContain("Alpha Team");
    });

    it("should not render a duplicated current team header", () => {
        expect(fixture.nativeElement.textContent).not.toContain("Current team");
    });

    it("should still render a trigger when only one team exists", () => {
        fixture.componentRef.setInput("teams", [mockTeams[0]]);
        fixture.detectChanges();

        const trigger = fixture.nativeElement.querySelector("[data-testid='team-switcher-trigger']");
        expect(trigger).toBeTruthy();
    });

    it("should emit addClicked from the create action", () => {
        const emitSpy = vi.spyOn(component.addClicked, "emit");
        fixture.componentRef.setInput("showAdd", true);
        fixture.detectChanges();
        const access = component as unknown as TeamSwitcherTestAccess;
        access.onCreateTeam();

        expect(emitSpy).toHaveBeenCalled();
    });

    it("should emit teamSelected when switching is allowed", async () => {
        const emitSpy = vi.spyOn(component.teamSelected, "emit");
        const access = component as unknown as TeamSwitcherTestAccess;

        await access.onTeamChange(mockTeams[1]);

        expect(emitSpy).toHaveBeenCalledWith(mockTeams[1]);
    });

    it("should not emit teamSelected when beforeSwitch rejects", async () => {
        const emitSpy = vi.spyOn(component.teamSelected, "emit");
        const access = component as unknown as TeamSwitcherTestAccess;
        fixture.componentRef.setInput("beforeSwitch", () => false);
        fixture.detectChanges();

        await access.onTeamChange(mockTeams[1]);

        expect(emitSpy).not.toHaveBeenCalled();
    });

    it("should generate local SVG avatar when logo is missing", () => {
        const access = component as unknown as TeamSwitcherTestAccess;
        const url = access.teamLogoUrl(mockTeams[0]);
        expect(url.startsWith("data:image/svg+xml;utf8,")).toBe(true);
    });
});
