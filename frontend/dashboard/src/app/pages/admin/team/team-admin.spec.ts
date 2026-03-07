import { ComponentFixture, TestBed } from "@angular/core/testing";
import { signal } from "@angular/core";
import { provideHttpClient } from "@angular/common/http";
import { provideHttpClientTesting } from "@angular/common/http/testing";
import { provideRouter } from "@angular/router";
import { TranslocoTestingModule } from "@jsverse/transloco";
import { provideTranslocoLocale } from "@jsverse/transloco-locale";
import { of } from "rxjs";
import { Team, TeamAuditListResponse, TeamInvite, TeamMember } from "@models/analytics.types";
import { TeamService } from "@services/team.service";
import { TeamAdminPage } from "./team-admin";

describe("TeamAdminPage", () => {
    let component: TeamAdminPage;
    let fixture: ComponentFixture<TeamAdminPage>;
    const activeTeam = signal<Team>({
        id: "team-1",
        name: "Acme",
        logo_url: "",
        role: "owner",
        created_at: "2026-01-01T00:00:00Z"
    });
    const teams = signal<Team[]>([activeTeam()]);
    const members: TeamMember[] = [
        {
            id: "membership-1",
            user_id: "user-1",
            email: "owner@example.com",
            role: "owner",
            added_at: "2026-01-01T00:00:00Z"
        }
    ];
    const invites: TeamInvite[] = [];
    const auditResponse: TeamAuditListResponse = {
        entries: [],
        total: 0,
        limit: 25,
        offset: 0,
        has_more: false
    };
    const teamServiceMock = {
        activeTeam,
        teams,
        listTeamMembers: () => of(members),
        listTeamInvites: () => of(invites),
        listTeamAudit: () => of(auditResponse),
        updateTeam: () => of({ status: "ok" }),
        leaveTeam: () => of({ status: "ok", active_team_id: "" }),
        archiveTeam: () => of({ status: "ok", active_team_id: "" }),
        loadTeams: () => {
            teams.set([activeTeam()]);
            return of({ teams: teams(), active_team_id: activeTeam().id });
        }
    };

    beforeEach(async () => {
        await TestBed.configureTestingModule({
            imports: [
                TeamAdminPage,
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
                provideHttpClient(),
                provideHttpClientTesting(),
                provideRouter([]),
                provideTranslocoLocale({
                    langToLocaleMapping: {
                        en: "en-US"
                    }
                }),
                {
                    provide: TeamService,
                    useValue: teamServiceMock
                }
            ]
        }).compileComponents();

        fixture = TestBed.createComponent(TeamAdminPage);
        component = fixture.componentInstance;
        fixture.detectChanges();
    });

    it("should create", () => {
        expect(component).toBeTruthy();
    });

    it("should show the activity tab for team admins and owners", () => {
        expect(fixture.nativeElement.textContent).toContain("admin.team.tabs.activity");
    });

    it("should hide the activity tab for non-managers", () => {
        activeTeam.set({
            id: "team-1",
            name: "Acme",
            logo_url: "",
            role: "member",
            created_at: "2026-01-01T00:00:00Z"
        });

        fixture.detectChanges();

        expect(fixture.nativeElement.textContent).not.toContain("admin.team.tabs.activity");
    });

    it("should reset the active tab when audit access is lost", () => {
        component["activeTab"].set("3");
        activeTeam.set({
            id: "team-1",
            name: "Acme",
            logo_url: "",
            role: "member",
            created_at: "2026-01-01T00:00:00Z"
        });

        fixture.detectChanges();

        expect(component["activeTab"]()).toBe("0");
    });
});
