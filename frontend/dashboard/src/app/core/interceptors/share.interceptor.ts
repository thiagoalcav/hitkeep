import { HttpInterceptorFn } from "@angular/common/http";
import { inject } from "@angular/core";
import { ShareService } from "@services/share.service";

export const shareInterceptor: HttpInterceptorFn = (req, next) => {
    const share = inject(ShareService);
    const token = share.token();

    if (!token) {
        return next(req);
    }

    if (!req.url.startsWith("/api/") || req.url.startsWith("/api/share/")) {
        return next(req);
    }

    if (req.method !== "GET" && req.method !== "HEAD") {
        return next(req);
    }

    if (req.url.startsWith("/api/sites/")) {
        const url = req.url.replace("/api/sites/", `/api/share/${encodeURIComponent(token)}/sites/`);
        return next(req.clone({ url }));
    }

    return next(req);
};
