import { ComponentFixture, TestBed } from '@angular/core/testing';
import { signal } from '@angular/core';
import { NoopAnimationsModule } from '@angular/platform-browser/animations';
import { provideRouter } from '@angular/router';
import { TranslocoTestingModule } from '@jsverse/transloco';
import { of, throwError } from 'rxjs';
import { vi } from 'vitest';

import { Team } from '@models/analytics.types';
import { CloudService } from '@services/cloud.service';
import { DashboardBootstrapService } from '@services/dashboard-bootstrap.service';
import { ShareService } from '@services/share.service';
import { TeamService } from '@services/team.service';
import { FreePlanRetentionNotice } from './free-plan-retention-notice';

describe('FreePlanRetentionNotice', () => {
    type TestAccess = FreePlanRetentionNotice & {
        startUpgrade(): void;
        redirectTo(url: string): void;
    };

    let fixture: ComponentFixture<FreePlanRetentionNotice>;
    let activeTeam = signal<Team | null>(freeTeam('team-a'));
    let cloudHosted = signal(true);
    let shareMode = signal(false);
    let createBillingCheckoutSession = vi.fn();

    beforeEach(async () => {
        activeTeam = signal<Team | null>(freeTeam('team-a'));
        cloudHosted = signal(true);
        shareMode = signal(false);
        createBillingCheckoutSession = vi.fn().mockReturnValue(of({ url: 'https://checkout.stripe.test/session' }));
        window.localStorage.removeItem('hitkeep.freeRetentionNotice.dismissed.team-a');
        window.localStorage.removeItem('hitkeep.freeRetentionNotice.dismissed.team-b');

        await TestBed.configureTestingModule({
            imports: [
                FreePlanRetentionNotice,
                NoopAnimationsModule,
                TranslocoTestingModule.forRoot({
                    langs: {
                        en: {
                            cloud: {
                                retentionNotice: {
                                    message: 'Free plan data is retained for {{count}} days.',
                                    upgradeAction: 'Upgrade to Pro',
                                    compareAction: 'Compare plans',
                                    dismissAction: 'Dismiss retention notice',
                                    checkoutError: 'Could not start checkout.'
                                }
                            }
                        }
                    },
                    translocoConfig: {
                        availableLangs: ['en'],
                        defaultLang: 'en'
                    }
                })
            ],
            providers: [
                provideRouter([]),
                {
                    provide: DashboardBootstrapService,
                    useValue: {
                        cloudHosted
                    }
                },
                {
                    provide: TeamService,
                    useValue: {
                        activeTeam
                    }
                },
                {
                    provide: ShareService,
                    useValue: {
                        isShareMode: shareMode
                    }
                },
                {
                    provide: CloudService,
                    useValue: {
                        createBillingCheckoutSession
                    }
                }
            ]
        }).compileComponents();

        fixture = TestBed.createComponent(FreePlanRetentionNotice);
        fixture.detectChanges();
    });

    afterEach(() => {
        window.localStorage.removeItem('hitkeep.freeRetentionNotice.dismissed.team-a');
        window.localStorage.removeItem('hitkeep.freeRetentionNotice.dismissed.team-b');
        vi.restoreAllMocks();
    });

    it('shows the retention notice for hosted free-plan teams', () => {
        const notice = noticeElement();

        expect(notice).not.toBeNull();
        expect(notice?.textContent).toContain('Free plan data is retained for 60 days.');
        expect(notice?.querySelector('a')?.getAttribute('href')).toBe('/admin/team');
    });

    it('hides the notice outside cloud free-plan dashboard mode', () => {
        activeTeam.set({
            ...freeTeam('team-a'),
            plan: { code: 'pro', name: 'Pro' }
        });
        fixture.detectChanges();
        expect(noticeElement()).toBeNull();

        activeTeam.set(freeTeam('team-a'));
        cloudHosted.set(false);
        fixture.detectChanges();
        expect(noticeElement()).toBeNull();

        cloudHosted.set(true);
        shareMode.set(true);
        fixture.detectChanges();
        expect(noticeElement()).toBeNull();
    });

    it('dismisses the notice per active team in localStorage', () => {
        dismissButton()?.click();
        fixture.detectChanges();

        expect(noticeElement()).toBeNull();
        expect(window.localStorage.getItem('hitkeep.freeRetentionNotice.dismissed.team-a')).toBe('dismissed');

        activeTeam.set(freeTeam('team-b'));
        fixture.detectChanges();
        expect(noticeElement()).not.toBeNull();

        activeTeam.set(freeTeam('team-a'));
        fixture.detectChanges();
        expect(noticeElement()).toBeNull();
    });

    it('starts a Pro checkout session and redirects to Stripe', () => {
        const component = fixture.componentInstance as TestAccess;
        const redirectSpy = vi.spyOn(component, 'redirectTo').mockImplementation(() => undefined);

        component.startUpgrade();

        expect(createBillingCheckoutSession).toHaveBeenCalledWith({
            plan_code: 'pro',
            locale: 'en'
        });
        expect(redirectSpy).toHaveBeenCalledWith('https://checkout.stripe.test/session');
    });

    it('shows an error when checkout cannot be started', () => {
        const component = fixture.componentInstance as TestAccess;
        createBillingCheckoutSession.mockReturnValueOnce(throwError(() => new Error('checkout unavailable')));

        component.startUpgrade();
        fixture.detectChanges();

        expect(noticeElement()?.textContent).toContain('Could not start checkout.');
    });

    function noticeElement(): HTMLElement | null {
        return fixture.nativeElement.querySelector('[data-testid="free-plan-retention-notice"]') as HTMLElement | null;
    }

    function dismissButton(): HTMLButtonElement | null {
        return fixture.nativeElement.querySelector('.free-plan-retention-notice__dismiss') as HTMLButtonElement | null;
    }
});

function freeTeam(id: string): Team {
    return {
        id,
        name: 'Acme Analytics',
        logo_url: '',
        role: 'owner',
        created_at: '2026-03-01T00:00:00Z',
        entitlements: {
            max_sites_per_team: 3,
            max_team_members: 3,
            max_retention_days: 60,
            allow_sso: false,
            allow_custom_branding: false
        },
        plan: {
            code: 'free',
            name: 'Free'
        }
    };
}
