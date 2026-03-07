import { Component, computed, effect, inject, OnInit, signal } from "@angular/core";
import { toSignal } from "@angular/core/rxjs-interop";

import { FormControl, ReactiveFormsModule } from "@angular/forms";
import { ConfirmationService } from "primeng/api";
import { ConfirmPopupModule } from "primeng/confirmpopup";
import { TableModule } from "primeng/table";
import { ButtonModule } from "primeng/button";
import { SelectModule } from "primeng/select";
import { CardModule } from "primeng/card";
import { TabsModule } from "primeng/tabs";
import { InputTextModule } from "primeng/inputtext";
import { MessageModule } from "primeng/message";
import { HttpClient } from "@angular/common/http";
import { HttpErrorResponse } from "@angular/common/http";
import { TranslocoPipe, TranslocoService } from "@jsverse/transloco";
import { PageHeader } from "@components/page-header/page-header";
import { PageBreadcrumb, PageBreadcrumbItem } from "@components/page-breadcrumb/page-breadcrumb";
import { RelativeDateTime } from "@components/relative-date-time/relative-date-time";
import { UserProfileService } from "@services/user-profile.service";
import { AdminGlobalExclusionSettings } from "./components/admin-global-exclusion-settings";

type InstanceRole = "owner" | "admin" | "user";

interface User {
    id: string;
    email: string;
    instance_role: InstanceRole;
    created_at: string;
}

interface Site {
    id: string;
    domain: string;
    user_id: string;
    owner_email?: string;
    created_at: string;
}

interface DeleteUserBlockedResponse {
    status: string;
    code: string;
    message: string;
    teams: {
        id: string;
        name: string;
    }[];
}

interface DeleteUserBlockState {
    email: string;
    teams: string[];
}

@Component({
    selector: "app-admin-settings",
    standalone: true,
    imports: [ReactiveFormsModule, ConfirmPopupModule, TableModule, ButtonModule, SelectModule, CardModule, TabsModule, InputTextModule, MessageModule, PageHeader, PageBreadcrumb, AdminGlobalExclusionSettings, RelativeDateTime, TranslocoPipe],
    templateUrl: "./admin-settings.html",
    styleUrl: "./admin-settings.css",
    providers: [ConfirmationService]
})
export class AdminSettings implements OnInit {
    private http = inject(HttpClient);
    private confirmationService = inject(ConfirmationService);
    private transloco = inject(TranslocoService);
    private profile = inject(UserProfileService);
    private activeLanguage = toSignal(this.transloco.langChanges$, { initialValue: this.transloco.getActiveLang() });

    protected users = signal<User[]>([]);
    protected sites = signal<Site[]>([]);
    protected isLoading = signal(false);
    protected isLoadingSites = signal(false);
    protected currentUserId = signal<string>("");
    protected roleControls = signal<Record<string, FormControl<InstanceRole>>>({});
    protected deleteUserBlock = signal<DeleteUserBlockState | null>(null);
    protected readonly usersByID = computed(() => new Map(this.users().map((user) => [user.id, user] as const)));
    protected readonly deleteUserBlockMessage = computed(() => {
        const block = this.deleteUserBlock();
        if (!block) {
            return "";
        }

        return this.transloco.translate("admin.errors.deleteUserBlockedOwnership", {
            email: block.email,
            teams: block.teams.join(", ")
        });
    });
    protected readonly breadcrumbItems = computed<PageBreadcrumbItem[]>(() => {
        this.activeLanguage();
        return [{ label: this.transloco.translate("nav.administration") }, { label: this.transloco.translate("nav.systemSettings"), isCurrent: true }];
    });

    protected roleOptions = computed(() => {
        this.activeLanguage();
        return [
            { label: this.transloco.translate("admin.roles.instanceOwner"), value: "owner" },
            { label: this.transloco.translate("admin.roles.instanceAdmin"), value: "admin" },
            { label: this.transloco.translate("admin.roles.user"), value: "user" }
        ];
    });

    constructor() {
        effect(() => {
            this.currentUserId.set(this.profile.profile()?.id ?? "");
        });

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
        if (!this.profile.profile()) {
            this.profile.loadProfile().subscribe({ error: (err) => console.error("Failed to load profile", err) });
        }

        this.loadUsers();
        this.loadSites();
    }

    loadUsers() {
        this.isLoading.set(true);
        this.http.get<User[]>("/api/admin/users").subscribe({
            next: (users) => {
                this.deleteUserBlock.update((current) => {
                    if (!current) {
                        return null;
                    }
                    const stillExists = users.some((user) => user.email === current.email);
                    return stillExists ? current : null;
                });
                const normalizedUsers = users.map((user) => ({
                    ...user,
                    instance_role: this.normalizeInstanceRole(user.instance_role)
                }));
                this.users.set(normalizedUsers);
                this.roleControls.set(
                    normalizedUsers.reduce<Record<string, FormControl<InstanceRole>>>((controls, user) => {
                        controls[user.id] = new FormControl<InstanceRole>(
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
                console.error("Failed to load users", err);
                this.isLoading.set(false);
            }
        });
    }

    loadSites() {
        this.isLoadingSites.set(true);
        this.http.get<Site[]>("/api/admin/sites").subscribe({
            next: (sites) => {
                this.sites.set(
                    sites.map((site) => ({
                        ...site,
                        owner_email: (site.owner_email ?? "").trim()
                    }))
                );
                this.isLoadingSites.set(false);
            },
            error: (err) => {
                console.error("Failed to load sites", err);
                this.isLoadingSites.set(false);
            }
        });
    }

    private updateUserRole(user: User, nextRole: InstanceRole, previousRole: InstanceRole): void {
        this.http
            .post(`/api/admin/users/${user.id}/role`, {
                role: nextRole
            })
            .subscribe({
                next: () => this.roleControl(user.id).setValue(nextRole, { emitEvent: false }),
                error: (err) => {
                    user.instance_role = previousRole;
                    this.roleControl(user.id).setValue(previousRole, { emitEvent: false });
                    console.error("Failed to update role", err);
                }
            });
    }

    protected roleControl(userId: string): FormControl<InstanceRole> {
        const existing = this.roleControls()[userId];
        if (existing) {
            return existing;
        }

        const fallback = new FormControl<InstanceRole>("user", { nonNullable: true });
        this.roleControls.update((controls) => ({ ...controls, [userId]: fallback }));
        return fallback;
    }

    protected onRoleChange(user: User, role: InstanceRole | null | undefined): void {
        if (!role || role === user.instance_role) {
            return;
        }

        const previousRole = user.instance_role;
        user.instance_role = role;
        this.updateUserRole(user, role, previousRole);
    }

    protected isInstanceOwner(user: User): boolean {
        return user.instance_role === "owner";
    }

    protected siteOwnerEmail(site: Site): string {
        return site.owner_email || this.usersByID().get(site.user_id)?.email || this.transloco.translate("admin.sites.ownerUnknown");
    }

    protected siteOwnerInstanceRole(site: Site): InstanceRole | null {
        return this.usersByID().get(site.user_id)?.instance_role ?? null;
    }

    protected roleLabel(role: InstanceRole): string {
        return this.roleOptions().find((entry) => entry.value === role)?.label ?? role;
    }

    private normalizeInstanceRole(role: string | null | undefined): InstanceRole {
        if (role === "owner" || role === "admin" || role === "user") {
            return role;
        }
        return "user";
    }

    confirmDeleteUser(event: Event, user: User) {
        this.confirmationService.confirm({
            key: "admin-delete",
            target: event.currentTarget as EventTarget,
            message: this.transloco.translate("admin.confirmDeleteUser", { email: user.email }),
            icon: "pi pi-exclamation-triangle",
            rejectButtonProps: {
                label: this.transloco.translate("common.actions.cancel"),
                severity: "secondary",
                outlined: true
            },
            acceptButtonProps: {
                label: this.transloco.translate("share.dialog.deleteAction"),
                severity: "danger"
            },
            accept: () => {
                this.deleteUserBlock.set(null);
                this.http.delete(`/api/admin/users/${user.id}`).subscribe({
                    next: () => this.loadUsers(),
                    error: (err) => {
                        if (this.handleDeleteUserError(err, user)) {
                            return;
                        }
                        console.error("Failed to delete user", err);
                    }
                });
            }
        });
    }

    confirmDeleteSite(event: Event, site: Site) {
        this.confirmationService.confirm({
            key: "admin-delete",
            target: event.currentTarget as EventTarget,
            message: this.transloco.translate("admin.confirmDeleteSite", { domain: site.domain }),
            icon: "pi pi-exclamation-triangle",
            rejectButtonProps: {
                label: this.transloco.translate("common.actions.cancel"),
                severity: "secondary",
                outlined: true
            },
            acceptButtonProps: {
                label: this.transloco.translate("share.dialog.deleteAction"),
                severity: "danger"
            },
            accept: () => {
                this.http.delete(`/api/admin/sites/${site.id}`).subscribe({
                    next: () => this.loadSites(),
                    error: (err) => console.error("Failed to delete site", err)
                });
            }
        });
    }

    private handleDeleteUserError(err: unknown, user: User): boolean {
        const httpErr = err instanceof HttpErrorResponse ? err : null;
        const response = httpErr?.error as DeleteUserBlockedResponse | undefined;
        if (!response || response.code !== "user_owns_teams" || !Array.isArray(response.teams)) {
            return false;
        }

        const teamNames = response.teams.map((team) => team?.name?.trim()).filter((name): name is string => !!name);

        if (teamNames.length === 0) {
            return false;
        }

        this.deleteUserBlock.set({
            email: user.email,
            teams: teamNames
        });
        return true;
    }
}
