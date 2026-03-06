import { ComponentFixture, TestBed } from "@angular/core/testing";
import { ActivatedRoute, convertToParamMap, provideRouter } from "@angular/router";
import { TranslocoTestingModule } from "@jsverse/transloco";
import { of } from "rxjs";

import { Login } from "@pages/login/login";
import { AuthService } from "@services/auth.service";
import { UserPreferencesService } from "@services/user-preferences.service";

describe("Login", () => {
    let component: Login;
    let fixture: ComponentFixture<Login>;
    let returnUrl: string | null;

    beforeEach(async () => {
        returnUrl = null;
        const authMock = {
            status: () => "unknown",
            login: () => of({ status: "ok" as const }),
            startPasskeyLogin: () =>
                of({
                    challenge_token: "",
                    publicKey: {
                        challenge: "",
                        rpId: "",
                        timeout: 0,
                        userVerification: "preferred" as UserVerificationRequirement
                    }
                }),
            finishPasskeyLogin: () => of(void 0),
            verifyMfaTotp: () => of(void 0)
        } as unknown as AuthService;

        const preferencesMock = {
            load: () => of(void 0)
        } as unknown as UserPreferencesService;

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
                { provide: AuthService, useValue: authMock },
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
        expect((component)["resolveReturnUrl"]()).toBe("/events?range=30d");
    });

    it("falls back for unsafe returnUrl", () => {
        returnUrl = "https://evil.example/phish";
        expect((component)["resolveReturnUrl"]()).toBe("/dashboard");
    });
});
