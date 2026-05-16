import { ChangeDetectionStrategy, Component, computed, inject, signal } from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';

import { FormControl, FormGroup, ReactiveFormsModule, Validators } from '@angular/forms';
import { finalize } from 'rxjs';
import { TranslocoPipe, TranslocoService } from '@jsverse/transloco';

import { ConfirmationService } from 'primeng/api';
import { ButtonModule } from 'primeng/button';
import { ConfirmDialogModule } from 'primeng/confirmdialog';
import { IconFieldModule } from 'primeng/iconfield';
import { InputIconModule } from 'primeng/inputicon';
import { InputTextModule } from 'primeng/inputtext';
import { MessageModule } from 'primeng/message';
import { TableModule } from 'primeng/table';

import { IPExclusion } from '@models/analytics.types';
import { ExclusionsService } from '@services/exclusions.service';
import { CopyControl } from '@components/copy-control/copy-control';
import { CrudDialog } from '@components/crud-dialog/crud-dialog';
import { dialogCancelButton, dialogDangerButton } from '@components/dialog-actions/dialog-actions';
import { RelativeDateTime } from '@components/relative-date-time/relative-date-time';
import { TableRowActionItem, TableRowActions } from '@components/table-row-actions/table-row-actions';

const ipOrCIDRPattern = /^(([0-9]{1,3}\.){3}[0-9]{1,3}(\/(3[0-2]|[12]?[0-9]))?|([0-9A-Fa-f:]+)(\/(12[0-8]|1[01][0-9]|[1-9]?[0-9]))?)$/;

interface ActionStatus {
    severity: 'success' | 'error';
    key: string;
    params?: Record<string, string | number>;
}

@Component({
    selector: 'app-admin-global-exclusion-settings',
    standalone: true,
    imports: [ReactiveFormsModule, ButtonModule, ConfirmDialogModule, IconFieldModule, InputIconModule, InputTextModule, MessageModule, TableModule, CopyControl, CrudDialog, RelativeDateTime, TableRowActions, TranslocoPipe],
    templateUrl: './admin-global-exclusion-settings.html',
    styleUrl: './admin-global-exclusion-settings.css',
    changeDetection: ChangeDetectionStrategy.OnPush,
    providers: [ConfirmationService]
})
export class AdminGlobalExclusionSettings {
    private exclusionsService = inject(ExclusionsService);
    private confirmationService = inject(ConfirmationService);
    private transloco = inject(TranslocoService);
    private activeLanguage = toSignal(this.transloco.langChanges$, { initialValue: this.transloco.getActiveLang() });

    protected readonly exclusions = signal<IPExclusion[]>([]);
    protected readonly isLoading = signal(false);
    protected readonly isSaving = signal(false);
    protected readonly error = signal<string | null>(null);
    protected readonly createError = signal<string | null>(null);
    protected readonly isAddDialogVisible = signal(false);
    protected readonly actionStatus = signal<ActionStatus | null>(null);
    protected readonly deletingRuleID = signal<string | null>(null);
    protected readonly isCurrentIPLoading = signal(false);
    protected readonly currentIPCIDR = signal('');
    protected readonly actionStatusMessage = computed(() => {
        this.activeLanguage();
        const status = this.actionStatus();
        if (!status) {
            return '';
        }

        return this.transloco.translate(status.key, status.params);
    });

    protected readonly form = new FormGroup({
        cidr: new FormControl('', { nonNullable: true, validators: [Validators.required, Validators.pattern(ipOrCIDRPattern)] }),
        description: new FormControl('', { nonNullable: true, validators: [Validators.maxLength(255)] })
    });

    constructor() {
        this.loadCurrentIP();
        this.loadExclusions();
    }

    protected addRule(): void {
        if (this.isSaving()) {
            return;
        }
        if (this.form.invalid) {
            this.form.markAllAsTouched();
            return;
        }

        this.createError.set(null);
        this.actionStatus.set(null);
        this.isSaving.set(true);

        this.exclusionsService
            .createInstanceExclusion({
                cidr: this.form.controls.cidr.value.trim(),
                description: this.form.controls.description.value.trim()
            })
            .pipe(finalize(() => this.isSaving.set(false)))
            .subscribe({
                next: (rule) => {
                    this.exclusions.update((current) => [rule, ...current]);
                    this.actionStatus.set({
                        severity: 'success',
                        key: 'admin.exclusions.status.createSuccess',
                        params: { cidr: rule.cidr }
                    });
                    this.closeAddDialog();
                },
                error: () => {
                    this.actionStatus.set(null);
                    this.createError.set('admin.exclusions.errors.createFailed');
                }
            });
    }

    protected openAddDialog(): void {
        this.form.reset({ cidr: '', description: '' });
        this.createError.set(null);
        this.isAddDialogVisible.set(true);
    }

    protected onAddDialogVisibleChange(visible: boolean): void {
        if (!visible && this.isSaving()) {
            this.isAddDialogVisible.set(true);
            return;
        }
        this.isAddDialogVisible.set(visible);
        if (!visible) {
            this.closeAddDialog();
        }
    }

    protected ruleActions(rule: IPExclusion): TableRowActionItem[] {
        this.activeLanguage();
        return [
            {
                label: this.transloco.translate('share.dialog.deleteAction'),
                icon: 'pi pi-trash',
                danger: true,
                command: () => this.confirmDeleteRule(rule)
            }
        ];
    }

    protected confirmDeleteRule(rule: IPExclusion): void {
        this.confirmationService.confirm({
            message: this.transloco.translate('admin.exclusions.confirmDelete', { cidr: rule.cidr }),
            icon: 'pi pi-exclamation-triangle',
            rejectButtonProps: dialogCancelButton(this.transloco.translate('common.actions.cancel')),
            acceptButtonProps: dialogDangerButton(this.transloco.translate('share.dialog.deleteAction')),
            accept: () => this.deleteRule(rule)
        });
    }

    protected deleteRule(rule: IPExclusion): void {
        this.error.set(null);
        this.actionStatus.set(null);
        this.deletingRuleID.set(rule.id);
        this.exclusionsService
            .deleteInstanceExclusion(rule.id)
            .pipe(finalize(() => this.deletingRuleID.set(null)))
            .subscribe({
                next: () => {
                    this.exclusions.update((current) => current.filter((entry) => entry.id !== rule.id));
                    this.actionStatus.set({
                        severity: 'success',
                        key: 'admin.exclusions.status.deleteSuccess',
                        params: { cidr: rule.cidr }
                    });
                },
                error: () => {
                    this.actionStatus.set(null);
                    this.error.set('admin.exclusions.errors.deleteFailed');
                }
            });
    }

    protected reload(): void {
        this.loadExclusions();
    }

    private loadCurrentIP(): void {
        this.isCurrentIPLoading.set(true);
        this.currentIPCIDR.set('');

        this.exclusionsService
            .getCurrentIP()
            .pipe(finalize(() => this.isCurrentIPLoading.set(false)))
            .subscribe({
                next: (currentIP) => {
                    this.currentIPCIDR.set(currentIP.cidr);
                },
                error: () => {
                    this.currentIPCIDR.set('');
                }
            });
    }

    private loadExclusions(): void {
        this.isLoading.set(true);
        this.error.set(null);
        this.actionStatus.set(null);

        this.exclusionsService
            .listInstanceExclusions()
            .pipe(finalize(() => this.isLoading.set(false)))
            .subscribe({
                next: (rules) => {
                    this.exclusions.set(rules);
                },
                error: () => {
                    this.error.set('admin.exclusions.errors.loadFailed');
                }
            });
    }

    private closeAddDialog(): void {
        this.isAddDialogVisible.set(false);
        this.form.reset({ cidr: '', description: '' });
        this.createError.set(null);
    }
}
