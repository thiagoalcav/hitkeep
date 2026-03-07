import { ComponentFixture, TestBed } from "@angular/core/testing";
import { signal } from "@angular/core";
import { provideHttpClient } from "@angular/common/http";
import { HttpTestingController, provideHttpClientTesting } from "@angular/common/http/testing";
import { TranslocoTestingModule } from "@jsverse/transloco";
import { provideTranslocoLocale } from "@jsverse/transloco-locale";
import { vi } from "vitest";
import { SiteService } from "@features/sites/services/site.service";
import { TeamService } from "@services/team.service";
import { SiteTeamSettings } from "./site-team-settings";

interface SiteTeamSettingsTestAccess {
    availableTransferTeams(): { label: string; value: string }[];
    transferSite(): void;
    transferForm: {
        teamId(): {
            control(): {
                setValue(value: string): void;
            };
        };
    };
    transferSuccessKey(): string | null;
}

describe("SiteTeamSettings", () => {
    let fixture: ComponentFixture<SiteTeamSettings>;
    let component: SiteTeamSettings;
    let httpMock: HttpTestingController;

    const currentSite = {
        id: "site-1",
        user_id: "user-1",
        domain: "example.com",
        created_at: "2026-01-01T00:00:00Z"
    };

    const siteServiceMock = {
        sites: signal([currentSite]),
        activeSite: signal(currentSite),
        loadSites: vi.fn()
    };

    const teamServiceMock = {
        activeTeamId: signal("team-1"),
        teams: signal([
            {
                id: "team-1",
                name: "Current team",
                logo_url: "",
                role: "owner" as const,
                created_at: "2026-01-01T00:00:00Z"
            },
            {
                id: "team-2",
                name: "Destination team",
                logo_url: "",
                role: "admin" as const,
                created_at: "2026-01-02T00:00:00Z"
            },
            {
                id: "team-3",
                name: "Viewer team",
                logo_url: "",
                role: "member" as const,
                created_at: "2026-01-03T00:00:00Z"
            }
        ])
    };

    beforeEach(async () => {
        await TestBed.configureTestingModule({
            imports: [
                SiteTeamSettings,
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
                { provide: TeamService, useValue: teamServiceMock },
                { provide: SiteService, useValue: siteServiceMock },
                provideTranslocoLocale({
                    langToLocaleMapping: {
                        en: "en-US"
                    }
                })
            ]
        }).compileComponents();

        fixture = TestBed.createComponent(SiteTeamSettings);
        component = fixture.componentInstance;
        fixture.componentRef.setInput("site", currentSite);
        fixture.detectChanges();

        httpMock = TestBed.inject(HttpTestingController);
        httpMock.expectOne("/api/sites/site-1/members").flush([]);
    });

    afterEach(() => {
        vi.restoreAllMocks();
        httpMock.verify();
    });

    it("filters transfer targets to teams the user can manage", () => {
        const access = component as unknown as SiteTeamSettingsTestAccess;
        expect(access.availableTransferTeams()).toEqual([
            {
                label: "Destination team",
                value: "team-2"
            }
        ]);
    });

    it("transfers the site and refreshes scoped site state", () => {
        const access = component as unknown as SiteTeamSettingsTestAccess;
        access.transferForm.teamId().control().setValue("team-2");

        access.transferSite();

        const request = httpMock.expectOne("/api/sites/site-1/transfer-team");
        expect(request.request.method).toBe("POST");
        expect(request.request.body).toEqual({ team_id: "team-2" });
        request.flush({
            status: "ok",
            site_id: "site-1",
            source_team_id: "team-1",
            destination_team_id: "team-2"
        });

        expect(siteServiceMock.sites()).toEqual([]);
        expect(siteServiceMock.activeSite()).toBeNull();
        expect(siteServiceMock.loadSites).toHaveBeenCalled();
        expect(access.transferSuccessKey()).toBe("sites.team.transfer.success");
    });
});
