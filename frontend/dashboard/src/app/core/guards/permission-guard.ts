import { inject } from '@angular/core';
import { CanActivateFn, Router, UrlTree } from '@angular/router';
import { PermissionService } from '@services/permission.service';
import { map } from 'rxjs';

export const permissionGuard: CanActivateFn = (route) => {
    const perms = inject(PermissionService);
    const router = inject(Router);

    // Get required roles from route data, e.g., data: { roles: ['owner', 'editor'] }
    const requiredRoles = route.data['roles'] as string[] | undefined;

    const checkPermissions = (): boolean | UrlTree => {
        const userRole = perms.permissions()?.instance_role || '';

        // Always allow instance owners
        if (userRole === 'owner') return true;

        // If no specific roles defined in route, allow access (or deny based on your preference)
        if (!requiredRoles || requiredRoles.length === 0) return true;

        if (requiredRoles.includes(userRole)) {
            return true;
        }

        // Default fallback
        return router.createUrlTree(['/dashboard']);
    };

    if (perms.permissions()) {
        return checkPermissions();
    }

    return perms.loadPermissions().pipe(map(() => checkPermissions()));
};
