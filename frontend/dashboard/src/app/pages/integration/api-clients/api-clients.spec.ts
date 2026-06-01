import { ComponentFixture, TestBed } from '@angular/core/testing';
import { signal } from '@angular/core';
import { provideRouter } from '@angular/router';
import { TranslocoTestingModule } from '@jsverse/transloco';
import { of } from 'rxjs';
import { vi } from 'vitest';

import { APIClientsPage } from './api-clients';
import { AccessService } from '@services/access.service';
import { APIClientsService } from '@services/api-clients.service';
import { PermissionService } from '@services/permission.service';
import { TeamService } from '@services/team.service';
import { SiteService } from '@features/sites/services/site.service';

describe('APIClientsPage', () => {
    let fixture: ComponentFixture<APIClientsPage>;
    const activeTeam = signal<object | null>({ id: 'team-1', name: 'Team' });
    const accessServiceMock = {
        canActiveTeam: vi.fn(() => true)
    };
    const apiClientsServiceMock = {
        listClients: vi.fn(() => of([])),
        createClient: vi.fn(),
        updateClient: vi.fn(),
        rotateClient: vi.fn(),
        deleteClient: vi.fn()
    };
    const siteServiceMock = {
        sites: signal([])
    };
    const permissions = signal({ instance_role: 'owner', permissions: {} });

    beforeEach(async () => {
        vi.clearAllMocks();
        accessServiceMock.canActiveTeam.mockReturnValue(true);
        activeTeam.set({ id: 'team-1', name: 'Team' });

        await TestBed.configureTestingModule({
            imports: [
                APIClientsPage,
                TranslocoTestingModule.forRoot({
                    langs: {
                        en: {
                            nav: { integration: 'Integration', apiClients: 'API clients' },
                            integration: { apiClients: { subtitle: 'Manage API clients', manageTeamClients: 'Manage team API clients' } }
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
                { provide: AccessService, useValue: accessServiceMock },
                { provide: APIClientsService, useValue: apiClientsServiceMock },
                { provide: SiteService, useValue: siteServiceMock },
                { provide: PermissionService, useValue: { permissions } },
                { provide: TeamService, useValue: { activeTeam } }
            ]
        }).compileComponents();

        fixture = TestBed.createComponent(APIClientsPage);
        fixture.detectChanges();
    });

    it('exposes the team API client link only when the active team capability is present', () => {
        expect(fixture.componentInstance['canManageTeamAPIClients']()).toBe(true);

        accessServiceMock.canActiveTeam.mockReturnValue(false);
        activeTeam.set({ id: 'team-2', name: 'Team 2' });
        fixture.detectChanges();

        expect(fixture.componentInstance['canManageTeamAPIClients']()).toBe(false);
    });

    it('renders the team API client link inside the API-client card actions', () => {
        const cardActions = fixture.nativeElement.querySelector('.api-client-card-actions') as HTMLElement | null;

        expect(cardActions?.textContent).toContain('Manage team API clients');
    });

    it('hides the team API client link when no team is active', () => {
        activeTeam.set(null);
        fixture.detectChanges();

        expect(fixture.componentInstance['canManageTeamAPIClients']()).toBe(false);
        expect(fixture.nativeElement.querySelector('.api-client-card-actions')?.textContent).not.toContain('Manage team API clients');
    });
});
