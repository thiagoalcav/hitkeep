import { ChangeDetectionStrategy, Component, computed, effect, inject, OnInit, signal } from "@angular/core";
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
import { TagModule } from "primeng/tag";
import { HttpClient } from "@angular/common/http";
import { HttpErrorResponse } from "@angular/common/http";
import { TranslocoPipe, TranslocoService } from "@jsverse/transloco";
import { PageHeader, PageHeaderLeft } from "@components/page-header/page-header";
import { PageBreadcrumb, PageBreadcrumbItem } from "@components/page-breadcrumb/page-breadcrumb";
import { RelativeDateTime } from "@components/relative-date-time/relative-date-time";
import { PermissionService } from "@services/permission.service";
import { UserProfileService } from "@services/user-profile.service";
import { AdminGlobalExclusionSettings } from "./components/admin-global-exclusion-settings";
import { finalize } from "rxjs";

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

interface DisableUserMFAResponse {
    status: string;
    totp_disabled: boolean;
    passkeys_deleted: number;
    sessions_invalidated: number;
}

interface AdminTeam {
    id: string;
    name: string;
    is_default: boolean;
    is_archived: boolean;
    member_count: number;
    site_count: number;
    created_at: string;
}

interface StatusState {
    severity: "success" | "error";
    key: string;
    params?: Record<string, string | number>;
}

@Component({
    selector: "app-admin-settings",
    imports: [
        ReactiveFormsModule,
        ConfirmPopupModule,
        TableModule,
        ButtonModule,
        SelectModule,
        CardModule,
        TabsModule,
        InputTextModule,
        MessageModule,
        TagModule,
        PageHeader,
        PageHeaderLeft,
        PageBreadcrumb,
        AdminGlobalExclusionSettings,
        RelativeDateTime,
        TranslocoPipe
    ],
    templateUrl: "./admin-settings.html",
    styleUrl: "./admin-settings.css",
    changeDetection: ChangeDetectionStrategy.OnPush,
    providers: [ConfirmationService]
})
export class AdminSettings implements OnInit {
    private http = inject(HttpClient);
    private confirmationService = inject(ConfirmationService);
    private transloco = inject(TranslocoService);
    private profile = inject(UserProfileService);
    private perms = inject(PermissionService);
    private activeLanguage = toSignal(this.transloco.langChanges$, { initialValue: this.transloco.getActiveLang() });

    protected users = signal<User[]>([]);
    protected sites = signal<Site[]>([]);
    protected teams = signal<AdminTeam[]>([]);
    protected isLoading = signal(false);
    protected isLoadingSites = signal(false);
    protected isLoadingTeams = signal(false);
    protected disablingUserId = signal("");
    protected currentUserId = signal<string>("");
    protected roleControls = signal<Record<string, FormControl<InstanceRole>>>({});
    protected deleteUserBlock = signal<DeleteUserBlockState | null>(null);
    protected userMfaStatus = signal<StatusState | null>(null);
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
    protected readonly userMfaStatusMessage = computed(() => {
        const state = this.userMfaStatus();
        this.activeLanguage();
        if (!state) {
            return "";
        }

        return this.transloco.translate(state.key, state.params);
    });
    protected readonly breadcrumbItems = computed<PageBreadcrumbItem[]>(() => {
        this.activeLanguage();
        return [{ label: this.transloco.translate("nav.administration") }, { label: this.transloco.translate("nav.systemSettings"), isCurrent: true }];
    });
    protected readonly canDisableUserMfa = computed(() => this.perms.isInstanceOwner() || this.usersByID().get(this.currentUserId())?.instance_role === "owner");

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
        this.loadTeams();
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

    protected isDisablingUser(user: User): boolean {
        return this.disablingUserId() === user.id;
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

    protected confirmDisableUserMfa(event: Event, user: User): void {
        const target = event.currentTarget;
        if (!(target instanceof HTMLElement) || !this.canDisableUserMfa() || this.isDisablingUser(user)) {
            return;
        }

        this.confirmationService.confirm({
            key: "admin-disable-mfa",
            target,
            message: this.transloco.translate("admin.confirmDisable2fa", { email: user.email }),
            icon: "pi pi-exclamation-triangle",
            rejectButtonProps: {
                label: this.transloco.translate("common.actions.cancel"),
                severity: "secondary",
                outlined: true
            },
            acceptButtonProps: {
                label: this.transloco.translate("admin.users.disable2faAction"),
                severity: "warn"
            },
            accept: () => {
                this.userMfaStatus.set(null);
                this.disablingUserId.set(user.id);
                this.http
                    .post<DisableUserMFAResponse>(`/api/admin/users/${user.id}/disable-2fa`, {})
                    .pipe(finalize(() => this.disablingUserId.set("")))
                    .subscribe({
                        next: () => {
                            this.userMfaStatus.set({
                                severity: "success",
                                key: "admin.status.disable2faSuccess",
                                params: { email: user.email }
                            });
                        },
                        error: () => {
                            this.userMfaStatus.set({
                                severity: "error",
                                key: "admin.errors.disable2faFailed",
                                params: { email: user.email }
                            });
                        }
                    });
            }
        });
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
                this.http.delete(`/api/admin/users/${user.id}?force=true`).subscribe({
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

    loadTeams() {
        this.isLoadingTeams.set(true);
        this.http.get<AdminTeam[]>("/api/admin/teams").subscribe({
            next: (teams) => {
                this.teams.set(teams);
                this.isLoadingTeams.set(false);
            },
            error: (err) => {
                console.error("Failed to load teams", err);
                this.isLoadingTeams.set(false);
            }
        });
    }

    confirmDeleteTeam(event: Event, team: AdminTeam) {
        const messageKey = team.site_count > 0 ? "admin.confirmDeleteTeamWithSites" : "admin.confirmDeleteTeam";

        this.confirmationService.confirm({
            key: "admin-delete",
            target: event.currentTarget as EventTarget,
            message: this.transloco.translate(messageKey, { name: team.name, sites: team.site_count }),
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
                this.http.delete(`/api/admin/teams/${team.id}?force=true`).subscribe({
                    next: () => {
                        this.loadTeams();
                        this.loadSites();
                    },
                    error: (err) => console.error("Failed to delete team", err)
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
