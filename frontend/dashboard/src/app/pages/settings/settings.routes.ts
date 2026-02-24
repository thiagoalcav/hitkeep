import { Route } from "@angular/router";

export const SETTINGS_ROUTES: Route[] = [
    {
        path: "",
        loadComponent: () => import("@pages/settings/user/user-settings").then((m) => m.UserSettings)
    },
    { path: "user", redirectTo: "", pathMatch: "full" },
    { path: "preferences", redirectTo: "", pathMatch: "full" },
    {
        path: "reports",
        loadComponent: () => import("@pages/settings/reports/report-settings").then((m) => m.ReportSettings)
    }
];
