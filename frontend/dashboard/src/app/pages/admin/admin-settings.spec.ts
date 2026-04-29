import { signal } from '@angular/core';
import { provideHttpClient } from '@angular/common/http';
import { HttpErrorResponse } from '@angular/common/http';
import { TestBed } from '@angular/core/testing';
import { TranslocoTestingModule } from '@jsverse/transloco';
import { ConfirmationService } from 'primeng/api';
import { provideRouter } from '@angular/router';
import { provideHttpClientTesting } from '@angular/common/http/testing';
import { vi } from 'vitest';

import { UserProfileService } from '@services/user-profile.service';
import { PermissionService } from '@services/permission.service';
import { AdminSettings } from './admin-settings';

interface AdminSettingsTestAccess {
    handleDeleteUserError(err: unknown, user: { email: string }): boolean;
    deleteUserBlock(): {
        email: string;
        teams: string[];
    } | null;
    deleteUserBlockMessage(): string;
    userActionStatus: {
        set(
            value: {
                severity: 'success' | 'error';
                key: string;
                params?: Record<string, string | number>;
            } | null
        ): void;
    };
    userActionStatusMessage(): string;
    canDisableUserMfa(): boolean;
    currentUserId: { set(value: string): void };
    users: {
        set(
            value: {
                id: string;
                email: string;
                instance_role: 'owner' | 'admin' | 'user';
                created_at: string;
            }[]
        ): void;
    };
}

describe('AdminSettings', () => {
    let component: AdminSettingsTestAccess;
    const permissionServiceMock = {
        isInstanceOwner: signal(false),
        permissions: signal(null)
    };

    beforeEach(() => {
        TestBed.configureTestingModule({
            imports: [
                TranslocoTestingModule.forRoot({
                    langs: {
                        en: {
                            admin: {
                                errors: {
                                    deleteUserBlockedOwnership: 'Cannot delete {{email}} until ownership is transferred for: {{teams}}.'
                                },
                                status: {
                                    deleteUserSuccess: 'Deleted user {{email}}.'
                                }
                            }
                        }
                    },
                    translocoConfig: {
                        availableLangs: ['en'],
                        defaultLang: 'en'
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
                        profile: signal({ id: 'admin-user', email: 'admin@example.com' }),
                        loadProfile: vi.fn()
                    }
                },
                {
                    provide: PermissionService,
                    useValue: permissionServiceMock
                }
            ]
        });

        permissionServiceMock.isInstanceOwner.set(false);
        permissionServiceMock.permissions.set(null);
        component = TestBed.runInInjectionContext(() => new AdminSettings()) as unknown as AdminSettingsTestAccess;
    });

    it('stores blocking team details for sole-owner delete errors', () => {
        const handled = component.handleDeleteUserError(
            new HttpErrorResponse({
                status: 409,
                error: {
                    status: 'error',
                    code: 'user_owns_teams',
                    message: 'Transfer ownership before deleting this user.',
                    teams: [
                        { id: 'team-1', name: 'Acme' },
                        { id: 'team-2', name: 'Northwind Studio' }
                    ]
                }
            }),
            { email: 'owner@example.com' }
        );

        expect(handled).toBe(true);
        expect(component.deleteUserBlock()).toEqual({
            email: 'owner@example.com',
            teams: ['Acme', 'Northwind Studio']
        });
        expect(component.deleteUserBlockMessage()).toContain('owner@example.com');
        expect(component.deleteUserBlockMessage()).toContain('Acme, Northwind Studio');
    });

    it('ignores unrelated delete errors', () => {
        const handled = component.handleDeleteUserError(
            new HttpErrorResponse({
                status: 500,
                error: {
                    status: 'error',
                    code: 'unexpected',
                    message: 'Unexpected failure'
                }
            }),
            { email: 'owner@example.com' }
        );

        expect(handled).toBe(false);
        expect(component.deleteUserBlock()).toBeNull();
    });

    it('renders inline admin action feedback messages', () => {
        component.userActionStatus.set({
            severity: 'success',
            key: 'admin.status.deleteUserSuccess',
            params: { email: 'owner@example.com' }
        });

        expect(component.userActionStatusMessage()).toBe('Deleted user owner@example.com.');
    });

    it('allows MFA recovery actions when the current user is an instance owner', () => {
        permissionServiceMock.isInstanceOwner.set(true);

        expect(component.canDisableUserMfa()).toBe(true);
    });

    it('falls back to the loaded current user role for MFA recovery actions', () => {
        component.currentUserId.set('owner-user');
        component.users.set([
            {
                id: 'owner-user',
                email: 'owner@example.com',
                instance_role: 'owner',
                created_at: '2026-03-10T00:00:00Z'
            }
        ]);

        expect(component.canDisableUserMfa()).toBe(true);
    });
});
