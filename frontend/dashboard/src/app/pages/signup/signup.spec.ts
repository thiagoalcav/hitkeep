import { ComponentFixture, TestBed } from "@angular/core/testing";
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
                { provide: AnalyticsService, useValue: analyticsServiceMock }
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
        expect(component["jurisdictionLabel"]()).toBe("EU");
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
});
