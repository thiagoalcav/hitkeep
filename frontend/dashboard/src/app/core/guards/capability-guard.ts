import { inject } from '@angular/core';
import { CanActivateFn, Router, UrlTree } from '@angular/router';
import { map } from 'rxjs';
import { InstanceCapability, SiteCapability, TeamCapability } from '@core/access/capabilities';
import { AccessService } from '@services/access.service';
import { PermissionService } from '@services/permission.service';

export interface CapabilityRouteData {
    instanceCapability?: InstanceCapability;
    activeSiteCapability?: SiteCapability;
    activeTeamCapability?: TeamCapability;
}

export const capabilityGuard: CanActivateFn = (route) => {
    const access = inject(AccessService);
    const permissions = inject(PermissionService);
    const router = inject(Router);
    const data = route.data as CapabilityRouteData;

    const evaluate = (): boolean | UrlTree => {
        if (data.instanceCapability && !access.hasInstance(data.instanceCapability)) {
            return router.createUrlTree(['/dashboard']);
        }
        if (data.activeSiteCapability && !access.canActiveSite(data.activeSiteCapability)) {
            return router.createUrlTree(['/dashboard']);
        }
        if (data.activeTeamCapability && !access.canActiveTeam(data.activeTeamCapability)) {
            return router.createUrlTree(['/dashboard']);
        }
        return true;
    };

    if (permissions.permissions()) {
        return evaluate();
    }

    return permissions.loadPermissions().pipe(map(() => evaluate()));
};
