import { HttpErrorResponse, HttpInterceptorFn } from '@angular/common/http';
import { inject } from '@angular/core';
import { Router } from '@angular/router';
import { catchError, throwError } from 'rxjs';
import { AuthService } from '@services/auth.service';
import { ShareService } from '@services/share.service';

export const authInterceptor: HttpInterceptorFn = (req, next) => {
    const router = inject(Router);
    const auth = inject(AuthService);
    const share = inject(ShareService);

    // We clone the request to ensure credentials (cookies) are included.
    // This ensures the http-only cookie is sent to the backend.
    const authReq = req.clone({
        withCredentials: true
    });

    return next(authReq).pipe(
        catchError((error: HttpErrorResponse) => {
            // If we receive a 401 Unauthorized, it means the cookie is missing or invalid.
            if (error.status === 401 && !share.isShareMode()) {
                auth.markUnauthenticated();
                // Avoid redirect loops if already on login or setup
                const currentUrl = router.routerState.snapshot.url;
                if (!currentUrl.startsWith('/login') && !currentUrl.startsWith('/setup')) {
                    router.navigate(['/login']);
                }
            }
            return throwError(() => error);
        })
    );
};
