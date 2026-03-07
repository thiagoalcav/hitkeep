import { ChangeDetectionStrategy, Component, computed, effect, inject, input, signal } from "@angular/core";
import { toSignal } from "@angular/core/rxjs-interop";
import { HttpClient, HttpErrorResponse } from "@angular/common/http";
import { FormControl, ReactiveFormsModule, Validators } from "@angular/forms";
import { compatForm } from "@angular/forms/signals/compat";
import { TranslocoPipe, TranslocoService } from "@jsverse/transloco";
import { RelativeDateTime } from "@components/relative-date-time/relative-date-time";
import { Site } from "@models/analytics.types";
import { TeamService } from "@services/team.service";
import { SiteService } from "@features/sites/services/site.service";
import { ConfirmationService } from "primeng/api";
import { ButtonModule } from "primeng/button";
import { ConfirmPopupModule } from "primeng/confirmpopup";
import { InputTextModule } from "primeng/inputtext";
import { MessageModule } from "primeng/message";
import { SelectModule } from "primeng/select";
import { TableModule } from "primeng/table";

interface SiteMember {
    id: string;
    user_id: string;
    email: string;
    role: string;
    added_at: string;
}

@Component({
    selector: "app-site-team-settings",
    imports: [ReactiveFormsModule, ConfirmPopupModule, TableModule, ButtonModule, SelectModule, InputTextModule, MessageModule, RelativeDateTime, TranslocoPipe],
    providers: [ConfirmationService],
    changeDetection: ChangeDetectionStrategy.OnPush,
    template: `
        <p-confirmpopup key="site-member-remove" />
        <div class="flex flex-col gap-4">
            @if (availableTransferTeams().length) {
                <div class="rounded-xl border border-surface-200 bg-surface-50 p-4 flex flex-col gap-3">
                    <div>
                        <h3 class="text-base font-semibold">{{ "sites.team.transfer.title" | transloco }}</h3>
                        <p class="text-sm text-muted-color">{{ "sites.team.transfer.description" | transloco }}</p>
                    </div>
                    @if (transferSuccessKey(); as key) {
                        <p-message severity="success" [text]="key | transloco" />
                    }
                    @if (transferErrorKey(); as key) {
                        <p-message severity="error" [text]="key | transloco" />
                    }
                    <div class="flex flex-col md:flex-row gap-3 md:items-end">
                        <div class="flex-1">
                            <label for="site-transfer-team" class="text-sm font-medium mb-2 block">{{ "sites.team.transfer.teamLabel" | transloco }}</label>
                            <p-select
                                inputId="site-transfer-team"
                                [options]="availableTransferTeams()"
                                [formControl]="transferForm.teamId().control()"
                                optionLabel="label"
                                optionValue="value"
                                [placeholder]="'sites.team.transfer.teamPlaceholder' | transloco"
                                class="w-full"
                            />
                        </div>

                        <p-button
                            [label]="'sites.team.transfer.action' | transloco"
                            icon="pi pi-arrow-right-arrow-left"
                            [loading]="isTransferring()"
                            [disabled]="isTransferring() || transferForm().invalid()"
                            (onClick)="transferSite()"
                        />
                    </div>
                </div>
            }

            <div class="flex items-end gap-2">
                <div class="flex-1">
                    <label for="member-email" class="text-sm font-medium mb-2 block">{{ "common.emailAddress" | transloco }}</label>
                    <input id="member-email" pInputText [formControl]="memberForm.email().control()" [placeholder]="'sites.team.emailPlaceholder' | transloco" class="w-full" />
                </div>

                <div class="w-40">
                    <label for="member-role" class="text-sm font-medium mb-2 block">{{ "common.columns.role" | transloco }}</label>
                    <p-select inputId="member-role" [options]="roleOptions()" [formControl]="memberForm.role().control()" optionLabel="label" optionValue="value" class="w-full" />
                </div>

                <p-button [label]="'sites.team.addMemberAction' | transloco" icon="pi pi-plus" (onClick)="addMember()" [loading]="isAdding()" [disabled]="isAdding() || memberForm().invalid()" />
            </div>

            <div class="flex justify-end">
                <span class="p-input-icon-left hk-crud-search">
                    <i class="pi pi-search"></i>
                    <input pInputText #memberSearch [placeholder]="'common.searchPlaceholder' | transloco" (input)="membersTable.filterGlobal($any($event.target).value, 'contains')" class="w-full" />
                </span>
            </div>

            <div class="hk-crud-table-wrap">
                <p-table #membersTable [value]="members()" [loading]="isLoading()" [globalFilterFields]="['email', 'role', 'added_at']" [sortField]="'added_at'" [sortOrder]="-1" styleClass="hk-crud-table p-datatable-sm">
                    <ng-template pTemplate="header">
                        <tr>
                            <th pSortableColumn="email">
                                {{ "common.columns.email" | transloco }}
                                <p-sortIcon field="email" />
                            </th>
                            <th pSortableColumn="role">
                                {{ "common.columns.role" | transloco }}
                                <p-sortIcon field="role" />
                            </th>
                            <th pSortableColumn="added_at">
                                {{ "common.columns.added" | transloco }}
                                <p-sortIcon field="added_at" />
                            </th>
                            <th>{{ "common.columns.actions" | transloco }}</th>
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
                            <td><app-relative-date-time [value]="member.added_at" /></td>
                            <td>
                                <p-button icon="pi pi-trash" severity="danger" [text]="true" (onClick)="confirmRemoveMember($event, member)" />
                            </td>
                        </tr>
                    </ng-template>
                </p-table>
            </div>
        </div>
    `
})
export class SiteTeamSettings {
    private http = inject(HttpClient);
    private confirmationService = inject(ConfirmationService);
    private transloco = inject(TranslocoService);
    private teamService = inject(TeamService);
    private siteService = inject(SiteService);
    private activeLanguage = toSignal(this.transloco.langChanges$, { initialValue: this.transloco.getActiveLang() });

    site = input.required<Site | null>();

    protected members = signal<SiteMember[]>([]);
    protected isLoading = signal(false);
    protected isAdding = signal(false);
    protected isTransferring = signal(false);
    protected transferErrorKey = signal<string | null>(null);
    protected transferSuccessKey = signal<string | null>(null);

    private readonly memberFormModel = signal({
        email: new FormControl("", { nonNullable: true, validators: [Validators.required, Validators.email] }),
        role: new FormControl("viewer", { nonNullable: true, validators: [Validators.required] })
    });
    protected readonly memberForm = compatForm(this.memberFormModel);
    private readonly transferFormModel = signal({
        teamId: new FormControl("", { nonNullable: true, validators: [Validators.required] })
    });
    protected readonly transferForm = compatForm(this.transferFormModel);

    protected roleOptions = computed(() => {
        this.activeLanguage();
        return [
            { label: this.transloco.translate("roles.owner"), value: "owner" },
            { label: this.transloco.translate("roles.admin"), value: "admin" },
            { label: this.transloco.translate("roles.editor"), value: "editor" },
            { label: this.transloco.translate("roles.viewer"), value: "viewer" }
        ];
    });
    protected readonly availableTransferTeams = computed(() => {
        this.activeLanguage();
        const currentTeamId = this.teamService.activeTeamId();
        return this.teamService
            .teams()
            .filter((team) => team.id !== currentTeamId && (team.role === "owner" || team.role === "admin"))
            .map((team) => ({
                label: team.name,
                value: team.id
            }));
    });

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
                console.error("Failed to load members", err);
                this.isLoading.set(false);
            }
        });
    }

    addMember() {
        const siteId = this.site()?.id;
        const email = this.memberForm.email().value().trim();
        const role = this.memberForm.role().value();
        if (!siteId || !email) return;

        this.isAdding.set(true);
        this.http
            .post(`/api/sites/${siteId}/members`, {
                email,
                role
            })
            .subscribe({
                next: () => {
                    this.memberForm.email().control().reset("");
                    this.isAdding.set(false);
                    this.loadMembers(siteId);
                },
                error: (err) => {
                    console.error("Failed to add member", err);
                    this.isAdding.set(false);
                    alert(this.transloco.translate("sites.team.errors.addFailed"));
                }
            });
    }

    transferSite() {
        const siteId = this.site()?.id;
        const teamId = this.transferForm.teamId().value().trim();
        if (!siteId || !teamId) return;

        this.isTransferring.set(true);
        this.transferErrorKey.set(null);
        this.transferSuccessKey.set(null);

        this.http
            .post(`/api/sites/${siteId}/transfer-team`, {
                team_id: teamId
            })
            .subscribe({
                next: () => {
                    this.transferForm.teamId().control().reset("");
                    this.transferSuccessKey.set("sites.team.transfer.success");
                    this.siteService.sites.update((sites) => sites.filter((site) => site.id !== siteId));
                    if (this.siteService.activeSite()?.id === siteId) {
                        this.siteService.activeSite.set(null);
                    }
                    this.siteService.loadSites();
                    this.isTransferring.set(false);
                },
                error: (error: unknown) => {
                    if (error instanceof HttpErrorResponse && error.status === 403) {
                        this.transferErrorKey.set("sites.team.transfer.errors.forbidden");
                    } else {
                        this.transferErrorKey.set("sites.team.transfer.errors.generic");
                    }
                    this.isTransferring.set(false);
                }
            });
    }

    confirmRemoveMember(event: Event, member: SiteMember) {
        const siteId = this.site()?.id;
        if (!siteId) return;

        this.confirmationService.confirm({
            key: "site-member-remove",
            target: event.currentTarget as EventTarget,
            message: this.transloco.translate("sites.team.confirmRemove", { email: member.email }),
            icon: "pi pi-exclamation-triangle",
            rejectButtonProps: {
                label: this.transloco.translate("common.actions.cancel"),
                severity: "secondary",
                outlined: true
            },
            acceptButtonProps: {
                label: this.transloco.translate("teams.management.removeAction"),
                severity: "danger"
            },
            accept: () => {
                this.http.delete(`/api/sites/${siteId}/members/${member.user_id}`).subscribe({
                    next: () => this.loadMembers(siteId),
                    error: (err) => {
                        console.error("Failed to remove member", err);
                    }
                });
            }
        });
    }

    getRoleLabel(role: string): string {
        return this.roleOptions().find((r) => r.value === role)?.label || role;
    }

    getRoleBadgeClass(role: string): string {
        switch (role) {
            case "owner":
                return "bg-red-100 text-red-700";
            case "admin":
                return "bg-purple-100 text-purple-700";
            case "editor":
                return "bg-blue-100 text-blue-700";
            case "viewer":
                return "bg-gray-100 text-gray-700";
            default:
                return "bg-gray-100 text-gray-700";
        }
    }
}
