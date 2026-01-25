import { Component, inject, input, signal, effect } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { HttpClient } from '@angular/common/http';
import { TableModule } from 'primeng/table';
import { ButtonModule } from 'primeng/button';
import { SelectModule } from 'primeng/select';
import { InputTextModule } from 'primeng/inputtext';
import { Site } from '@models/analytics.types';

interface SiteMember {
    id: string;
    user_id: string;
    email: string;
    role: string;
    added_at: string;
}

@Component({
    selector: 'app-site-team-settings',
    standalone: true,
    imports: [CommonModule, FormsModule, TableModule, ButtonModule, SelectModule, InputTextModule],
    template: `
        <div class="flex flex-col gap-4">
            <div class="flex items-end gap-2">
                <div class="flex-1">
                    <label for="member-email" class="text-sm font-medium mb-2 block">Email Address</label>
                    <input id="member-email" pInputText [(ngModel)]="newMemberEmail" placeholder="user@example.com" class="w-full" />
                </div>

                <div class="w-40">
                    <label for="member-role" class="text-sm font-medium mb-2 block">Role</label>
                    <p-select inputId="member-role" [options]="roleOptions" [(ngModel)]="newMemberRole" optionLabel="label" optionValue="value" class="w-full" />
                </div>

                <p-button label="Add Member" icon="pi pi-plus" (onClick)="addMember()" [loading]="isAdding()" />
            </div>

            <p-table [value]="members()" [loading]="isLoading()">
                <ng-template pTemplate="header">
                    <tr>
                        <th>Email</th>
                        <th>Role</th>
                        <th>Added</th>
                        <th>Actions</th>
                    </tr>
                </ng-template>

                <ng-template pTemplate="body" let-member>
                    <tr>
                        <td>{{ member.email }}</td>
                        <td>
                            <span class="px-2 py-1 rounded text-xs font-medium" [class]="getRoleBadgeClass(member.role)">
                                {{ getRoleLabel(member.role) }}
                            </span>
                        </td>
                        <td>{{ member.added_at | date: 'short' }}</td>
                        <td>
                            <p-button icon="pi pi-trash" severity="danger" [text]="true" (onClick)="removeMember(member)" />
                        </td>
                    </tr>
                </ng-template>
            </p-table>
        </div>
    `
})
export class SiteTeamSettings {
    // Removed implements OnInit
    private http = inject(HttpClient);

    site = input.required<Site | null>();

    protected members = signal<SiteMember[]>([]);
    protected isLoading = signal(false);
    protected isAdding = signal(false);

    protected newMemberEmail = '';
    protected newMemberRole = 'viewer';

    protected roleOptions = [
        { label: 'Owner', value: 'owner' },
        { label: 'Admin', value: 'admin' },
        { label: 'Editor', value: 'editor' },
        { label: 'Viewer', value: 'viewer' }
    ];

    constructor() {
        // Automatically reload members whenever the 'site' input signal changes
        effect(() => {
            const currentSite = this.site();
            if (currentSite) {
                this.loadMembers(currentSite.id);
            } else {
                this.members.set([]);
            }
        });
    }

    loadMembers(siteId: string) {
        this.isLoading.set(true);
        this.http.get<SiteMember[]>(`/api/sites/${siteId}/members`).subscribe({
            next: (members) => {
                this.members.set(members);
                this.isLoading.set(false);
            },
            error: (err) => {
                console.error('Failed to load members', err);
                this.isLoading.set(false);
            }
        });
    }

    addMember() {
        const siteId = this.site()?.id;
        if (!siteId || !this.newMemberEmail) return;

        this.isAdding.set(true);
        this.http
            .post(`/api/sites/${siteId}/members`, {
                email: this.newMemberEmail,
                role: this.newMemberRole
            })
            .subscribe({
                next: () => {
                    this.newMemberEmail = '';
                    this.isAdding.set(false);
                    this.loadMembers(siteId);
                },
                error: (err) => {
                    console.error('Failed to add member', err);
                    this.isAdding.set(false);
                    alert('Failed to add member. Ensure user exists.');
                }
            });
    }

    removeMember(member: SiteMember) {
        const siteId = this.site()?.id;
        if (!siteId) return;

        if (confirm(`Remove ${member.email} from site?`)) {
            this.http.delete(`/api/sites/${siteId}/members/${member.user_id}`).subscribe({
                next: () => this.loadMembers(siteId),
                error: (err) => {
                    console.error('Failed to remove member', err);
                    alert('Failed to remove member: ' + (err.error || 'Unknown error'));
                }
            });
        }
    }

    getRoleLabel(role: string): string {
        return this.roleOptions.find((r) => r.value === role)?.label || role;
    }

    getRoleBadgeClass(role: string): string {
        switch (role) {
            case 'owner':
                return 'bg-red-100 text-red-700';
            case 'admin':
                return 'bg-purple-100 text-purple-700';
            case 'editor':
                return 'bg-blue-100 text-blue-700';
            case 'viewer':
                return 'bg-gray-100 text-gray-700';
            default:
                return 'bg-gray-100 text-gray-700';
        }
    }
}
