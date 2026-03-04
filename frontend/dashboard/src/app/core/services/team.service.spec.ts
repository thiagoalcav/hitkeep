import { TestBed } from "@angular/core/testing";
import { HttpTestingController, provideHttpClientTesting } from "@angular/common/http/testing";
import { provideHttpClient } from "@angular/common/http";
import { TeamService } from "@services/team.service";

describe("TeamService", () => {
    let service: TeamService;
    let httpMock: HttpTestingController;

    beforeEach(() => {
        TestBed.configureTestingModule({
            providers: [TeamService, provideHttpClient(), provideHttpClientTesting()]
        });

        service = TestBed.inject(TeamService);
        httpMock = TestBed.inject(HttpTestingController);
    });

    afterEach(() => {
        httpMock.verify();
    });

    it("should be created", () => {
        expect(service).toBeTruthy();
    });

    it("should load teams and set active team", () => {
        service.loadTeams().subscribe();

        const req = httpMock.expectOne("/api/user/teams");
        expect(req.request.method).toBe("GET");
        req.flush({
            active_team_id: "00000000-0000-0000-0000-000000000002",
            teams: [
                {
                    id: "00000000-0000-0000-0000-000000000001",
                    name: "Alpha",
                    logo_url: "",
                    role: "owner",
                    created_at: "2026-01-01T00:00:00Z"
                },
                {
                    id: "00000000-0000-0000-0000-000000000002",
                    name: "Bravo",
                    logo_url: "",
                    role: "admin",
                    created_at: "2026-01-02T00:00:00Z"
                }
            ]
        });

        expect(service.teams().length).toBe(2);
        expect(service.activeTeamId()).toBe("00000000-0000-0000-0000-000000000002");
        expect(service.activeTeam()?.name).toBe("Bravo");
        expect(service.hasMultipleTeams()).toBe(true);
    });

    it("should switch active team", () => {
        service.teams.set([
            { id: "00000000-0000-0000-0000-000000000001", name: "Alpha", logo_url: "", role: "owner", created_at: "2026-01-01T00:00:00Z" },
            { id: "00000000-0000-0000-0000-000000000002", name: "Bravo", logo_url: "", role: "admin", created_at: "2026-01-02T00:00:00Z" }
        ]);
        service.activeTeamId.set("00000000-0000-0000-0000-000000000001");

        service.setActiveTeam("00000000-0000-0000-0000-000000000002").subscribe();

        const req = httpMock.expectOne("/api/user/teams/active");
        expect(req.request.method).toBe("PUT");
        expect(req.request.body).toEqual({ team_id: "00000000-0000-0000-0000-000000000002" });
        req.flush({
            status: "ok",
            active_team_id: "00000000-0000-0000-0000-000000000002"
        });

        expect(service.activeTeamId()).toBe("00000000-0000-0000-0000-000000000002");
    });

    it("should skip request when selecting current team", () => {
        service.activeTeamId.set("00000000-0000-0000-0000-000000000001");

        service.setActiveTeam("00000000-0000-0000-0000-000000000001").subscribe((response) => {
            expect(response.active_team_id).toBe("00000000-0000-0000-0000-000000000001");
        });

        httpMock.expectNone("/api/user/teams/active");
    });

    it("should list team members", () => {
        service.listTeamMembers("team-id").subscribe((members) => {
            expect(members.length).toBe(1);
            expect(members[0].email).toBe("member@example.com");
        });

        const req = httpMock.expectOne("/api/user/teams/team-id/members");
        expect(req.request.method).toBe("GET");
        req.flush([
            {
                id: "member-row-id",
                user_id: "user-id",
                email: "member@example.com",
                role: "member",
                added_at: "2026-01-03T00:00:00Z"
            }
        ]);
    });

    it("should upsert team member", () => {
        service
            .upsertTeamMember("team-id", {
                email: "new@example.com",
                role: "admin"
            })
            .subscribe((response) => {
                expect(response.status).toBe("ok");
            });

        const req = httpMock.expectOne("/api/user/teams/team-id/members");
        expect(req.request.method).toBe("POST");
        expect(req.request.body).toEqual({
            email: "new@example.com",
            role: "admin"
        });
        req.flush({
            status: "ok",
            is_invite: true
        });
    });

    it("should remove team member", () => {
        service.removeTeamMember("team-id", "user-id").subscribe((response) => {
            expect(response.status).toBe("ok");
        });

        const req = httpMock.expectOne("/api/user/teams/team-id/members/user-id");
        expect(req.request.method).toBe("DELETE");
        req.flush({
            status: "ok"
        });
    });

    it("should list team audit entries", () => {
        service.listTeamAudit("team-id").subscribe((entries) => {
            expect(entries.length).toBe(1);
            expect(entries[0].action).toBe("member.role_updated");
        });

        const req = httpMock.expectOne("/api/user/teams/team-id/audit");
        expect(req.request.method).toBe("GET");
        req.flush([
            {
                id: "audit-1",
                team_id: "team-id",
                action: "member.role_updated",
                details: "Role changed from member to admin",
                actor_email: "owner@example.com",
                target_email: "member@example.com",
                created_at: "2026-01-04T00:00:00Z"
            }
        ]);
    });

    it("should leave team and update local state", () => {
        service.teams.set([
            { id: "team-a", name: "Team A", logo_url: "", role: "owner", created_at: "2026-01-01T00:00:00Z" },
            { id: "team-b", name: "Team B", logo_url: "", role: "admin", created_at: "2026-01-02T00:00:00Z" }
        ]);
        service.activeTeamId.set("team-b");

        service.leaveTeam("team-b").subscribe((response) => {
            expect(response.status).toBe("ok");
        });

        const req = httpMock.expectOne("/api/user/teams/team-b/leave");
        expect(req.request.method).toBe("DELETE");
        req.flush({
            status: "ok",
            active_team_id: "team-a"
        });

        expect(service.activeTeamId()).toBe("team-a");
        expect(service.teams().map((team) => team.id)).toEqual(["team-a"]);
    });
});
