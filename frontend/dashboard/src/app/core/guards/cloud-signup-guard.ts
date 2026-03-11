import { inject } from "@angular/core";
import { HttpClient } from "@angular/common/http";
import { CanActivateFn, Router } from "@angular/router";
import { catchError, map, of } from "rxjs";

import { SystemStatus } from "@models/analytics.types";

export const cloudSignupGuard: CanActivateFn = () => {
    const http = inject(HttpClient);
    const router = inject(Router);

    return http.get<SystemStatus>("/api/status").pipe(
        map((status) => {
            if (status.cloud?.hosted && status.cloud.signup_enabled) {
                return true;
            }
            if (status.needs_setup) {
                return router.createUrlTree(["/setup"]);
            }
            return router.createUrlTree(["/login"]);
        }),
        catchError(() => {
            console.error("Could not determine cloud signup availability.");
            return of(router.createUrlTree(["/login"]));
        })
    );
};
