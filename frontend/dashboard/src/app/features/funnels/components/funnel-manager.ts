import { ChangeDetectionStrategy, Component, computed, effect, inject, input, model, output, signal } from '@angular/core';

import { CdkDragDrop, DragDropModule, moveItemInArray } from '@angular/cdk/drag-drop';
import { FormControl, ReactiveFormsModule, Validators } from '@angular/forms';
import { compatForm } from '@angular/forms/signals/compat';
import { rxResource, toSignal } from '@angular/core/rxjs-interop';
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
import { Funnel, FunnelStep } from '@models/analytics.types';

type FunnelManagerAction = 'create' | 'update' | 'delete';

interface FunnelStepControl {
    typeControl: FormControl<FunnelStep['type']>;
    valueControl: FormControl<string>;
}

@Component({
    selector: 'app-funnel-manager',
    imports: [
        DragDropModule,
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
        <p-dialog [header]="'funnels.manager.dialogTitle' | transloco" [(visible)]="visible" [modal]="true" [style]="dialogStyle" [draggable]="false" [resizable]="false" (onHide)="onHide()">
            <p class="text-sm text-muted-color mb-4">{{ "funnels.manager.dialogDescription" | transloco }}</p>

            @if (successKey(); as key) {
                <p-message severity="success" styleClass="w-full mb-4" [text]="key | transloco" />
            } @else if (errorKey(); as key) {
                <p-message severity="error" styleClass="w-full mb-4" [text]="key | transloco" />
            }

            <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between mb-3">
                <p-button [label]="'funnels.manager.newAction' | transloco" icon="pi pi-plus" (onClick)="resetEditor()" [outlined]="true" size="small" />
                <p-iconfield>
                    <p-inputicon class="pi pi-search" />
                    <input pInputText #funnelSearch [placeholder]="'common.searchPlaceholder' | transloco" (input)="funnelsTable.filterGlobal($any($event.target).value, 'contains')" class="w-full" />
                </p-iconfield>
            </div>

            <div class="hk-crud-table-wrap">
                <p-table #funnelsTable [value]="funnels()" [loading]="funnelsResource.isLoading()" styleClass="hk-crud-table p-datatable-sm" [rowHover]="true" [globalFilterFields]="['name']" [sortField]="'name'" [sortOrder]="1">
                    <ng-template pTemplate="header">
                        <tr>
                            <th pSortableColumn="name">
                                {{ "common.columns.name" | transloco }}
                                <p-sortIcon field="name" />
                            </th>
                            <th>{{ "funnels.manager.stepsLabel" | transloco }}</th>
                            <th style="width: 8rem"></th>
                        </tr>
                    </ng-template>
                    <ng-template pTemplate="body" let-funnel>
                        <tr [class]="editingFunnel()?.id === funnel.id ? 'bg-surface-50 dark:bg-surface-800' : ''">
                            <td class="font-medium">{{ funnel.name }}</td>
                            <td>
                                <div class="flex items-center gap-1 flex-wrap">
                                    @for (step of funnel.steps; track $index; let last = $last) {
                                        <p-tag [value]="step.value" [severity]="step.type === 'event' ? 'info' : 'secondary'" [rounded]="true" [icon]="step.type === 'event' ? 'pi pi-bolt' : 'pi pi-link'" />
                                        @if (!last) {
                                            <i class="pi pi-arrow-right text-xs text-muted-color"></i>
                                        }
                                    }
                                </div>
                            </td>
                            <td>
                                <div class="flex gap-1 justify-end">
                                    <p-button
                                        icon="pi pi-chart-bar"
                                        (onClick)="viewFunnel.emit(funnel)"
                                        styleClass="p-button-text p-button-sm"
                                        [rounded]="true"
                                        [pTooltip]="'funnels.manager.viewStats' | transloco"
                                        [ariaLabel]="'funnels.manager.viewStats' | transloco"
                                        [disabled]="isBusy()"
                                    />
                                    <p-button
                                        icon="pi pi-pencil"
                                        (onClick)="editFunnel(funnel)"
                                        styleClass="p-button-text p-button-sm"
                                        [rounded]="true"
                                        [pTooltip]="'funnels.manager.editTooltip' | transloco"
                                        [ariaLabel]="'funnels.manager.editTooltip' | transloco"
                                        [disabled]="isBusy()"
                                    />
                                    <p-button
                                        icon="pi pi-trash"
                                        (onClick)="deleteFunnel(funnel.id)"
                                        styleClass="p-button-text p-button-danger p-button-sm"
                                        [rounded]="true"
                                        [pTooltip]="'funnels.manager.deleteTooltip' | transloco"
                                        [ariaLabel]="'funnels.manager.deleteTooltip' | transloco"
                                        [loading]="deletingFunnelId() === funnel.id"
                                        [disabled]="isBusy()"
                                    />
                                </div>
                            </td>
                        </tr>
                    </ng-template>
                    <ng-template pTemplate="emptymessage">
                        <tr>
                            <td colspan="3" class="text-center text-muted-color py-4">{{ "funnels.manager.empty" | transloco }}</td>
                        </tr>
                    </ng-template>
                </p-table>
            </div>

            <p-divider />

            <div class="flex items-center justify-between gap-3 mb-4">
                <h4 class="font-semibold text-sm mt-0 mb-0">{{ editorTitle() }}</h4>
                @if (editingFunnel()) {
                    <p-button [label]="'common.actions.cancel' | transloco" (onClick)="resetEditor()" [text]="true" size="small" />
                }
            </div>

            <div class="flex flex-col gap-4">
                <div class="flex flex-col gap-1">
                    <label for="f-name" class="text-xs font-medium">{{ "common.columns.name" | transloco }}</label>
                    <input pInputText id="f-name" [formControl]="newFunnelForm.name().control()" [placeholder]="'funnels.manager.namePlaceholder' | transloco" class="w-full" />
                </div>

                <div class="flex flex-col gap-1">
                    <span id="funnel-steps-label" class="text-xs font-medium mb-1">{{ "funnels.manager.stepsLabel" | transloco }}</span>

                    <div cdkDropList (cdkDropListDropped)="reorderStep($event)" class="flex flex-col gap-3">
                        @for (step of stepControls(); track $index; let i = $index) {
                            <div cdkDrag class="flex gap-2 items-center">
                                <i cdkDragHandle class="pi pi-bars text-muted-color cursor-grab shrink-0"></i>
                                <span
                                    class="flex w-7 h-7 shrink-0 items-center justify-center rounded-full text-xs font-bold"
                                    [class]="i === 0 ? 'bg-primary text-primary-contrast' : 'bg-surface-200 dark:bg-surface-700 text-surface-600 dark:text-surface-300'"
                                >
                                    {{ i + 1 }}
                                </span>
                                <p-selectbutton [options]="types()" [formControl]="step.typeControl" optionLabel="label" optionValue="value" size="small" ariaLabelledBy="funnel-steps-label" />
                                <p-inputgroup>
                                    <p-inputgroup-addon>
                                        <i [class]="step.typeControl.value === 'event' ? 'pi pi-bolt' : 'pi pi-link'"></i>
                                    </p-inputgroup-addon>
                                    <input pInputText [formControl]="step.valueControl" [placeholder]="step.typeControl.value === 'path' ? ('funnels.manager.stepPathPlaceholder' | transloco) : ('funnels.manager.stepEventPlaceholder' | transloco)" />
                                </p-inputgroup>
                                <p-button icon="pi pi-times" (onClick)="removeStep(i)" severity="danger" [text]="true" [rounded]="true" size="small" [disabled]="stepControls().length <= 2 || isBusy()" />
                                <div *cdkDragPlaceholder class="rounded-md border-2 border-dashed border-primary/30 bg-primary/5 h-10 w-full"></div>
                            </div>
                        }
                    </div>

                    <p-button [label]="'funnels.manager.addStep' | transloco" icon="pi pi-plus" (onClick)="addStep()" [text]="true" size="small" class="mt-1" [disabled]="isBusy()" />
                </div>
            </div>

            <ng-template pTemplate="footer">
                <p-button [label]="primaryActionLabel()" [icon]="editingFunnel() ? 'pi pi-save' : 'pi pi-plus'" (onClick)="saveFunnel()" [loading]="saving()" [disabled]="!canSave()" size="small" />
            </ng-template>
        </p-dialog>
    `
})
export class FunnelManager {
    visible = model(false);
    readonly siteId = input<string | null>(null);
    readonly editFunnelId = input<string | null>(null);
    readonly funnelsChanged = output();
    readonly viewFunnel = output<Funnel>();

    private analyticsService = inject(AnalyticsService);
    private transloco = inject(TranslocoService);
    private activeLanguage = toSignal(this.transloco.langChanges$, { initialValue: this.transloco.getActiveLang() });

    protected readonly dialogStyle = { width: '880px', maxWidth: '94vw' };
    protected readonly saving = signal(false);
    protected readonly deletingFunnelId = signal<string | null>(null);
    protected readonly editingFunnel = signal<Funnel | null>(null);
    protected readonly isBusy = computed(() => this.saving() || this.deletingFunnelId() !== null);
    protected readonly successKey = signal<string | null>(null);
    protected readonly errorKey = signal<string | null>(null);
    private readonly lastAppliedEditFunnelId = signal<string | null>(null);

    protected readonly funnelsResource = rxResource({
        params: () => {
            const siteId = this.siteId();
            return this.visible() && siteId ? { siteId } : undefined;
        },
        stream: ({ params }) => this.analyticsService.getFunnels(params.siteId)
    });

    protected readonly funnels = computed(() => this.funnelsResource.value() ?? []);

    protected readonly types = computed(() => {
        this.activeLanguage();
        return [
            { label: this.transloco.translate('funnels.manager.typePagePath'), value: 'path' },
            { label: this.transloco.translate('funnels.manager.typeCustomEvent'), value: 'event' }
        ];
    });

    protected readonly editorTitle = computed(() => {
        this.activeLanguage();
        return this.editingFunnel() ? this.transloco.translate('funnels.manager.editTitle') : this.transloco.translate('funnels.manager.createTitle');
    });

    protected readonly primaryActionLabel = computed(() => {
        this.activeLanguage();
        return this.editingFunnel() ? this.transloco.translate('funnels.manager.saveAction') : this.transloco.translate('funnels.manager.createAction');
    });

    private readonly newFunnelModel = signal({
        name: new FormControl('', { nonNullable: true, validators: [Validators.required] })
    });
    protected readonly newFunnelForm = compatForm(this.newFunnelModel);
    protected readonly stepControls = signal<FunnelStepControl[]>([this.createStepControl(), this.createStepControl()]);

    protected readonly canSave = computed(() => {
        const name = this.newFunnelForm.name().value().trim();
        const steps = this.stepControls();
        return !this.isBusy() && !this.newFunnelForm().invalid() && name.length > 0 && steps.length >= 2 && steps.every((step) => step.valueControl.value.trim().length > 0);
    });

    constructor() {
        effect(() => {
            const editFunnelId = this.editFunnelId();
            if (!this.visible() || !editFunnelId || this.isBusy() || this.lastAppliedEditFunnelId() === editFunnelId) return;
            const funnel = this.funnels().find((item) => item.id === editFunnelId);
            if (!funnel) return;
            this.lastAppliedEditFunnelId.set(editFunnelId);
            this.editFunnel(funnel);
        });
    }

    addStep() {
        if (this.isBusy()) return;
        this.stepControls.update((steps) => [...steps, this.createStepControl()]);
    }

    removeStep(index: number) {
        if (this.isBusy()) return;
        this.stepControls.update((steps) => (steps.length > 2 ? steps.filter((_, i) => i !== index) : steps));
    }

    reorderStep(event: CdkDragDrop<FunnelStepControl[]>) {
        if (this.isBusy()) return;
        this.stepControls.update((steps) => {
            const reordered = [...steps];
            moveItemInArray(reordered, event.previousIndex, event.currentIndex);
            return reordered;
        });
    }

    editFunnel(funnel: Funnel) {
        if (this.isBusy()) return;
        this.clearFeedback();
        this.editingFunnel.set(funnel);
        this.newFunnelForm.name().control().setValue(funnel.name);
        const steps = [...funnel.steps];
        while (steps.length < 2) {
            steps.push({ type: 'path', value: '' });
        }
        this.stepControls.set(steps.map((step) => this.createStepControl(step.type, step.value)));
    }

    resetEditor() {
        this.clearFeedback();
        this.editingFunnel.set(null);
        this.newFunnelForm.name().control().reset('');
        this.stepControls.set([this.createStepControl(), this.createStepControl()]);
    }

    onHide() {
        this.visible.set(false);
        this.resetEditor();
        this.lastAppliedEditFunnelId.set(null);
    }

    saveFunnel() {
        const siteId = this.siteId();
        if (!siteId || !this.canSave()) return;
        const payload: { name: string; steps: FunnelStep[] } = {
            name: this.newFunnelForm.name().value().trim(),
            steps: this.stepControls().map((step) => ({
                type: step.typeControl.value,
                value: step.valueControl.value.trim()
            }))
        };
        const editingFunnel = this.editingFunnel();
        const action: FunnelManagerAction = editingFunnel ? 'update' : 'create';

        this.clearFeedback();
        this.saving.set(true);
        const request = editingFunnel ? this.analyticsService.createFunnel(siteId, payload).pipe(switchMap(() => this.analyticsService.deleteFunnel(siteId, editingFunnel.id))) : this.analyticsService.createFunnel(siteId, payload);

        request.pipe(finalize(() => this.saving.set(false))).subscribe({
            next: () => {
                this.resetEditor();
                this.funnelsResource.reload();
                this.funnelsChanged.emit();
                this.setSuccess(action);
            },
            error: () => this.setError(action)
        });
    }

    deleteFunnel(id: string) {
        const siteId = this.siteId();
        if (!siteId || this.isBusy()) return;
        this.clearFeedback();
        this.deletingFunnelId.set(id);
        this.analyticsService
            .deleteFunnel(siteId, id)
            .pipe(finalize(() => this.deletingFunnelId.set(null)))
            .subscribe({
                next: () => {
                    if (this.editingFunnel()?.id === id) {
                        this.resetEditor();
                    }
                    this.funnelsResource.reload();
                    this.funnelsChanged.emit();
                    this.setSuccess('delete');
                },
                error: () => this.setError('delete')
            });
    }

    private createStepControl(type: FunnelStep['type'] = 'path', value = ''): FunnelStepControl {
        return {
            typeControl: new FormControl(type, { nonNullable: true, validators: [Validators.required] }),
            valueControl: new FormControl(value, { nonNullable: true, validators: [Validators.required] })
        };
    }

    private clearFeedback() {
        this.successKey.set(null);
        this.errorKey.set(null);
    }

    private setSuccess(action: FunnelManagerAction) {
        this.successKey.set(`funnels.manager.messages.${action}Success`);
        this.errorKey.set(null);
    }

    private setError(action: FunnelManagerAction) {
        this.errorKey.set(`funnels.manager.messages.${action}Error`);
        this.successKey.set(null);
    }
}
