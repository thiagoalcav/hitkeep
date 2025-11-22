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
    path: '',
    component: MainLayout,
    canActivate: [setupGuard],
    children: [
      {
        path: 'dashboard',
        loadComponent: () => import('./pages/dashboard/dashboard').then(m => m.Dashboard)
      },
      { path: '', redirectTo: 'dashboard', pathMatch: 'full' }
    ]
  },
  { path: '**', redirectTo: '/dashboard' }
];
