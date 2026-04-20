import { HttpClient, HttpContext, HttpErrorResponse } from '@angular/common/http';
import { inject } from '@angular/core';
import { CanActivateFn, Router } from '@angular/router';
import { catchError, map, of, switchMap } from 'rxjs';

import { SKIP_AUTH_REDIRECT } from '@core/interceptors/auth.interceptor';
import { SystemStatus } from '@models/analytics.types';
import { AuthService } from '@services/auth.service';

export const cloudSignupGuard: CanActivateFn = () => {
    const http = inject(HttpClient);
    const router = inject(Router);
    const auth = inject(AuthService);

    return http.get<SystemStatus>('/api/status').pipe(
        switchMap((status) => {
            if (status.cloud?.hosted && status.cloud.signup_enabled) {
                return http.get('/api/user/profile', { context: new HttpContext().set(SKIP_AUTH_REDIRECT, true) }).pipe(
                    map(() => {
                        auth.markAuthenticated();
                        return router.createUrlTree(['/dashboard']);
                    }),
                    catchError((error: HttpErrorResponse) => {
                        if (error.status === 401) {
                            auth.markUnauthenticated();
                        }
                        return of(true);
                    })
                );
            }
            if (status.needs_setup) {
                return of(router.createUrlTree(['/setup']));
            }
            return of(router.createUrlTree(['/login']));
        }),
        catchError(() => {
            console.error('Could not determine cloud signup availability.');
            return of(router.createUrlTree(['/login']));
        })
    );
};
