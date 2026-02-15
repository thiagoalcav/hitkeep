import { Component, Input, Output, EventEmitter, inject, signal, OnChanges, computed } from '@angular/core';

import { FormControl, ReactiveFormsModule, Validators } from '@angular/forms';
import { compatForm } from '@angular/forms/signals/compat';
import { toSignal } from '@angular/core/rxjs-interop';
import { TranslocoPipe, TranslocoService } from '@jsverse/transloco';
import { DialogModule } from 'primeng/dialog';
import { ButtonModule } from 'primeng/button';
import { InputTextModule } from 'primeng/inputtext';
import { SelectModule } from 'primeng/select';
import { TableModule } from 'primeng/table';
import { TooltipModule } from 'primeng/tooltip';
import { AnalyticsService } from '@services/analytics.service';
import { Funnel, FunnelStep } from '@models/analytics.types';

interface FunnelStepControl {
    typeControl: FormControl<FunnelStep['type']>;
    valueControl: FormControl<string>;
}

@Component({
    selector: 'app-funnel-manager',
    standalone: true,
    imports: [ReactiveFormsModule, DialogModule, ButtonModule, InputTextModule, SelectModule, TableModule, TooltipModule, TranslocoPipe],
    template: `
        <p-dialog [header]="'funnels.manager.dialogTitle' | transloco" [(visible)]="visible" [modal]="true" [style]="{ width: '800px', maxWidth: '90vw' }" [draggable]="false" [resizable]="false" (onHide)="onHide()">
            <p class="text-sm text-muted-color mb-4">{{ 'funnels.manager.dialogDescription' | transloco }}</p>

            <!-- List Funnels -->
            <p-table [value]="funnels()" [loading]="loading()" styleClass="p-datatable-sm mb-6" [rowHover]="true">
                <ng-template pTemplate="header">
                    <tr>
                        <th>{{ 'common.columns.name' | transloco }}</th>
                        <th>{{ 'funnels.manager.stepsLabel' | transloco }}</th>
                        <th style="width: 8rem"></th>
                    </tr>
                </ng-template>
                <ng-template pTemplate="body" let-funnel>
                    <tr>
                        <td class="font-medium">{{ funnel.name }}</td>
                        <td>
                            <div class="flex items-center gap-1 flex-wrap">
                                @for (step of funnel.steps; track $index; let last = $last) {
                                    <span class="text-xs bg-surface-100 dark:bg-surface-800 px-2 py-1 rounded border border-surface-200 dark:border-surface-700" [title]="step.value"> {{ step.type === 'path' ? '/' : '' }}{{ step.value }} </span>
                                    @if (!last) {
                                        <i class="pi pi-arrow-right text-xs text-muted-color"></i>
                                    }
                                }
                            </div>
                        </td>
                        <td>
                            <div class="flex gap-2">
                                <p-button icon="pi pi-chart-bar" (onClick)="viewFunnel.emit(funnel)" styleClass="p-button-text p-button-sm" [rounded]="true" [pTooltip]="'funnels.manager.viewStats' | transloco"></p-button>
                                <p-button icon="pi pi-trash" (onClick)="deleteFunnel(funnel.id)" styleClass="p-button-text p-button-danger p-button-sm" [rounded]="true"></p-button>
                            </div>
                        </td>
                    </tr>
                </ng-template>
                <ng-template pTemplate="emptymessage">
                    <tr>
                        <td colspan="3" class="text-center text-muted-color py-4">{{ 'funnels.manager.empty' | transloco }}</td>
                    </tr>
                </ng-template>
            </p-table>

            <!-- Add Funnel Form -->
            <div class="flex flex-col gap-4 p-4 border border-surface-200 dark:border-surface-700 rounded-lg bg-surface-50 dark:bg-surface-900">
                <div class="flex items-center justify-between">
                    <h4 class="font-semibold text-sm m-0">{{ 'funnels.manager.createTitle' | transloco }}</h4>
                </div>

                <div class="flex flex-col gap-3">
                    <div class="flex flex-col gap-1">
                        <label for="f-name" class="text-xs font-medium">{{ 'common.columns.name' | transloco }}</label>
                        <input pInputText id="f-name" [formControl]="newFunnelForm.name().control()" [placeholder]="'funnels.manager.namePlaceholder' | transloco" class="p-inputtext-sm w-full" />
                    </div>

                    <div class="flex flex-col gap-2">
                        <span class="text-xs font-medium">{{ 'funnels.manager.stepsLabel' | transloco }}</span>
                        @for (step of stepControls(); track $index) {
                            <div class="flex gap-2 items-center">
                                <span class="text-xs font-bold text-muted-color w-4">{{ $index + 1 }}.</span>
                                <p-select [options]="types()" [formControl]="step.typeControl" optionLabel="label" optionValue="value" styleClass="w-32 p-inputtext-sm" appendTo="body" />
                                <input
                                    pInputText
                                    [formControl]="step.valueControl"
                                    [placeholder]="step.typeControl.value === 'path' ? ('funnels.manager.stepPathPlaceholder' | transloco) : ('funnels.manager.stepEventPlaceholder' | transloco)"
                                    class="p-inputtext-sm flex-1"
                                />
                                <p-button icon="pi pi-trash" (onClick)="removeStep($index)" styleClass="p-button-text p-button-danger p-button-sm" [rounded]="true" [disabled]="stepControls().length <= 2"></p-button>
                            </div>
                        }
                        <div class="flex">
                            <p-button [label]="'funnels.manager.addStep' | transloco" icon="pi pi-plus" (onClick)="addStep()" styleClass="p-button-text p-button-sm" size="small"></p-button>
                        </div>
                    </div>
                </div>

                <div class="flex justify-end mt-2">
                    <p-button [label]="'funnels.manager.createAction' | transloco" icon="pi pi-plus" (onClick)="createFunnel()" [loading]="creating()" [disabled]="newFunnelForm().invalid() || !isValid()" size="small"></p-button>
                </div>
            </div>
        </p-dialog>
    `
})
export class FunnelManager implements OnChanges {
    @Input() visible = false;
    @Output() visibleChange = new EventEmitter<boolean>();
    @Input() siteId: string | null = null;
    @Output() funnelsChanged = new EventEmitter<void>();
    @Output() viewFunnel = new EventEmitter<Funnel>();

    private analyticsService = inject(AnalyticsService);
    private transloco = inject(TranslocoService);
    private activeLanguage = toSignal(this.transloco.langChanges$, { initialValue: this.transloco.getActiveLang() });

    funnels = signal<Funnel[]>([]);
    loading = signal(false);
    creating = signal(false);

    protected readonly types = computed(() => {
        this.activeLanguage();
        return [
            { label: this.transloco.translate('funnels.manager.typePagePath'), value: 'path' },
            { label: this.transloco.translate('funnels.manager.typeCustomEvent'), value: 'event' }
        ];
    });

    private readonly newFunnelModel = signal({
        name: new FormControl('', { nonNullable: true, validators: [Validators.required] })
    });
    protected readonly newFunnelForm = compatForm(this.newFunnelModel);
    protected readonly stepControls = signal<FunnelStepControl[]>([this.createStepControl(), this.createStepControl()]);

    ngOnChanges() {
        if (this.visible && this.siteId) {
            this.loadFunnels();
        }
    }

    loadFunnels() {
        if (!this.siteId) return;
        this.loading.set(true);
        this.analyticsService.getFunnels(this.siteId).subscribe({
            next: (funnels) => {
                this.funnels.set(funnels || []);
                this.loading.set(false);
            },
            error: () => this.loading.set(false)
        });
    }

    addStep() {
        this.stepControls.update((steps) => [...steps, this.createStepControl()]);
    }

    removeStep(index: number) {
        this.stepControls.update((steps) => (steps.length > 2 ? steps.filter((_, i) => i !== index) : steps));
    }

    createFunnel() {
        if (!this.siteId || !this.isValid()) return;
        const payload: { name: string; steps: FunnelStep[] } = {
            name: this.newFunnelForm.name().value().trim(),
            steps: this.stepControls().map((step) => ({
                type: step.typeControl.value,
                value: step.valueControl.value.trim()
            }))
        };

        this.creating.set(true);
        this.analyticsService.createFunnel(this.siteId, payload).subscribe({
            next: () => {
                this.creating.set(false);
                this.newFunnelForm.name().control().reset('');
                this.stepControls.set([this.createStepControl(), this.createStepControl()]);
                this.loadFunnels();
                this.funnelsChanged.emit();
            },
            error: () => this.creating.set(false)
        });
    }

    deleteFunnel(id: string) {
        if (!this.siteId) return;
        this.analyticsService.deleteFunnel(this.siteId, id).subscribe({
            next: () => {
                this.loadFunnels();
                this.funnelsChanged.emit();
            }
        });
    }

    isValid() {
        return this.newFunnelForm.name().value().trim().length > 0 && this.stepControls().length >= 2 && this.stepControls().every((step) => step.valueControl.value.trim().length > 0);
    }

    onHide() {
        this.visible = false;
        this.visibleChange.emit(false);
    }

    private createStepControl(type: FunnelStep['type'] = 'path', value = ''): FunnelStepControl {
        return {
            typeControl: new FormControl(type, { nonNullable: true, validators: [Validators.required] }),
            valueControl: new FormControl(value, { nonNullable: true, validators: [Validators.required] })
        };
    }
}
