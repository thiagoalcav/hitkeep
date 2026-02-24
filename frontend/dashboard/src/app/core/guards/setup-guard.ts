import { inject } from "@angular/core";
import { HttpClient } from "@angular/common/http";
import { CanActivateFn, Router } from "@angular/router";
import { map, catchError, of } from "rxjs";

export const setupGuard: CanActivateFn = (route, state) => {
    const http = inject(HttpClient);
    const router = inject(Router);

    // Check if we are already on the setup page to avoid redirect loops
    const isSetupRoute = state.url.startsWith("/setup");

    return http.get<{ needs_setup: boolean }>("/api/status").pipe(
        map((status) => {
            if (status.needs_setup) {
                // If setup is needed and we are NOT on the setup page, redirect to it.
                if (!isSetupRoute) {
                    return router.createUrlTree(["/setup"]);
                }
                // Otherwise, allow access (we are already on the setup page).
                return true;
            } else {
                // If setup is NOT needed and we ARE on the setup page, redirect away (to login).
                if (isSetupRoute) {
                    return router.createUrlTree(["/login"]);
                }
                // Otherwise, allow access to the requested page (e.g., /login, /dashboard).
                return true;
            }
        }),
        catchError(() => {
            // If the API fails, we can't know the status.
            // Redirect to an error page or show a message. For now, block access.
            console.error("Could not determine application status.");
            return of(false);
        })
    );
};
