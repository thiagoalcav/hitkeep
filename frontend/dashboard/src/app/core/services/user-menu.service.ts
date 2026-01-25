import { Injectable, computed, inject } from '@angular/core';
import { Router } from '@angular/router';
import { MenuItem } from 'primeng/api';
import { PermissionService } from '@services/permission.service';
import { AuthService } from '@services/auth.service';
import { catchError, finalize, of } from 'rxjs';

@Injectable({ providedIn: 'root' })
export class UserMenuService {
    private router = inject(Router);
    private perms = inject(PermissionService);
    private auth = inject(AuthService);
    private isSigningOut = false;

    readonly menuItems = computed<MenuItem[]>(() => [
        {
            label: 'Administration',
            icon: 'pi pi-shield',
            visible: this.perms.isInstanceAdmin(),
            command: () => this.router.navigate(['/admin'])
        },
        {
            label: 'User Settings',
            icon: 'pi pi-user',
            command: () => this.router.navigate(['/settings/user'])
        },
        {
            label: 'Preferences',
            icon: 'pi pi-cog',
            command: () => this.router.navigate(['/settings/preferences'])
        },
        { separator: true },
        {
            label: 'Sign Out',
            icon: 'pi pi-sign-out',
            command: () => this.signOut()
        }
    ]);

    signOut() {
        if (this.isSigningOut) return;
        this.isSigningOut = true;
        this.auth
            .logout()
            .pipe(
                catchError(() => of(null)),
                finalize(() => {
                    this.isSigningOut = false;
                    this.router.navigate(['/login']);
                })
            )
            .subscribe();
    }
}
