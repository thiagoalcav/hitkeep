import { ChangeDetectionStrategy, Component, computed, inject, OnInit, signal } from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';

import { FormsModule } from '@angular/forms';
import { TableModule } from 'primeng/table';
import { ButtonModule } from 'primeng/button';
import { SelectModule } from 'primeng/select';
import { IconFieldModule } from 'primeng/iconfield';
import { InputIconModule } from 'primeng/inputicon';
import { InputTextModule } from 'primeng/inputtext';
import { TagModule } from 'primeng/tag';
import { DatePickerModule } from 'primeng/datepicker';
import { MessageModule } from 'primeng/message';
import { TranslocoPipe, TranslocoService } from '@jsverse/transloco';
import { RelativeDateTime } from '@components/relative-date-time/relative-date-time';
import { AdminSystemService, InstanceAuditEntry, InstanceAuditListResponse } from '@services/admin-system.service';
import { debounceTime, Subject, switchMap } from 'rxjs';

type AuditOutcome = '' | 'success' | 'failure' | 'denied';

@Component({
    selector: 'app-system-audit',
    imports: [FormsModule, TableModule, ButtonModule, SelectModule, IconFieldModule, InputIconModule, InputTextModule, TagModule, DatePickerModule, MessageModule, RelativeDateTime, TranslocoPipe],
    templateUrl: './system-audit.html',
    styleUrl: './system-audit.css',
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class SystemAudit implements OnInit {
    private system = inject(AdminSystemService);
    private transloco = inject(TranslocoService);
    private activeLanguage = toSignal(this.transloco.langChanges$, { initialValue: this.transloco.getActiveLang() });

    protected entries = signal<InstanceAuditEntry[]>([]);
    protected total = signal(0);
    protected limit = signal(25);
    protected offset = signal(0);
    protected isLoading = signal(false);
    protected hasError = signal(false);

    protected selectedAction = signal('');
    protected selectedOutcome = signal<AuditOutcome>('');
    protected selectedTargetType = signal('');
    protected searchQuery = signal('');
    protected dateRange = signal<Date[]>([]);

    protected readonly actionOptions = computed(() => {
        this.activeLanguage();
        return [
            { label: this.transloco.translate('admin.system.audit.filters.all'), value: '' },
            { label: this.transloco.translate('admin.system.audit.actions.roleUpdated'), value: 'role.updated' },
            { label: this.transloco.translate('admin.system.audit.actions.mfaDisabled'), value: 'mfa.disabled' },
            { label: this.transloco.translate('admin.system.audit.actions.backupTriggered'), value: 'backup.triggered' },
            { label: this.transloco.translate('admin.system.audit.actions.backupCompleted'), value: 'backup.completed' },
            { label: this.transloco.translate('admin.system.audit.actions.backupFailed'), value: 'backup.failed' },
            { label: this.transloco.translate('admin.system.audit.actions.spamRefreshed'), value: 'spam_filter.refresh' },
            { label: this.transloco.translate('admin.system.audit.actions.mailTested'), value: 'mail.test' },
            { label: this.transloco.translate('admin.system.audit.actions.diagnosticsExported'), value: 'diagnostics.export' },
            { label: this.transloco.translate('admin.system.audit.actions.accessDenied'), value: 'access.denied' }
        ];
    });

    protected readonly outcomeOptions = computed(() => {
        this.activeLanguage();
        return [
            { label: this.transloco.translate('admin.system.audit.filters.all'), value: '' },
            { label: this.transloco.translate('admin.system.audit.outcome.success'), value: 'success' },
            { label: this.transloco.translate('admin.system.audit.outcome.failure'), value: 'failure' },
            { label: this.transloco.translate('admin.system.audit.outcome.denied'), value: 'denied' }
        ];
    });

    protected readonly hasMore = computed(() => this.offset() + this.entries().length < this.total());

    private searchSubject = new Subject<void>();

    constructor() {
        this.searchSubject
            .pipe(
                debounceTime(300),
                switchMap(() => {
                    this.offset.set(0);
                    this.isLoading.set(true);
                    this.hasError.set(false);
                    return this.system.listAudit(this.buildFilter());
                })
            )
            .subscribe({
                next: (resp) => this.onData(resp),
                error: () => {
                    this.isLoading.set(false);
                    this.hasError.set(true);
                }
            });
    }

    ngOnInit() {
        this.loadEntries();
    }

    protected loadEntries() {
        this.isLoading.set(true);
        this.hasError.set(false);
        this.system.listAudit(this.buildFilter()).subscribe({
            next: (resp) => this.onData(resp),
            error: () => {
                this.isLoading.set(false);
                this.hasError.set(true);
            }
        });
    }

    protected loadMore() {
        if (this.hasMore()) {
            this.offset.update((o) => o + this.limit());
            this.isLoading.set(true);
            this.system.listAudit(this.buildFilter()).subscribe({
                next: (resp) => {
                    this.entries.update((prev) => [...prev, ...resp.entries]);
                    this.total.set(resp.total);
                    this.isLoading.set(false);
                },
                error: () => {
                    this.isLoading.set(false);
                    this.hasError.set(true);
                }
            });
        }
    }

    protected onFilterChange() {
        this.searchSubject.next();
    }

    protected exportAudit() {
        const filter = this.buildFilter();
        this.system.exportAudit(filter).subscribe({
            next: (blob) => {
                const url = window.URL.createObjectURL(blob as Blob);
                const a = document.createElement('a');
                a.href = url;
                a.download = `instance-audit-${new Date().toISOString().split('T')[0]}.json`;
                a.click();
                window.URL.revokeObjectURL(url);
            }
        });
    }

    protected outcomeSeverity(outcome: string): 'success' | 'danger' | 'warn' | 'secondary' | 'info' | 'contrast' {
        switch (outcome) {
            case 'success':
                return 'success';
            case 'failure':
                return 'danger';
            case 'denied':
                return 'warn';
            default:
                return 'secondary';
        }
    }

    protected auditActionLabel(action: string) {
        this.activeLanguage();
        const labels: Record<string, string> = {
            'role.updated': 'admin.system.audit.actions.roleUpdated',
            'mfa.disabled': 'admin.system.audit.actions.mfaDisabled',
            'backup.triggered': 'admin.system.audit.actions.backupTriggered',
            'backup.completed': 'admin.system.audit.actions.backupCompleted',
            'backup.failed': 'admin.system.audit.actions.backupFailed',
            'spam_filter.refresh': 'admin.system.audit.actions.spamRefreshed',
            'mail.test': 'admin.system.audit.actions.mailTested',
            'diagnostics.export': 'admin.system.audit.actions.diagnosticsExported',
            'access.denied': 'admin.system.audit.actions.accessDenied'
        };
        return labels[action] ? this.transloco.translate(labels[action]) : this.humanizeAuditValue(action);
    }

    protected auditTargetTypeLabel(targetType: string) {
        this.activeLanguage();
        const labels: Record<string, string> = {
            system: 'admin.system.audit.targetTypes.system',
            mail: 'admin.system.audit.targetTypes.mail',
            user: 'admin.system.audit.targetTypes.user',
            team: 'admin.system.audit.targetTypes.team',
            site: 'admin.system.audit.targetTypes.site',
            backup: 'admin.system.audit.targetTypes.backup',
            spam_filter: 'admin.system.audit.targetTypes.spamFilter',
            diagnostics: 'admin.system.audit.targetTypes.diagnostics'
        };
        return labels[targetType] ? this.transloco.translate(labels[targetType]) : this.humanizeAuditValue(targetType);
    }

    protected auditTargetLabel(entry: InstanceAuditEntry) {
        return entry.target_label || entry.target_id || '-';
    }

    protected auditOutcomeLabel(outcome: string) {
        this.activeLanguage();
        const labels: Record<string, string> = {
            success: 'admin.system.audit.outcome.success',
            failure: 'admin.system.audit.outcome.failure',
            denied: 'admin.system.audit.outcome.denied'
        };
        return labels[outcome] ? this.transloco.translate(labels[outcome]) : this.humanizeAuditValue(outcome);
    }

    protected auditRoleLabel(role: string) {
        this.activeLanguage();
        const labels: Record<string, string> = {
            owner: 'admin.roles.instanceOwner',
            admin: 'admin.roles.instanceAdmin',
            user: 'admin.roles.user'
        };
        return labels[role] ? this.transloco.translate(labels[role]) : this.humanizeAuditValue(role);
    }

    private onData(resp: InstanceAuditListResponse) {
        this.entries.set(resp.entries);
        this.total.set(resp.total);
        this.limit.set(resp.limit);
        this.offset.set(resp.offset);
        this.isLoading.set(false);
    }

    private buildFilter() {
        let from: string | undefined;
        let to: string | undefined;
        const range = this.dateRange();
        if (range && range.length === 2 && range[0] && range[1]) {
            from = (range[0] as Date).toISOString();
            to = (range[1] as Date).toISOString();
        }

        return {
            action: this.selectedAction() || undefined,
            outcome: this.selectedOutcome() || undefined,
            target_type: this.selectedTargetType() || undefined,
            query: this.searchQuery() || undefined,
            from,
            to,
            limit: this.limit(),
            offset: this.offset()
        };
    }

    private humanizeAuditValue(value: string) {
        if (!value) return '-';
        return value.replace(/[._-]+/g, ' ').replace(/\b\w/g, (char) => char.toUpperCase());
    }
}
