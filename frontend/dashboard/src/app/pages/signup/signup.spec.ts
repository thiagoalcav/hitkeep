import { ComponentFixture, TestBed } from '@angular/core/testing';
import { ActivatedRoute, convertToParamMap } from '@angular/router';
import { Router } from '@angular/router';
import { TranslocoTestingModule } from '@jsverse/transloco';
import { NEVER, of } from 'rxjs';
import { vi } from 'vitest';

import { Signup } from '@pages/signup/signup';
import { AnalyticsService } from '@services/analytics.service';
import { CloudSignupTrackingService } from '@services/cloud-signup-tracking.service';
import { CloudService, CloudSignupResponse } from '@services/cloud.service';

describe('Signup', () => {
    let component: Signup;
    let fixture: ComponentFixture<Signup>;
    let queryParams: Record<string, string>;

    const cloudServiceMock: { signup: ReturnType<typeof vi.fn> } = {
        signup: vi.fn(() => NEVER)
    };

    const analyticsServiceMock = {
        getSystemStatus: vi.fn(() =>
            of({
                needs_setup: false,
                version: 'v2.0.0',
                cloud: {
                    hosted: true,
                    signup_enabled: true,
                    jurisdiction: 'EU'
                }
            })
        )
    };

    const routerMock = {
        navigateByUrl: vi.fn(() => Promise.resolve(true))
    };
    const signupTrackingMock = {
        install: vi.fn()
    };

    beforeEach(async () => {
        vi.clearAllMocks();
        routerMock.navigateByUrl.mockClear();
        signupTrackingMock.install.mockClear();
        queryParams = {};

        await TestBed.configureTestingModule({
            imports: [
                Signup,
                TranslocoTestingModule.forRoot({
                    langs: { en: {} },
                    translocoConfig: {
                        availableLangs: ['en'],
                        defaultLang: 'en'
                    },
                    preloadLangs: true
                })
            ],
            providers: [
                { provide: Router, useValue: routerMock },
                { provide: CloudService, useValue: cloudServiceMock },
                { provide: AnalyticsService, useValue: analyticsServiceMock },
                { provide: CloudSignupTrackingService, useValue: signupTrackingMock },
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
                    template: '<div></div>'
                }
            })
            .compileComponents();

        fixture = TestBed.createComponent(Signup);
        component = fixture.componentInstance;
        fixture.detectChanges();
    });

    it('shows the hosted jurisdiction from system status', () => {
        expect(component['currentJurisdiction']()).toBe('EU');
    });

    it('installs signup-only tracking for hosted cloud signup', () => {
        expect(signupTrackingMock.install).toHaveBeenCalledTimes(1);
    });

    it('submits a cloud signup request with free plan', () => {
        cloudServiceMock.signup.mockReturnValue(of({ status: 'verification_sent', plan_code: 'free' } as CloudSignupResponse));

        component['signupForm'].teamName().control().setValue('Cloud Team');
        component['signupForm'].email().control().setValue('user@example.com');
        component['signupForm'].password().control().setValue('password123');
        component['signupForm'].acceptedTos().control().setValue(true);

        component['onSubmit']();

        const payload = (cloudServiceMock.signup.mock.calls[0]?.[0] ?? null) as Record<string, unknown> | null;
        expect(payload?.['team_name']).toBe('Cloud Team');
        expect(payload?.['email']).toBe('user@example.com');
        expect(payload?.['plan_code']).toBe('free');
        expect(payload?.['jurisdiction']).toBe('EU');
        expect(payload?.['locale']).toBe('en');
        expect(payload?.['accepted_tos']).toBe(true);
    });

    it('hydrates team name and email from query params', async () => {
        queryParams = {
            team_name: 'Analytical Engine',
            email: 'ada@example.com'
        };

        fixture = TestBed.createComponent(Signup);
        component = fixture.componentInstance;
        fixture.detectChanges();

        expect(component['signupForm'].teamName().value()).toBe('Analytical Engine');
        expect(component['signupForm'].email().value()).toBe('ada@example.com');
    });

    it('provides a jurisdiction href for the alternate region', () => {
        component['signupForm'].teamName().control().setValue('Cloud Team');
        component['signupForm'].email().control().setValue('user@example.com');

        const href = component['jurisdictionHref']('US');
        expect(href).toContain('https://cloud.hitkeep.com/signup');
        expect(href).toContain('team_name=Cloud+Team');
        expect(href).toContain('email=user%40example.com');
    });
});
