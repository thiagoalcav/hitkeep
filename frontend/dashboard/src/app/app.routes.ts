// frontend/dashboard/src/app/app.routes.ts
import { Routes } from '@angular/router';
import { Setup } from './setup/setup';
import {setupGuard} from './core/guards/setup-guard';
import {Dashboard} from './dashboard/dashboard';
import {Login} from './login/login';

export const routes: Routes = [
  {
    path: 'setup',
    component: Setup,
    canActivate: [setupGuard]
  },
  {
    path: 'login',
    component: Login,
    canActivate: [setupGuard]
  },
  {
    path: 'dashboard',
    component: Dashboard,
    canActivate: [setupGuard]
  },
  { path: '', redirectTo: '/dashboard', pathMatch: 'full' },
];
