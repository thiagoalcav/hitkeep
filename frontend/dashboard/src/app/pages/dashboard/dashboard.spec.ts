import { ComponentFixture, TestBed } from "@angular/core/testing";
import { provideHttpClient } from "@angular/common/http";
import { provideRouter } from "@angular/router";
import { TranslocoTestingModule } from "@jsverse/transloco";
import { provideTranslocoLocale } from "@jsverse/transloco-locale";

import { Dashboard } from "@pages/dashboard/dashboard";
import { SiteService } from "@features/sites/services/site.service";
import { TeamService } from "@services/team.service";

describe("Dashboard", () => {
    let component: Dashboard;
    let fixture: ComponentFixture<Dashboard>;

    beforeEach(async () => {
        await TestBed.configureTestingModule({
            imports: [
                Dashboard,
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
                provideRouter([]),
                provideTranslocoLocale({
                    defaultLocale: "en-US",
                    langToLocaleMapping: {
                        en: "en-US",
                        "en-US": "en-US"
                    }
                })
            ]
        }).compileComponents();

        fixture = TestBed.createComponent(Dashboard);
        component = fixture.componentInstance;
        fixture.detectChanges();
    });

    it("should create", () => {
        expect(component).toBeTruthy();
    });

    it("should show team onboarding copy when the active team has no sites", () => {
        const siteService = TestBed.inject(SiteService);
        const teamService = TestBed.inject(TeamService);

        siteService.sites.set([]);
        siteService.activeSite.set(null);
        teamService.teams.set([
            {
                id: "team-1",
                name: "Acme Growth",
                logo_url: "",
                role: "owner",
                created_at: "2026-01-01T00:00:00Z"
            }
        ]);
        teamService.activeTeamId.set("team-1");

        fixture.detectChanges();

        expect(fixture.nativeElement.textContent).toContain("dashboard.empty.teamTitle");
    });
});
