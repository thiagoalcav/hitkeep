import { ComponentFixture, TestBed } from "@angular/core/testing";
import { ActivatedRoute, convertToParamMap } from "@angular/router";
import { Router } from "@angular/router";
import { TranslocoTestingModule } from "@jsverse/transloco";
import { NEVER, of } from "rxjs";
import { vi } from "vitest";

import { Signup } from "@pages/signup/signup";
import { AnalyticsService } from "@services/analytics.service";
import { CloudService } from "@services/cloud.service";

describe("Signup", () => {
    let component: Signup;
    let fixture: ComponentFixture<Signup>;
    let queryParams: Record<string, string>;

    const cloudServiceMock = {
        signup: vi.fn(() => NEVER)
    };

    const analyticsServiceMock = {
        getSystemStatus: vi.fn(() =>
            of({
                needs_setup: false,
                version: "v2.0.0",
                cloud: {
                    hosted: true,
                    signup_enabled: true,
                    jurisdiction: "EU"
                }
            })
        )
    };

    const routerMock = {
        navigateByUrl: vi.fn(() => Promise.resolve(true))
    };

    beforeEach(async () => {
        vi.clearAllMocks();
        routerMock.navigateByUrl.mockClear();
        queryParams = {};

        await TestBed.configureTestingModule({
            imports: [
                Signup,
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
                { provide: Router, useValue: routerMock },
                { provide: CloudService, useValue: cloudServiceMock },
                { provide: AnalyticsService, useValue: analyticsServiceMock },
                {
                    provide: ActivatedRoute,
                    useValue: {
                        snapshot: {
                            get queryParamMap() {
                                return convertToParamMap(queryParams);
                            }
                        }
                    }
                }
            ]
        })
            .overrideComponent(Signup, {
                set: {
                    imports: [],
                    template: "<div></div>"
                }
            })
            .compileComponents();

        fixture = TestBed.createComponent(Signup);
        component = fixture.componentInstance;
        fixture.detectChanges();
    });

    it("shows the hosted jurisdiction from system status", () => {
        expect(component["currentJurisdiction"]()).toBe("EU");
        expect(component["selectedJurisdiction"]()).toBe("EU");
    });

    it("submits a cloud signup request with the selected plan", () => {
        component["signupForm"].teamName().control().setValue("Cloud Team");
        component["signupForm"].email().control().setValue("user@example.com");
        component["signupForm"].password().control().setValue("password123");
        component["selectPlan"]("pro");

        component["onSubmit"]();

        const payload = ((cloudServiceMock.signup as unknown as { mock: { calls: unknown[][] } }).mock.calls[0]?.[0] ?? null) as Record<string, string> | null;
        expect(payload?.["team_name"]).toBe("Cloud Team");
        expect(payload?.["email"]).toBe("user@example.com");
        expect(payload?.["plan_code"]).toBe("pro");
        expect(payload?.["jurisdiction"]).toBe("EU");
        expect(payload?.["locale"]).toBe("en");
    });

    it("hydrates plan and profile fields from query params", async () => {
        queryParams = {
            plan: "business",
            jurisdiction: "US",
            given_name: "Ada",
            last_name: "Lovelace",
            team_name: "Analytical Engine",
            email: "ada@example.com"
        };

        fixture = TestBed.createComponent(Signup);
        component = fixture.componentInstance;
        fixture.detectChanges();

        expect(component["selectedPlan"]()).toBe("business");
        expect(component["selectedJurisdiction"]()).toBe("US");
        expect(component["signupForm"].givenName().value()).toBe("Ada");
        expect(component["signupForm"].lastName().value()).toBe("Lovelace");
        expect(component["signupForm"].teamName().value()).toBe("Analytical Engine");
        expect(component["signupForm"].email().value()).toBe("ada@example.com");
    });

    it("redirects to the other region instead of creating a local account", () => {
        const redirectSpy = vi.spyOn(component as unknown as Record<string, (...args: unknown[]) => unknown>, "redirectToExternal").mockImplementation(() => undefined);

        component["signupForm"].teamName().control().setValue("Cloud Team");
        component["signupForm"].email().control().setValue("user@example.com");
        component["signupForm"].password().control().setValue("password123");
        component["signupForm"].givenName().control().setValue("Ada");
        component["selectPlan"]("pro");
        component["selectJurisdiction"]("US");

        component["onSubmit"]();

        expect(cloudServiceMock.signup).not.toHaveBeenCalled();
        expect(redirectSpy).toHaveBeenCalledTimes(1);
        expect(redirectSpy.mock.calls[0]?.[0]).toContain("https://cloud.hitkeep.com/signup");
        expect(redirectSpy.mock.calls[0]?.[0]).toContain("plan=pro");
        expect(redirectSpy.mock.calls[0]?.[0]).toContain("team_name=Cloud+Team");
        expect(redirectSpy.mock.calls[0]?.[0]).toContain("email=user%40example.com");
    });
});
