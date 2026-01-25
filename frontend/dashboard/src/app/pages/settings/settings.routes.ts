import { Route } from '@angular/router';

export const SETTINGS_ROUTES: Route[] = [
    {
        path: 'user',
        loadComponent: () => import('@pages/settings/user/user-settings').then((m) => m.UserSettings)
    },
    {
        path: 'preferences',
        loadComponent: () => import('@pages/settings/preferences/preferences').then((m) => m.Preferences)
    },
    { path: '', redirectTo: 'user', pathMatch: 'full' }
];
