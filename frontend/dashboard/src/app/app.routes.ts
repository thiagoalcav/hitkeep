import { Routes } from '@angular/router';
import { setupGuard } from './core/guards/setup-guard';
import { MainLayout } from './core/layout/main-layout';

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
        path: 'settings',
        loadComponent: () => import('./pages/settings/settings').then(m => m.Settings)
      },
      { path: '', redirectTo: 'dashboard', pathMatch: 'full' }
    ]
  },
  { path: '**', redirectTo: '/dashboard' }
];
