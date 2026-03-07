import { ComponentFixture, TestBed } from "@angular/core/testing";
import { signal } from "@angular/core";
import { of } from "rxjs";
import { TranslocoTestingModule } from "@jsverse/transloco";
import { provideTranslocoLocale } from "@jsverse/transloco-locale";
import { vi } from "vitest";
import { TeamMembersPage } from "./team-members";
import { TeamService } from "@services/team.service";

interface TeamMembersTestAccess {
    members(): unknown[];
    pendingInvites(): unknown[];
}

describe("TeamMembersPage", () => {
    let fixture: ComponentFixture<TeamMembersPage>;
    let component: TeamMembersPage;

    const teamServiceMock = {
        activeTeam: signal({
            id: "team-1",
            name: "Acme",
            logo_url: "",
            role: "owner" as const,
            created_at: "2026-01-01T00:00:00Z"
        }),
        listTeamMembers: vi.fn((teamID: string) => {
            void teamID;
            return of([
                {
                    id: "member-row",
                    user_id: "user-1",
                    email: "owner@example.com",
                    role: "owner" as const,
                    added_at: "2026-01-01T00:00:00Z"
                }
            ]);
        }),
        listTeamInvites: vi.fn((teamID: string) => {
            void teamID;
            return of([
                {
                    id: "invite-1",
                    team_id: "team-1",
                    email: "invitee@example.com",
                    role: "admin" as const,
                    status: "pending" as const,
                    created_at: "2026-01-03T00:00:00Z",
                    expires_at: "2026-01-10T00:00:00Z"
                }
            ]);
        }),
        upsertTeamMember: vi.fn(() => of({ status: "ok", is_invite: true })),
        removeTeamMember: vi.fn(() => of({ status: "ok" })),
        resendTeamInvite: vi.fn(() => of({ status: "ok" })),
        revokeTeamInvite: vi.fn(() => of({ status: "ok" })),
        transferTeamOwnership: vi.fn(() => of({ status: "ok" })),
        loadTeams: vi.fn(() => of({ active_team_id: "team-1", teams: [] }))
    };

    beforeEach(async () => {
        await TestBed.configureTestingModule({
            imports: [
                TeamMembersPage,
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

        fixture = TestBed.createComponent(TeamMembersPage);
        component = fixture.componentInstance;
        fixture.detectChanges();
    });

    afterEach(() => {
        vi.restoreAllMocks();
    });

    it("loads members and pending invites for managers", () => {
        const access = component as unknown as TeamMembersTestAccess;
        expect(teamServiceMock.listTeamMembers).toHaveBeenCalledWith("team-1");
        expect(teamServiceMock.listTeamInvites).toHaveBeenCalledWith("team-1");
        expect(access.members().length).toBe(1);
        expect(access.pendingInvites().length).toBe(1);
    });
});
