import { ChangeDetectionStrategy, Component, computed, inject, input, model, output, signal } from '@angular/core';

import { rxResource, toSignal } from '@angular/core/rxjs-interop';
import { FormControl, ReactiveFormsModule, Validators } from '@angular/forms';
import { compatForm } from '@angular/forms/signals/compat';
import { TranslocoPipe, TranslocoService } from '@jsverse/transloco';
import { switchMap, finalize } from 'rxjs';
import { DialogModule } from 'primeng/dialog';
import { ButtonModule } from 'primeng/button';
import { DividerModule } from 'primeng/divider';
import { IconFieldModule } from 'primeng/iconfield';
import { InputIconModule } from 'primeng/inputicon';
import { InputGroupModule } from 'primeng/inputgroup';
import { InputGroupAddonModule } from 'primeng/inputgroupaddon';
import { InputTextModule } from 'primeng/inputtext';
import { SelectButtonModule } from 'primeng/selectbutton';
import { TableModule } from 'primeng/table';
import { TagModule } from 'primeng/tag';
import { TooltipModule } from 'primeng/tooltip';
import { MessageModule } from 'primeng/message';
import { AnalyticsService } from '@services/analytics.service';
import { Goal } from '@models/analytics.types';

type GoalManagerAction = 'create' | 'update' | 'delete';

@Component({
    selector: 'app-goal-manager',
    imports: [
        ReactiveFormsModule,
        DialogModule,
        ButtonModule,
        DividerModule,
        IconFieldModule,
        InputIconModule,
        InputGroupModule,
        InputGroupAddonModule,
        InputTextModule,
        SelectButtonModule,
        TableModule,
        TagModule,
        TooltipModule,
        MessageModule,
        TranslocoPipe
    ],
    changeDetection: ChangeDetectionStrategy.OnPush,
    template: `
        <p-dialog [header]="'goals.manager.dialogTitle' | transloco" [(visible)]="visible" [modal]="true" [style]="dialogStyle" [draggable]="false" [resizable]="false" (onHide)="resetEditor()">
            <p class="text-sm text-muted-color mb-4">{{ "goals.manager.dialogDescription" | transloco }}</p>

            @if (successKey(); as key) {
                <p-message severity="success" styleClass="w-full mb-4" [text]="key | transloco" />
            } @else if (errorKey(); as key) {
                <p-message severity="error" styleClass="w-full mb-4" [text]="key | transloco" />
            }

            <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between mb-3">
                <p-button [label]="'goals.manager.newAction' | transloco" icon="pi pi-plus" (onClick)="resetEditor()" [outlined]="true" size="small" />
                <p-iconfield>
                    <p-inputicon class="pi pi-search" />
                    <input pInputText #goalSearch [placeholder]="'common.searchPlaceholder' | transloco" (input)="goalsTable.filterGlobal($any($event.target).value, 'contains')" class="w-full" />
                </p-iconfield>
            </div>

            <div class="hk-crud-table-wrap">
                <p-table #goalsTable [value]="goals()" [loading]="goalsResource.isLoading()" styleClass="hk-crud-table p-datatable-sm" [rowHover]="true" [globalFilterFields]="['name', 'type', 'value']" [sortField]="'name'" [sortOrder]="1">
                    <ng-template pTemplate="header">
                        <tr>
                            <th pSortableColumn="name">
                                {{ "common.columns.name" | transloco }}
                                <p-sortIcon field="name" />
                            </th>
                            <th pSortableColumn="type" style="width: 100px">
                                {{ "common.columns.type" | transloco }}
                                <p-sortIcon field="type" />
                            </th>
                            <th pSortableColumn="value">
                                {{ "common.columns.value" | transloco }}
                                <p-sortIcon field="value" />
                            </th>
                            <th style="width: 7rem"></th>
                        </tr>
                    </ng-template>
                    <ng-template pTemplate="body" let-goal>
                        <tr [class]="editingGoal()?.id === goal.id ? 'bg-surface-50 dark:bg-surface-800' : ''">
                            <td class="font-medium">{{ goal.name }}</td>
                            <td>
                                <p-tag [value]="goalTypeLabel(goal.type)" [severity]="goal.type === 'event' ? 'info' : 'warn'" [rounded]="true" />
                            </td>
                            <td class="font-mono text-sm text-muted-color truncate max-w-[150px]" [title]="goal.value">{{ goal.value }}</td>
                            <td>
                                <div class="flex justify-end gap-1">
                                    <p-button
                                        icon="pi pi-pencil"
                                        (onClick)="editGoal(goal)"
                                        styleClass="p-button-text p-button-sm"
                                        [rounded]="true"
                                        [pTooltip]="'goals.manager.editTooltip' | transloco"
                                        [ariaLabel]="'goals.manager.editTooltip' | transloco"
                                        [disabled]="isBusy()"
                                    />
                                    <p-button
                                        icon="pi pi-trash"
                                        (onClick)="deleteGoal(goal.id)"
                                        styleClass="p-button-text p-button-danger p-button-sm"
                                        [rounded]="true"
                                        [pTooltip]="'goals.manager.deleteTooltip' | transloco"
                                        [ariaLabel]="'goals.manager.deleteTooltip' | transloco"
                                        [loading]="deletingGoalId() === goal.id"
                                        [disabled]="isBusy()"
                                    />
                                </div>
                            </td>
                        </tr>
                    </ng-template>
                    <ng-template pTemplate="emptymessage">
                        <tr>
                            <td colspan="4" class="text-center text-muted-color py-4">{{ "goals.manager.empty" | transloco }}</td>
                        </tr>
                    </ng-template>
                </p-table>
            </div>

            <p-divider />

            <div class="flex items-center justify-between gap-3 mb-4">
                <h4 class="font-semibold text-sm mt-0 mb-0">{{ editorTitle() }}</h4>
                @if (editingGoal()) {
                    <p-button [label]="'common.actions.cancel' | transloco" (onClick)="resetEditor()" [text]="true" size="small" />
                }
            </div>

            <div class="flex flex-col gap-4">
                <div class="flex flex-col gap-1">
                    <label for="g-name" class="text-xs font-medium">{{ "common.columns.name" | transloco }}</label>
                    <input pInputText id="g-name" [formControl]="newGoalForm.name().control()" [placeholder]="'goals.manager.namePlaceholder' | transloco" class="w-full" />
                </div>

                <div class="flex flex-col gap-1">
                    <span id="goal-type-label" class="text-xs font-medium">{{ "common.columns.type" | transloco }}</span>
                    <p-selectbutton [options]="types()" [formControl]="newGoalForm.type().control()" optionLabel="label" optionValue="value" size="small" ariaLabelledBy="goal-type-label" />
                </div>

                <div class="flex flex-col gap-1">
                    <label for="g-value" class="text-xs font-medium">
                        {{ newGoalForm.type().value() === "path" ? ("goals.manager.urlPathLabel" | transloco) : ("goals.manager.eventNameLabel" | transloco) }}
                    </label>
                    @if (newGoalForm.type().value() === "path") {
                        <p-inputgroup>
                            <p-inputgroup-addon><i class="pi pi-link"></i></p-inputgroup-addon>
                            <input pInputText id="g-value" [formControl]="newGoalForm.value().control()" [placeholder]="'goals.manager.urlPathPlaceholder' | transloco" />
                        </p-inputgroup>
                        <small class="text-xs text-muted-color">{{ "goals.manager.urlPathHelp" | transloco }}</small>
                    } @else {
                        <p-inputgroup>
                            <p-inputgroup-addon><i class="pi pi-bolt"></i></p-inputgroup-addon>
                            <input pInputText id="g-value" [formControl]="newGoalForm.value().control()" [placeholder]="'goals.manager.eventNamePlaceholder' | transloco" />
                        </p-inputgroup>
                        <small class="text-xs text-muted-color"> {{ "goals.manager.eventNameHelpPrefix" | transloco }} <code>hk.event('name')</code>{{ "goals.manager.eventNameHelpSuffix" | transloco }} </small>
                    }
                </div>
            </div>

            <ng-template pTemplate="footer">
                <p-button [label]="primaryActionLabel()" [icon]="editingGoal() ? 'pi pi-save' : 'pi pi-plus'" (onClick)="saveGoal()" [loading]="saving()" [disabled]="!canSave()" size="small" />
            </ng-template>
        </p-dialog>
    `
})
export class GoalManager {
    visible = model(false);
    readonly siteId = input<string | null>(null);
    readonly goalsChanged = output();

    private analyticsService = inject(AnalyticsService);
    private transloco = inject(TranslocoService);
    private activeLanguage = toSignal(this.transloco.langChanges$, { initialValue: this.transloco.getActiveLang() });

    protected readonly dialogStyle = { width: '760px', maxWidth: '94vw' };
    protected readonly saving = signal(false);
    protected readonly deletingGoalId = signal<string | null>(null);
    protected readonly editingGoal = signal<Goal | null>(null);
    protected readonly isBusy = computed(() => this.saving() || this.deletingGoalId() !== null);
    protected readonly successKey = signal<string | null>(null);
    protected readonly errorKey = signal<string | null>(null);

    protected readonly goalsResource = rxResource({
        params: () => {
            const siteId = this.siteId();
            return this.visible() && siteId ? { siteId } : undefined;
        },
        stream: ({ params }) => this.analyticsService.getGoals(params.siteId)
    });

    protected readonly goals = computed(() => this.goalsResource.value() ?? []);

    protected readonly types = computed(() => {
        this.activeLanguage();
        return [
            { label: this.transloco.translate('goals.manager.typePagePath'), value: 'path', icon: 'pi pi-link' },
            { label: this.transloco.translate('goals.manager.typeCustomEvent'), value: 'event', icon: 'pi pi-bolt' }
        ];
    });

    protected readonly editorTitle = computed(() => {
        this.activeLanguage();
        return this.editingGoal() ? this.transloco.translate('goals.manager.editTitle') : this.transloco.translate('goals.manager.addTitle');
    });

    protected readonly primaryActionLabel = computed(() => {
        this.activeLanguage();
        return this.editingGoal() ? this.transloco.translate('goals.manager.saveAction') : this.transloco.translate('goals.manager.createAction');
    });

    private readonly newGoalModel = signal({
        name: new FormControl('', { nonNullable: true, validators: [Validators.required] }),
        type: new FormControl<'path' | 'event'>('path', { nonNullable: true, validators: [Validators.required] }),
        value: new FormControl('', { nonNullable: true, validators: [Validators.required] })
    });
    protected readonly newGoalForm = compatForm(this.newGoalModel);

    protected readonly canSave = computed(() => {
        const name = this.newGoalForm.name().value().trim();
        const value = this.newGoalForm.value().value().trim();
        return !this.isBusy() && !this.newGoalForm().invalid() && name.length > 0 && value.length > 0;
    });

    protected goalTypeLabel(type: Goal['type']): string {
        if (type === 'event') {
            return this.transloco.translate('goals.manager.typeCustomEvent');
        }
        return this.transloco.translate('goals.manager.typePagePath');
    }

    editGoal(goal: Goal) {
        if (this.isBusy()) return;
        this.clearFeedback();
        this.editingGoal.set(goal);
        this.newGoalForm.name().control().setValue(goal.name);
        this.newGoalForm.type().control().setValue(goal.type);
        this.newGoalForm.value().control().setValue(goal.value);
    }

    resetEditor() {
        this.clearFeedback();
        this.editingGoal.set(null);
        this.newGoalForm.name().control().reset('');
        this.newGoalForm.type().control().reset('path');
        this.newGoalForm.value().control().reset('');
    }

    saveGoal() {
        const siteId = this.siteId();
        if (!siteId || !this.canSave()) return;
        const payload = {
            name: this.newGoalForm.name().value().trim(),
            type: this.newGoalForm.type().value(),
            value: this.newGoalForm.value().value().trim()
        };
        const editingGoal = this.editingGoal();
        const action: GoalManagerAction = editingGoal ? 'update' : 'create';

        this.clearFeedback();
        this.saving.set(true);
        const request = editingGoal ? this.analyticsService.createGoal(siteId, payload).pipe(switchMap(() => this.analyticsService.deleteGoal(siteId, editingGoal.id))) : this.analyticsService.createGoal(siteId, payload);

        request.pipe(finalize(() => this.saving.set(false))).subscribe({
            next: () => {
                this.resetEditor();
                this.goalsResource.reload();
                this.goalsChanged.emit();
                this.setSuccess(action);
            },
            error: () => this.setError(action)
        });
    }

    deleteGoal(id: string) {
        const siteId = this.siteId();
        if (!siteId || this.isBusy()) return;
        this.clearFeedback();
        this.deletingGoalId.set(id);
        this.analyticsService
            .deleteGoal(siteId, id)
            .pipe(finalize(() => this.deletingGoalId.set(null)))
            .subscribe({
                next: () => {
                    if (this.editingGoal()?.id === id) {
                        this.resetEditor();
                    }
                    this.goalsResource.reload();
                    this.goalsChanged.emit();
                    this.setSuccess('delete');
                },
                error: () => this.setError('delete')
            });
    }

    private clearFeedback() {
        this.successKey.set(null);
        this.errorKey.set(null);
    }

    private setSuccess(action: GoalManagerAction) {
        this.successKey.set(`goals.manager.messages.${action}Success`);
        this.errorKey.set(null);
    }

    private setError(action: GoalManagerAction) {
        this.errorKey.set(`goals.manager.messages.${action}Error`);
        this.successKey.set(null);
    }
}
