import { TestBed } from '@angular/core/testing';
import { ActivatedRouteSnapshot, Router, UrlTree, provideRouter } from '@angular/router';
import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { firstValueFrom, isObservable } from 'rxjs';
import { INSTANCE_CAPABILITIES, SITE_CAPABILITIES, TEAM_CAPABILITIES } from '@core/access/capabilities';
import { SiteService } from '@features/sites/services/site.service';
import { PermissionService, UserPermissions } from '@services/permission.service';
import { TeamService } from '@services/team.service';
import { capabilityGuard } from './capability-guard';

describe('capabilityGuard', () => {
    beforeEach(() => {
        TestBed.configureTestingModule({
            providers: [provideRouter([]), provideHttpClient(), provideHttpClientTesting()]
        });
    });

    afterEach(() => {
        TestBed.inject(HttpTestingController).verify();
    });

    it('allows routes when all declared capabilities are available', async () => {
        seedSite();
        seedTeam('team-1', 'admin');
        seedPermissions({
            instance_capabilities: [INSTANCE_CAPABILITIES.viewSystem],
            site_capabilities: { 'site-1': [SITE_CAPABILITIES.manageData] },
            active_team_id: 'team-1',
            active_team_role: 'admin',
            active_team_capabilities: [TEAM_CAPABILITIES.manageSettings]
        });

        const result = await runGuard({
            instanceCapability: INSTANCE_CAPABILITIES.viewSystem,
            activeSiteCapability: SITE_CAPABILITIES.manageData,
            activeTeamCapability: TEAM_CAPABILITIES.manageSettings
        });
        expect(result).toBe(true);
    });

    it('redirects to dashboard when an instance, site, or team capability is missing', async () => {
        seedSite();
        seedPermissions({
            instance_capabilities: [],
            site_capabilities: { 'site-1': [SITE_CAPABILITIES.view] },
            active_team_capabilities: []
        });

        for (const data of [{ instanceCapability: INSTANCE_CAPABILITIES.viewSystem }, { activeSiteCapability: SITE_CAPABILITIES.manageData }, { activeTeamCapability: TEAM_CAPABILITIES.manageSettings }]) {
            const result = await runGuard(data);
            expect(result instanceof UrlTree).toBe(true);
            expect(TestBed.inject(Router).serializeUrl(result as UrlTree)).toBe('/dashboard');
        }
    });

    it('loads permissions before evaluating when no context exists yet', async () => {
        seedSite();

        const resultPromise = runGuard({ activeSiteCapability: SITE_CAPABILITIES.manageData });
        TestBed.inject(HttpTestingController)
            .expectOne('/api/user/permissions')
            .flush({
                instance_role: 'user',
                permissions: { 'site-1': 'viewer' },
                site_capabilities: { 'site-1': [SITE_CAPABILITIES.manageData] },
                active_team_capabilities: []
            });

        expect(await resultPromise).toBe(true);
    });
});

function seedSite() {
    TestBed.inject(SiteService).applySites([{ id: 'site-1', user_id: 'user-1', domain: 'example.com', created_at: '2026-01-01T00:00:00Z' }]);
}

function seedTeam(id: string, role: 'owner' | 'admin' | 'member') {
    TestBed.inject(TeamService).applyTeams({
        active_team_id: id,
        teams: [{ id, name: 'Team', logo_url: '', role, created_at: '2026-01-01T00:00:00Z' }]
    });
}

function seedPermissions(partial: Partial<UserPermissions>) {
    TestBed.inject(PermissionService).applyPermissions({
        instance_role: 'user',
        permissions: { 'site-1': 'viewer' },
        instance_permissions: [],
        active_team_capabilities: [],
        ...partial
    });
}

async function runGuard(data: ActivatedRouteSnapshot['data']) {
    const route = new ActivatedRouteSnapshot();
    Object.assign(route, { data });
    const result = TestBed.runInInjectionContext(() => capabilityGuard(route, {} as never));
    return isObservable(result) ? firstValueFrom(result) : result;
}
