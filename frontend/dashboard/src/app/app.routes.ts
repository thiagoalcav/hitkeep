import { Routes } from '@angular/router';
import { setupGuard } from './core/guards/setup-guard';
import { MainLayout } from './core/layout/main-layout';
import { adminGuard } from './core/guards/admin-guard';

export const routes: Routes = [
  {
    path: 'setup',
    loadComponent: () => import('./pages/setup/setup').then(m => m.Setup),
    canActivate: [setupGuard]
  },
  {
    path: 'login',
    loadComponent: () => import('./pages/login/login').then(m => m.Login),
    canActivate: [setupGuard]
  },
  {
    path: 'forgot-password',
    loadComponent: () => import('./pages/password/forgot-password').then(m => m.ForgotPassword)
  },
  {
    path: 'reset-password',
    loadComponent: () => import('./pages/password/reset-password').then(m => m.ResetPassword)
  },
  {
    path: '',
    component: MainLayout,
    canActivate: [setupGuard],
    children: [
      {
        path: 'dashboard',
        loadComponent: () => import('./pages/dashboard/dashboard').then(m => m.Dashboard)
      },
      {
        path: 'goals',
        loadComponent: () => import('./pages/goals/goals').then(m => m.Goals)
      },
      {
        path: 'funnels',
        loadComponent: () => import('./pages/funnels/funnels').then(m => m.Funnels)
      },
      {
        path: 'settings',
        loadChildren: () => import('./pages/settings/settings.routes').then(m => m.SETTINGS_ROUTES)
      },
      {
        path: 'admin',
        loadComponent: () => import('./pages/admin/admin-settings').then(m => m.AdminSettings),
        canActivate: [adminGuard]
      },
      { path: '', redirectTo: 'dashboard', pathMatch: 'full' }
    ]
  },
  { path: '**', redirectTo: '/dashboard' }
];
