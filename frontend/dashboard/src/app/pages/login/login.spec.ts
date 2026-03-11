import { ComponentFixture, TestBed } from "@angular/core/testing";
import { ActivatedRoute, convertToParamMap, provideRouter } from "@angular/router";
import { TranslocoTestingModule } from "@jsverse/transloco";
import { of } from "rxjs";
import { vi } from "vitest";

import { Login } from "@pages/login/login";
import { AnalyticsService } from "@services/analytics.service";
import { AuthService } from "@services/auth.service";
import { UserPreferencesService } from "@services/user-preferences.service";

describe("Login", () => {
    let component: Login;
    let fixture: ComponentFixture<Login>;
    let returnUrl: string | null;
    const authMock: {
        status: () => string;
        login: ReturnType<typeof vi.fn>;
        startPasskeyLogin: ReturnType<typeof vi.fn>;
        finishPasskeyLogin: ReturnType<typeof vi.fn>;
        verifyMfaTotp: ReturnType<typeof vi.fn>;
        verifyMfaRecoveryCode: ReturnType<typeof vi.fn>;
    } = {
        status: () => "unknown",
        login: vi.fn(() => of({ status: "ok" as const })),
        startPasskeyLogin: vi.fn(() =>
            of({
                challenge_token: "",
                publicKey: {
                    challenge: "",
                    rpId: "",
                    timeout: 0,
                    userVerification: "preferred" as UserVerificationRequirement
                }
            })
        ),
        finishPasskeyLogin: vi.fn(() => of(void 0)),
        verifyMfaTotp: vi.fn(() => of(void 0)),
        verifyMfaRecoveryCode: vi.fn(() => of(void 0))
    };

    beforeEach(async () => {
        returnUrl = null;
        vi.clearAllMocks();

        const preferencesMock = {
            load: () => of(void 0)
        } as unknown as UserPreferencesService;
        const analyticsMock = {
            getSystemStatus: () =>
                of({
                    needs_setup: false,
                    version: "v2.0.0",
                    cloud: {
                        hosted: true,
                        signup_enabled: true,
                        jurisdiction: "EU"
                    }
                })
        } as unknown as AnalyticsService;

        await TestBed.configureTestingModule({
            imports: [
                Login,
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
                provideRouter([]),
                { provide: AuthService, useValue: authMock as unknown as AuthService },
                { provide: AnalyticsService, useValue: analyticsMock },
                { provide: UserPreferencesService, useValue: preferencesMock },
                {
                    provide: ActivatedRoute,
                    useValue: {
                        snapshot: {
                            get queryParamMap() {
                                return convertToParamMap(returnUrl ? { returnUrl } : {});
                            }
                        }
                    }
                }
            ]
        }).compileComponents();

        fixture = TestBed.createComponent(Login);
        component = fixture.componentInstance;
        fixture.detectChanges();
    });

    it("should create", () => {
        expect(component).toBeTruthy();
    });

    it("resolves valid in-app returnUrl", () => {
        returnUrl = "/events?range=30d";
        expect(component["resolveReturnUrl"]()).toBe("/events?range=30d");
    });

    it("falls back for unsafe returnUrl", () => {
        returnUrl = "https://evil.example/phish";
        expect(component["resolveReturnUrl"]()).toBe("/dashboard");
    });

    it("stores recovery-code MFA state from the login response", () => {
        authMock.login.mockReturnValueOnce(
            of({
                status: "mfa_required" as const,
                challenge_token: "challenge-123",
                factors: ["recovery_code" as const]
            })
        );

        component["loginForm"].email().control().setValue("user@example.com");
        component["loginForm"].password().control().setValue("password123");
        component.onSubmit();

        expect(component["mfaChallengeToken"]()).toBe("challenge-123");
        expect(component["mfaHasRecoveryCode"]()).toBe(true);
        expect(component["mfaHasTotp"]()).toBe(false);
    });

    it("verifies recovery code MFA with the current challenge token", () => {
        component["mfaChallengeToken"].set("challenge-456");
        component["mfaFactors"].set(["recovery_code"]);
        component["loginForm"].recoveryCode().control().setValue("ABCD-EFGH");

        component["verifyRecoveryCodeMfa"]();

        expect(authMock.verifyMfaRecoveryCode).toHaveBeenCalledWith("challenge-456", "ABCD-EFGH");
    });

    it("builds region-aware signup URLs for hosted cloud", () => {
        expect(component["currentJurisdiction"]()).toBe("EU");
        expect(component["primarySignupUrl"]()).toBe("/signup");
        expect(component["alternateJurisdiction"]()).toBe("US");
        expect(component["alternateSignupUrl"]()).toBe("https://cloud.hitkeep.com/signup");
    });
});
