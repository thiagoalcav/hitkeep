import { ComponentFixture, TestBed } from "@angular/core/testing";
import { MainLayout } from "@layout/main-layout";
import { provideRouter } from "@angular/router";
import { By } from "@angular/platform-browser";
import { provideHttpClient } from "@angular/common/http";
import { HttpTestingController, provideHttpClientTesting } from "@angular/common/http/testing";
import { TranslocoTestingModule } from "@jsverse/transloco";
import { TeamService } from "@services/team.service";
import { vi } from "vitest";

describe("MainLayout", () => {
    let component: MainLayout;
    let fixture: ComponentFixture<MainLayout>;
    let httpMock: HttpTestingController;

    beforeEach(async () => {
        await TestBed.configureTestingModule({
            imports: [
                MainLayout,
                TranslocoTestingModule.forRoot({
                    langs: { en: {} },
                    translocoConfig: {
                        availableLangs: ["en"],
                        defaultLang: "en"
                    },
                    preloadLangs: true
                })
            ],
            providers: [provideRouter([]), provideHttpClient(), provideHttpClientTesting()]
        }).compileComponents();

        fixture = TestBed.createComponent(MainLayout);
        component = fixture.componentInstance;
        httpMock = TestBed.inject(HttpTestingController);
        fixture.detectChanges();
        flushBootstrapRequests();
        fixture.detectChanges();
    });

    afterEach(() => {
        httpMock.verify();
        vi.restoreAllMocks();
    });

    it("should create", () => {
        expect(component).toBeTruthy();
    });

    it("A11Y: should have correct landmarks", () => {
        const aside = fixture.debugElement.query(By.css("aside"));
        const main = fixture.debugElement.query(By.css("main"));
        const nav = fixture.debugElement.query(By.css("nav"));

        expect(aside).toBeTruthy();
        expect(main).toBeTruthy();
        expect(nav).toBeTruthy();

        // Check labels
        expect(aside.attributes["aria-label"]).toBeTruthy();
        expect(main.attributes["role"]).toBe("main");
    });

    it("A11Y: buttons should have accessible labels", () => {
        const buttons = fixture.debugElement.queryAll(By.css("button"));
        const buttonsWithAria = buttons.filter((btn) => !!btn.attributes["aria-label"]);
        expect(buttonsWithAria.length).toBeGreaterThan(0);
    });

    it("should always render team switcher", () => {
        const switchers = fixture.debugElement.queryAll(By.css("app-team-switcher"));
        expect(switchers.length).toBeGreaterThan(0);
    });

    it("should show administration section for team owner/admin role", () => {
        const adminLinks = Array.from(fixture.nativeElement.querySelectorAll("nav a")) as HTMLElement[];
        const hasTeamLink = adminLinks.some((link: HTMLElement) => link.getAttribute("href") === "/admin/team");
        expect(hasTeamLink).toBeTruthy();
    });

    it("should hide administration section for team member role", () => {
        const teamService = TestBed.inject(TeamService);
        teamService.teams.set([{ id: "00000000-0000-0000-0000-000000000001", name: "Alpha Team", logo_url: "", role: "member", created_at: "2026-01-01T00:00:00Z" }]);
        teamService.activeTeamId.set("00000000-0000-0000-0000-000000000001");
        fixture.detectChanges();
        const adminLinks = Array.from(fixture.nativeElement.querySelectorAll("nav a")) as HTMLElement[];
        const hasTeamLink = adminLinks.some((link: HTMLElement) => link.getAttribute("href") === "/admin/team");
        expect(hasTeamLink).toBeFalsy();
    });

    it("should allow team switch without confirmation when settings drawer is closed", () => {
        const confirmSpy = vi.spyOn(window, "confirm");
        const result = (component as any).beforeTeamSwitch();
        expect(result).toBe(true);
        expect(confirmSpy).not.toHaveBeenCalled();
    });

    it("should block team switch when settings drawer is open and user cancels", () => {
        const confirmSpy = vi.spyOn(window, "confirm").mockReturnValue(false);
        (component as any).isSiteSettingsVisible.set(true);
        const result = (component as any).beforeTeamSwitch();
        expect(result).toBe(false);
        expect(confirmSpy).toHaveBeenCalled();
        expect((component as any).isSiteSettingsVisible()).toBe(true);
    });

    it("should close settings drawer when switch is confirmed", () => {
        const confirmSpy = vi.spyOn(window, "confirm").mockReturnValue(true);
        (component as any).isSiteSettingsVisible.set(true);
        const result = (component as any).beforeTeamSwitch();
        expect(result).toBe(true);
        expect(confirmSpy).toHaveBeenCalled();
        expect((component as any).isSiteSettingsVisible()).toBe(false);
    });

    function flushBootstrapRequests() {
        httpMock.expectOne("/api/user/teams").flush({
            active_team_id: "00000000-0000-0000-0000-000000000001",
            teams: [
                {
                    id: "00000000-0000-0000-0000-000000000001",
                    name: "Alpha Team",
                    logo_url: "",
                    role: "owner",
                    created_at: "2026-01-01T00:00:00Z"
                },
                {
                    id: "00000000-0000-0000-0000-000000000002",

                    name: "Beta Team",
                    logo_url: "",
                    role: "admin",
                    created_at: "2026-01-02T00:00:00Z"
                }
            ]
        });

        httpMock.expectOne("/api/sites").flush([]);
        httpMock.expectOne("/api/user/permissions").flush({
            instance_role: "owner",
            permissions: {}
        });
        httpMock.expectOne("/api/user/profile").flush({
            id: "00000000-0000-0000-0000-0000000000aa",
            email: "owner@example.com",
            display_name: "Owner",
            avatar_url: ""
        });
        httpMock.expectOne("/api/user/preferences").flush({
            default_locale: "en"
        });
    }
});
