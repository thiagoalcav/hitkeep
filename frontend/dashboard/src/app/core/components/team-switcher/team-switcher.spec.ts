import { ComponentFixture, TestBed } from "@angular/core/testing";
import { provideHttpClient } from "@angular/common/http";
import { TranslocoTestingModule } from "@jsverse/transloco";
import { vi } from "vitest";
import { TeamSwitcher } from "./team-switcher";
import { Team } from "@models/analytics.types";

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
                                    placeholder: "Select a team",
                                    regionAria: "Team switcher",
                                    selectAria: "Select team",
                                    currentTeamAria: "Current team {{ name }}",
                                    addTeamAria: "Add team"
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

    it("should render a select when multiple teams exist", () => {
        const select = fixture.nativeElement.querySelector("p-select");
        expect(select).toBeTruthy();
    });

    it("should render single-team view when only one team exists", () => {
        fixture.componentRef.setInput("teams", [mockTeams[0]]);
        fixture.detectChanges();

        const select = fixture.nativeElement.querySelector("p-select");
        expect(select).toBeFalsy();
    });

    it("should emit addClicked when add button is pressed", () => {
        const emitSpy = vi.spyOn(component.addClicked, "emit");
        fixture.componentRef.setInput("showAdd", true);
        fixture.detectChanges();

        const addButton = fixture.nativeElement.querySelector("button");
        addButton.click();

        expect(emitSpy).toHaveBeenCalled();
    });

    it("should emit teamSelected when switching is allowed", async () => {
        const emitSpy = vi.spyOn(component.teamSelected, "emit");

        await (component as any).onTeamChange(mockTeams[1]);

        expect(emitSpy).toHaveBeenCalledWith(mockTeams[1]);
    });

    it("should not emit teamSelected when beforeSwitch rejects", async () => {
        const emitSpy = vi.spyOn(component.teamSelected, "emit");
        fixture.componentRef.setInput("beforeSwitch", () => false);
        fixture.detectChanges();

        await (component as any).onTeamChange(mockTeams[1]);

        expect(emitSpy).not.toHaveBeenCalled();
    });

    it("should generate local SVG avatar when logo is missing", () => {
        const url = (component as any).teamLogoUrl(mockTeams[0]) as string;
        expect(url.startsWith("data:image/svg+xml;utf8,")).toBe(true);
    });
});
