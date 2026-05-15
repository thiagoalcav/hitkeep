import { DOCUMENT } from '@angular/common';
import { HttpContextToken, HttpErrorResponse, HttpInterceptorFn } from '@angular/common/http';
import { inject } from '@angular/core';
import { Router } from '@angular/router';
import { EMPTY, catchError, throwError } from 'rxjs';
import { AuthService } from '@services/auth.service';
import { ShareService } from '@services/share.service';
import { browserBasePath } from './base-path.interceptor';

export const SKIP_AUTH_REDIRECT = new HttpContextToken<boolean>(() => false);

interface ReturnUrlRouterContext {
    url: string;
    routerState: {
        snapshot: {
            url: string;
        };
    };
}

export function resolveCurrentReturnUrl(router: ReturnUrlRouterContext, basePath = '/'): string {
    const browserPath = typeof window !== 'undefined' && typeof window.location !== 'undefined' ? `${window.location.pathname || ''}${window.location.search || ''}${window.location.hash || ''}` : '';

    const normalizedBrowserPath = stripBrowserBasePath(browserPath, basePath);
    const candidate = normalizedBrowserPath && normalizedBrowserPath !== '/' ? normalizedBrowserPath : router.url || router.routerState.snapshot.url || '/dashboard';
    if (!candidate.startsWith('/') || candidate.startsWith('//')) {
        return '/dashboard';
    }
    return candidate;
}

export function shouldRedirectAfterUnauthorized(currentUrl: string): boolean {
    return !currentUrl.startsWith('/login') && !currentUrl.startsWith('/setup');
}

export const authInterceptor: HttpInterceptorFn = (req, next) => {
    const router = inject(Router);
    const auth = inject(AuthService);
    const share = inject(ShareService);
    const document = inject(DOCUMENT);
    const isAuthRequest =
        req.url.startsWith('/api/login') || req.url.startsWith('/api/logout') || req.url.startsWith('/api/initial-user') || req.url.startsWith('/api/auth/') || req.url.startsWith('/api/cloud/') || req.url.startsWith('/api/user/password');

    // We clone the request to ensure credentials (cookies) are included.
    // This ensures the http-only cookie is sent to the backend.
    const authReq = req.clone({
        withCredentials: true
    });

    return next(authReq).pipe(
        catchError((error: HttpErrorResponse) => {
            if (req.context.get(SKIP_AUTH_REDIRECT)) {
                return throwError(() => error);
            }

            // If we receive a 401 Unauthorized, it means the cookie is missing or invalid.
            if (error.status === 401 && !share.isShareMode() && !isAuthRequest) {
                auth.markUnauthenticated();

                // Avoid redirect loops if already on login or setup. We still navigate
                // when the local session timer already marked the user unauthenticated;
                // background dashboard refreshes are often the first server-confirmed
                // signal that the cookie expired.
                const currentUrl = resolveCurrentReturnUrl(router, browserBasePath(document));
                if (shouldRedirectAfterUnauthorized(currentUrl)) {
                    void router.navigate(['/login'], {
                        queryParams: { returnUrl: currentUrl }
                    });
                }

                // Complete the request stream so late 401s do not crash screens with
                // subscriptions that omit explicit error handlers.
                return EMPTY;
            }
            return throwError(() => error);
        })
    );
};

function stripBrowserBasePath(browserPath: string, basePath: string): string {
    if (!browserPath || basePath === '/') {
        return browserPath;
    }
    const prefix = basePath.endsWith('/') ? basePath.slice(0, -1) : basePath;
    if (browserPath === prefix) {
        return '/';
    }
    if (browserPath.startsWith(`${prefix}/`)) {
        return browserPath.slice(prefix.length) || '/';
    }
    return browserPath;
}
