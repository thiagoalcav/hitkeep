import { Injectable, computed, inject } from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { Router } from '@angular/router';
import { MenuItem } from 'primeng/api';
import { TranslocoService } from '@jsverse/transloco';
import { PermissionService } from '@services/permission.service';
import { AuthService } from '@services/auth.service';
import { catchError, finalize, of } from 'rxjs';

@Injectable({ providedIn: 'root' })
export class UserMenuService {
    private router = inject(Router);
    private perms = inject(PermissionService);
    private auth = inject(AuthService);
    private transloco = inject(TranslocoService);
    private isSigningOut = false;
    private activeLanguage = toSignal(this.transloco.langChanges$, { initialValue: this.transloco.getActiveLang() });

    readonly menuItems = computed<MenuItem[]>(() => {
        this.activeLanguage();
        return [
            {
                label: this.transloco.translate('userMenu.administration'),
                icon: 'pi pi-shield',
                visible: this.perms.isInstanceAdmin(),
                command: () => this.router.navigate(['/admin'])
            },
            {
                label: this.transloco.translate('userMenu.userSettings'),
                icon: 'pi pi-user',
                command: () => this.router.navigate(['/settings'])
            },
            { separator: true },
            {
                label: this.transloco.translate('userMenu.signOut'),
                icon: 'pi pi-sign-out',
                command: () => this.signOut()
            }
        ];
    });

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
