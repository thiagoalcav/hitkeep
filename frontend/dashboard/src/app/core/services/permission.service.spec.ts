import { TestBed } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import { provideHttpClientTesting } from '@angular/common/http/testing';

import { PermissionService } from './permission.service';

describe('PermissionService', () => {
    let service: PermissionService;

    beforeEach(() => {
        TestBed.configureTestingModule({
            providers: [provideHttpClient(), provideHttpClientTesting()]
        });
        service = TestBed.inject(PermissionService);
    });

    it('stores the latest permission context from the API', () => {
        service.applyPermissions({
            instance_role: 'user',
            instance_permissions: ['site.view'],
            instance_capabilities: ['site.view'],
            permissions: {
                'site-1': 'owner'
            },
            site_capabilities: {
                'site-1': ['site.view', 'site.delete']
            },
            active_team_role: 'admin',
            active_team_capabilities: ['team.manage_settings']
        });

        const context = service.permissions();
        expect(context?.instance_capabilities).toEqual(['site.view']);
        expect(context?.site_capabilities).toEqual({ 'site-1': ['site.view', 'site.delete'] });
        expect(context?.active_team_role).toBe('admin');
        expect(context?.active_team_capabilities).toEqual(['team.manage_settings']);
    });
});
