import { Component, effect, inject, signal, OnInit, computed } from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { CommonModule } from '@angular/common';
import { FormControl, ReactiveFormsModule } from '@angular/forms';
import { TableModule } from 'primeng/table';
import { ButtonModule } from 'primeng/button';
import { SelectModule } from 'primeng/select';
import { CardModule } from 'primeng/card';
import { TabsModule } from 'primeng/tabs';
import { HttpClient } from '@angular/common/http';
import { TranslocoPipe, TranslocoService } from '@jsverse/transloco';
import { TranslocoDatePipe } from '@jsverse/transloco-locale';
import { PageHeader } from '@components/page-header/page-header';
import { PageBreadcrumb, PageBreadcrumbItem } from '@components/page-breadcrumb/page-breadcrumb';

interface User {
    id: string;
    email: string;
    instance_role: string;
    created_at: string;
}

interface Site {
    id: string;
    domain: string;
    user_id: string;
    created_at: string;
}

@Component({
    selector: 'app-admin-settings',
    standalone: true,
    imports: [CommonModule, ReactiveFormsModule, TableModule, ButtonModule, SelectModule, CardModule, TabsModule, PageHeader, PageBreadcrumb, TranslocoPipe, TranslocoDatePipe],
    templateUrl: './admin-settings.html',
    styleUrl: './admin-settings.css'
})
export class AdminSettings implements OnInit {
    private http = inject(HttpClient);
    private transloco = inject(TranslocoService);
    private activeLanguage = toSignal(this.transloco.langChanges$, { initialValue: this.transloco.getActiveLang() });

    protected users = signal<User[]>([]);
    protected sites = signal<Site[]>([]);
    protected isLoading = signal(false);
    protected isLoadingSites = signal(false);
    protected currentUserId = signal<string>('');
    protected roleControls = signal<Record<string, FormControl<string>>>({});
    protected readonly breadcrumbItems = computed<PageBreadcrumbItem[]>(() => {
        this.activeLanguage();
        return [{ label: this.transloco.translate('admin.breadcrumb'), isCurrent: true }];
    });

    protected roleOptions = computed(() => {
        this.activeLanguage();
        return [
            { label: this.transloco.translate('admin.roles.instanceOwner'), value: 'owner' },
            { label: this.transloco.translate('admin.roles.instanceAdmin'), value: 'admin' },
            { label: this.transloco.translate('admin.roles.user'), value: 'user' }
        ];
    });

    constructor() {
        effect(() => {
            const currentId = this.currentUserId();
            const users = this.users();
            const controls = this.roleControls();

            for (const user of users) {
                const control = controls[user.id];
                if (!control) continue;

                const shouldDisable = user.id === currentId;
                if (shouldDisable && control.enabled) {
                    control.disable({ emitEvent: false });
                } else if (!shouldDisable && control.disabled) {
                    control.enable({ emitEvent: false });
                }
            }
        });
    }

    ngOnInit() {
        this.loadUsers();
        this.loadSites();
    }

    loadUsers() {
        this.isLoading.set(true);
        this.http.get<User[]>('/api/admin/users').subscribe({
            next: (users) => {
                this.users.set(users);
                this.roleControls.set(
                    users.reduce<Record<string, FormControl<string>>>((controls, user) => {
                        controls[user.id] = new FormControl(
                            {
                                value: user.instance_role,
                                disabled: user.id === this.currentUserId()
                            },
                            { nonNullable: true }
                        );
                        return controls;
                    }, {})
                );
                this.isLoading.set(false);
            },
            error: (err) => {
                console.error('Failed to load users', err);
                this.isLoading.set(false);
            }
        });
    }

    loadSites() {
        this.isLoadingSites.set(true);
        this.http.get<Site[]>('/api/admin/sites').subscribe({
            next: (sites) => {
                this.sites.set(sites);
                this.isLoadingSites.set(false);
            },
            error: (err) => {
                console.error('Failed to load sites', err);
                this.isLoadingSites.set(false);
            }
        });
    }

    updateUserRole(user: User) {
        this.http
            .post(`/api/admin/users/${user.id}/role`, {
                role: user.instance_role
            })
            .subscribe({
                next: () => console.log('Role updated'),
                error: (err) => console.error('Failed to update role', err)
            });
    }

    protected roleControl(userId: string): FormControl<string> {
        const existing = this.roleControls()[userId];
        if (existing) {
            return existing;
        }

        const fallback = new FormControl('', { nonNullable: true });
        this.roleControls.update((controls) => ({ ...controls, [userId]: fallback }));
        return fallback;
    }

    protected onRoleChange(user: User, role: string | null | undefined): void {
        if (!role || role === user.instance_role) {
            return;
        }

        user.instance_role = role;
        this.updateUserRole(user);
    }

    deleteUser(user: User) {
        const message = this.transloco.translate('admin.confirmDeleteUser', { email: user.email });
        if (!confirm(message)) {
            return;
        }
        this.http.delete(`/api/admin/users/${user.id}`).subscribe({
            next: () => this.loadUsers(),
            error: (err) => console.error('Failed to delete user', err)
        });
    }

    deleteSite(site: Site) {
        const message = this.transloco.translate('admin.confirmDeleteSite', { domain: site.domain });
        if (confirm(message)) {
            this.http.delete(`/api/admin/sites/${site.id}`).subscribe({
                next: () => this.loadSites(),
                error: (err) => console.error('Failed to delete site', err)
            });
        }
    }
}
