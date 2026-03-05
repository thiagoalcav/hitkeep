import { ComponentFixture, TestBed } from "@angular/core/testing";
import { signal } from "@angular/core";
import { of } from "rxjs";
import { TranslocoTestingModule } from "@jsverse/transloco";
import { provideTranslocoLocale } from "@jsverse/transloco-locale";
import { vi } from "vitest";
import { TeamAuditPage } from "./team-audit";
import { TeamService } from "@services/team.service";

describe("TeamAuditPage", () => {
    let fixture: ComponentFixture<TeamAuditPage>;
    const activeTeam = signal({
        id: "team-1",
        name: "Acme",
        logo_url: "",
        role: "owner" as const,
        created_at: "2026-01-01T00:00:00Z"
    });

    const teamServiceMock = {
        activeTeam,
        listTeamAudit: vi.fn(() =>
            of([
                {
                    id: "audit-1",
                    team_id: "team-1",
                    action: "member.added",
                    details: "Added user",
                    actor_email: "owner@example.com",
                    created_at: "2026-01-03T00:00:00Z"
                }
            ])
        )
    };

    beforeEach(async () => {
        await TestBed.configureTestingModule({
            imports: [
                TeamAuditPage,
                TranslocoTestingModule.forRoot({
                    langs: { en: {} },
                    translocoConfig: {
                        availableLangs: ["en"],
                        defaultLang: "en"
                    },
                    preloadLangs: true
                })
            ],
            providers: [
                { provide: TeamService, useValue: teamServiceMock },
                provideTranslocoLocale({
                    langToLocaleMapping: {
                        en: "en-US"
                    }
                })
            ]
        }).compileComponents();

        fixture = TestBed.createComponent(TeamAuditPage);
        fixture.detectChanges();
    });

    afterEach(() => {
        vi.restoreAllMocks();
    });

    it("should load team audit rows for owner/admin", () => {
        expect(teamServiceMock.listTeamAudit).toHaveBeenCalled();
        expect((teamServiceMock.listTeamAudit as any).mock.calls[0][0]).toBe("team-1");
    });
});
