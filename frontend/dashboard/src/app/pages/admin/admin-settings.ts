import { Component, inject, signal, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { TableModule } from 'primeng/table';
import { ButtonModule } from 'primeng/button';
import { SelectModule } from 'primeng/select';
import { CardModule } from 'primeng/card';
import { TabsModule } from 'primeng/tabs';
import { HttpClient } from '@angular/common/http';
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
    imports: [CommonModule, FormsModule, TableModule, ButtonModule, SelectModule, CardModule, TabsModule, PageHeader, PageBreadcrumb],
    templateUrl: './admin-settings.html',
    styleUrl: './admin-settings.css'
})
export class AdminSettings implements OnInit {
    private http = inject(HttpClient);

    protected users = signal<User[]>([]);
    protected sites = signal<Site[]>([]);
    protected isLoading = signal(false);
    protected isLoadingSites = signal(false);
    protected currentUserId = signal<string>('');
    protected readonly breadcrumbItems: PageBreadcrumbItem[] = [{ label: 'Instance Administration', isCurrent: true }];

    protected roleOptions = [
        { label: 'Instance Owner', value: 'owner' },
        { label: 'Instance Admin', value: 'admin' },
        { label: 'User', value: 'user' }
    ];

    ngOnInit() {
        this.loadUsers();
        this.loadSites();
    }

    loadUsers() {
        this.isLoading.set(true);
        this.http.get<User[]>('/api/admin/users').subscribe({
            next: (users) => {
                this.users.set(users);
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

    deleteUser(user: User) {
        if (confirm(`Delete user ${user.email}?`)) {
            this.http.delete(`/api/admin/users/${user.id}`).subscribe({
                next: () => this.loadUsers(),
                error: (err) => console.error('Failed to delete user', err)
            });
        }
    }

    deleteSite(site: Site) {
        if (confirm(`Delete site ${site.domain}? This action cannot be undone.`)) {
            this.http.delete(`/api/admin/sites/${site.id}`).subscribe({
                next: () => this.loadSites(),
                error: (err) => console.error('Failed to delete site', err)
            });
        }
    }
}
