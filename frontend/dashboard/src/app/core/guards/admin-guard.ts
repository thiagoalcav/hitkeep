import { inject } from '@angular/core';
import { CanActivateFn, Router, UrlTree } from '@angular/router';
import { PermissionService } from '@services/permission.service';
import { map, of, switchMap } from 'rxjs';

export const adminGuard: CanActivateFn = () => {
    const perms = inject(PermissionService);
    const router = inject(Router);

    const checkAdmin = (): boolean | UrlTree => {
        if (perms.isInstanceAdmin()) {
            return true;
        }
        return router.createUrlTree(['/dashboard']);
    };

    if (perms.permissions()) {
        return checkAdmin();
    }

    return perms.loadPermissions().pipe(
        map(() => checkAdmin()),
        switchMap((result) => of(result))
    );
};
