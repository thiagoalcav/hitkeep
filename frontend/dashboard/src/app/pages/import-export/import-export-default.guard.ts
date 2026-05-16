import { inject } from '@angular/core';
import { CanActivateFn, Router } from '@angular/router';
import { SITE_CAPABILITIES } from '@core/access/capabilities';
import { AccessService } from '@services/access.service';
import { SiteService } from '@features/sites/services/site.service';

export const importExportDefaultGuard: CanActivateFn = () => {
    const router = inject(Router);
    const siteService = inject(SiteService);
    const access = inject(AccessService);
    const activeSite = siteService.activeSite();
    const defaultTab = activeSite && access.canSite(activeSite.id, SITE_CAPABILITIES.manageData) ? 'import' : 'export';

    return router.parseUrl(`/import-export/${defaultTab}`);
};
