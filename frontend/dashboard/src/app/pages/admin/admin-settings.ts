import { Component, inject, signal, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { TableModule } from 'primeng/table';
import { ButtonModule } from 'primeng/button';
import { SelectModule } from 'primeng/select';
import { CardModule } from 'primeng/card';
import { TabsModule } from 'primeng/tabs';
import { HttpClient } from '@angular/common/http';

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
  imports: [CommonModule, FormsModule, TableModule, ButtonModule, SelectModule, CardModule, TabsModule],
  template: `
    <div class="max-w-6xl mx-auto p-6">
      <h1 class="text-2xl font-bold mb-6">Instance Administration</h1>
      
      <p-tabs value="0">
        <p-tablist>
            <p-tab value="0">Users</p-tab>
            <p-tab value="1">Sites</p-tab>
        </p-tablist>
        <p-tabpanels>
            <p-tabpanel value="0">
                <p-card>
                    <h2 class="text-lg font-semibold mb-4">Users</h2>
                    
                    <p-table [value]="users()" [loading]="isLoading()">
                    <ng-template pTemplate="header">
                        <tr>
                        <th>Email</th>
                        <th>Role</th>
                        <th>Created</th>
                        <th>Actions</th>
                        </tr>
                    </ng-template>
                    
                    <ng-template pTemplate="body" let-user>
                        <tr>
                        <td>{{ user.email }}</td>
                        <td>
                            <p-select
                            [options]="roleOptions"
                            [(ngModel)]="user.instance_role"
                            (onChange)="updateUserRole(user)"
                            optionLabel="label"
                            optionValue="value"
                            [disabled]="user.id === currentUserId()"
                            class="w-40" />
                        </td>
                        <td>{{ user.created_at | date:'short' }}</td>
                        <td>
                            <p-button
                            icon="pi pi-trash"
                            severity="danger"
                            [text]="true"
                            (onClick)="deleteUser(user)"
                            [disabled]="user.id === currentUserId()" />
                        </td>
                        </tr>
                    </ng-template>
                    </p-table>
                </p-card>
            </p-tabpanel>
            <p-tabpanel value="1">
                <p-card>
                    <h2 class="text-lg font-semibold mb-4">Sites</h2>
                    
                    <p-table [value]="sites()" [loading]="isLoadingSites()">
                    <ng-template pTemplate="header">
                        <tr>
                        <th>Domain</th>
                        <th>Created</th>
                        <th>Actions</th>
                        </tr>
                    </ng-template>
                    
                    <ng-template pTemplate="body" let-site>
                        <tr>
                        <td>
                            <a [href]="'https://' + site.domain" target="_blank" class="text-primary hover:underline">
                                {{ site.domain }}
                            </a>
                        </td>
                        <td>{{ site.created_at | date:'short' }}</td>
                        <td>
                            <p-button
                            icon="pi pi-trash"
                            severity="danger"
                            [text]="true"
                            (onClick)="deleteSite(site)" />
                        </td>
                        </tr>
                    </ng-template>
                    </p-table>
                </p-card>
            </p-tabpanel>
        </p-tabpanels>
      </p-tabs>
    </div>
  `
})
export class AdminSettings implements OnInit {
  private http = inject(HttpClient);
  
  protected users = signal<User[]>([]);
  protected sites = signal<Site[]>([]);
  protected isLoading = signal(false);
  protected isLoadingSites = signal(false);
  protected currentUserId = signal<string>('');
  
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
    this.http.post(`/api/admin/users/${user.id}/role`, {
      role: user.instance_role
    }).subscribe({
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