import { ComponentFixture, TestBed } from "@angular/core/testing";
import { signal } from "@angular/core";
import { provideRouter, Router } from "@angular/router";
import { of } from "rxjs";
import { TranslocoTestingModule } from "@jsverse/transloco";
import { TeamSettingsPage } from "./team-settings";
import { TeamService } from "@services/team.service";
import { SiteService } from "@features/sites/services/site.service";
import { PermissionService } from "@services/permission.service";
import { vi } from "vitest";

interface TeamSettingsTestAccess {
    leaveTeam(): void;
    archiveTeam(): void;
}

interface MockWithCalls {
    mock: {
        calls: unknown[][];
    };
}

describe("TeamSettingsPage", () => {
    let fixture: ComponentFixture<TeamSettingsPage>;
    let component: TeamSettingsPage;

    const activeTeam = signal({
        id: "team-1",
        name: "Acme",
        logo_url: "",
        role: "owner" as const,
        created_at: "2026-01-01T00:00:00Z"
    });

    const teamServiceMock = {
        activeTeam,
        updateTeam: vi.fn(() => of({ status: "ok" })),
        leaveTeam: vi.fn(() => of({ status: "ok", active_team_id: "team-2" })),
        archiveTeam: vi.fn(() => of({ status: "ok", active_team_id: "team-2" })),
        loadTeams: vi.fn(() => of({ active_team_id: "team-2", teams: [] }))
    };

    const siteServiceMock = {
        sites: signal([]),
        activeSite: signal(null),
        loadSites: vi.fn()
    };

    const permissionServiceMock = {
        loadPermissions: vi.fn(() => of({}))
    };

    beforeEach(async () => {
        await TestBed.configureTestingModule({
            imports: [
                TeamSettingsPage,
                TranslocoTestingModule.forRoot({
                    langs: { en: {} },
                    translocoConfig: {
                        availableLangs: ["en"],
                        defaultLang: "en"
                    },
                    preloadLangs: true
                })
            ],
            providers: [provideRouter([]), { provide: TeamService, useValue: teamServiceMock }, { provide: SiteService, useValue: siteServiceMock }, { provide: PermissionService, useValue: permissionServiceMock }]
        }).compileComponents();

        fixture = TestBed.createComponent(TeamSettingsPage);
        component = fixture.componentInstance;
        fixture.detectChanges();
    });

    afterEach(() => {
        vi.restoreAllMocks();
    });

    it("should call leaveTeam for current team", () => {
        const router = TestBed.inject(Router);
        const navigateSpy = vi.spyOn(router, "navigateByUrl").mockResolvedValue(true);
        const access = component as unknown as TeamSettingsTestAccess;

        access.leaveTeam();

        expect(teamServiceMock.leaveTeam).toHaveBeenCalled();
        expect((teamServiceMock.leaveTeam as unknown as MockWithCalls).mock.calls[0][0]).toBe("team-1");
        expect(permissionServiceMock.loadPermissions).toHaveBeenCalled();
        expect(navigateSpy).toHaveBeenCalledWith("/dashboard");
    });

    it("should call archiveTeam for current team", () => {
        const router = TestBed.inject(Router);
        const navigateSpy = vi.spyOn(router, "navigateByUrl").mockResolvedValue(true);
        const access = component as unknown as TeamSettingsTestAccess;

        access.archiveTeam();

        expect(teamServiceMock.archiveTeam).toHaveBeenCalled();
        expect((teamServiceMock.archiveTeam as unknown as MockWithCalls).mock.calls[0][0]).toBe("team-1");
        expect(permissionServiceMock.loadPermissions).toHaveBeenCalled();
        expect(navigateSpy).toHaveBeenCalledWith("/dashboard");
    });
});
