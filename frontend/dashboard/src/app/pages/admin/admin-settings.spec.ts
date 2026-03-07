import { signal } from "@angular/core";
import { provideHttpClient } from "@angular/common/http";
import { HttpErrorResponse } from "@angular/common/http";
import { TestBed } from "@angular/core/testing";
import { TranslocoTestingModule } from "@jsverse/transloco";
import { ConfirmationService } from "primeng/api";
import { provideRouter } from "@angular/router";
import { provideHttpClientTesting } from "@angular/common/http/testing";
import { vi } from "vitest";

import { UserProfileService } from "@services/user-profile.service";
import { AdminSettings } from "./admin-settings";

interface AdminSettingsTestAccess {
    handleDeleteUserError(err: unknown, user: { email: string }): boolean;
    deleteUserBlock(): {
        email: string;
        teams: string[];
    } | null;
    deleteUserBlockMessage(): string;
}

describe("AdminSettings", () => {
    let component: AdminSettingsTestAccess;

    beforeEach(() => {
        TestBed.configureTestingModule({
            imports: [
                TranslocoTestingModule.forRoot({
                    langs: {
                        en: {
                            admin: {
                                errors: {
                                    deleteUserBlockedOwnership: "Cannot delete {{email}} until ownership is transferred for: {{teams}}."
                                }
                            }
                        }
                    },
                    translocoConfig: {
                        availableLangs: ["en"],
                        defaultLang: "en"
                    },
                    preloadLangs: true
                })
            ],
            providers: [
                provideRouter([]),
                provideHttpClient(),
                provideHttpClientTesting(),
                ConfirmationService,
                {
                    provide: UserProfileService,
                    useValue: {
                        profile: signal({ id: "admin-user", email: "admin@example.com" }),
                        loadProfile: vi.fn()
                    }
                }
            ]
        });

        component = TestBed.runInInjectionContext(() => new AdminSettings()) as unknown as AdminSettingsTestAccess;
    });

    it("stores blocking team details for sole-owner delete errors", () => {
        const handled = component.handleDeleteUserError(
            new HttpErrorResponse({
                status: 409,
                error: {
                    status: "error",
                    code: "user_owns_teams",
                    message: "Transfer ownership before deleting this user.",
                    teams: [
                        { id: "team-1", name: "Acme" },
                        { id: "team-2", name: "Northwind Studio" }
                    ]
                }
            }),
            { email: "owner@example.com" }
        );

        expect(handled).toBe(true);
        expect(component.deleteUserBlock()).toEqual({
            email: "owner@example.com",
            teams: ["Acme", "Northwind Studio"]
        });
        expect(component.deleteUserBlockMessage()).toContain("owner@example.com");
        expect(component.deleteUserBlockMessage()).toContain("Acme, Northwind Studio");
    });

    it("ignores unrelated delete errors", () => {
        const handled = component.handleDeleteUserError(
            new HttpErrorResponse({
                status: 500,
                error: {
                    status: "error",
                    code: "unexpected",
                    message: "Unexpected failure"
                }
            }),
            { email: "owner@example.com" }
        );

        expect(handled).toBe(false);
        expect(component.deleteUserBlock()).toBeNull();
    });
});
