import { Component, Input, Output, EventEmitter, inject, signal, OnChanges } from '@angular/core';

import { FormsModule } from '@angular/forms';
import { DialogModule } from 'primeng/dialog';
import { ButtonModule } from 'primeng/button';
import { InputTextModule } from 'primeng/inputtext';
import { SelectModule } from 'primeng/select';
import { TableModule } from 'primeng/table';
import { TooltipModule } from 'primeng/tooltip';
import { AnalyticsService } from '@services/analytics.service';
import { Funnel, FunnelStep } from '@models/analytics.types';

@Component({
    selector: 'app-funnel-manager',
    standalone: true,
    imports: [FormsModule, DialogModule, ButtonModule, InputTextModule, SelectModule, TableModule, TooltipModule],
    template: `
        <p-dialog header="Manage Funnels" [(visible)]="visible" [modal]="true" [style]="{ width: '800px', maxWidth: '90vw' }" [draggable]="false" [resizable]="false" (onHide)="onHide()">
            <p class="text-sm text-muted-color mb-4">Track user journeys across multiple steps.</p>

            <!-- List Funnels -->
            <p-table [value]="funnels()" [loading]="loading()" styleClass="p-datatable-sm mb-6" [rowHover]="true">
                <ng-template pTemplate="header">
                    <tr>
                        <th>Name</th>
                        <th>Steps</th>
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
                                <p-button icon="pi pi-chart-bar" (onClick)="viewFunnel.emit(funnel)" styleClass="p-button-text p-button-sm" [rounded]="true" pTooltip="View Stats"></p-button>
                                <p-button icon="pi pi-trash" (onClick)="deleteFunnel(funnel.id)" styleClass="p-button-text p-button-danger p-button-sm" [rounded]="true"></p-button>
                            </div>
                        </td>
                    </tr>
                </ng-template>
                <ng-template pTemplate="emptymessage">
                    <tr>
                        <td colspan="3" class="text-center text-muted-color py-4">No funnels defined yet.</td>
                    </tr>
                </ng-template>
            </p-table>

            <!-- Add Funnel Form -->
            <div class="flex flex-col gap-4 p-4 border border-surface-200 dark:border-surface-700 rounded-lg bg-surface-50 dark:bg-surface-900">
                <div class="flex items-center justify-between">
                    <h4 class="font-semibold text-sm m-0">Create New Funnel</h4>
                </div>

                <div class="flex flex-col gap-3">
                    <div class="flex flex-col gap-1">
                        <label for="f-name" class="text-xs font-medium">Name</label>
                        <input pInputText id="f-name" [(ngModel)]="newFunnel.name" placeholder="e.g. Checkout Flow" class="p-inputtext-sm w-full" />
                    </div>

                    <div class="flex flex-col gap-2">
                        <span class="text-xs font-medium">Steps</span>
                        @for (step of newFunnel.steps; track $index) {
                            <div class="flex gap-2 items-center">
                                <span class="text-xs font-bold text-muted-color w-4">{{ $index + 1 }}.</span>
                                <p-select [options]="types" [(ngModel)]="step.type" optionLabel="label" optionValue="value" styleClass="w-32 p-inputtext-sm" appendTo="body" />
                                <input pInputText [(ngModel)]="step.value" [placeholder]="step.type === 'path' ? '/path' : 'event_name'" class="p-inputtext-sm flex-1" />
                                <p-button icon="pi pi-trash" (onClick)="removeStep($index)" styleClass="p-button-text p-button-danger p-button-sm" [rounded]="true" [disabled]="newFunnel.steps.length <= 2"></p-button>
                            </div>
                        }
                        <div class="flex">
                            <p-button label="Add Step" icon="pi pi-plus" (onClick)="addStep()" styleClass="p-button-text p-button-sm" size="small"></p-button>
                        </div>
                    </div>
                </div>

                <div class="flex justify-end mt-2">
                    <p-button label="Create Funnel" icon="pi pi-plus" (onClick)="createFunnel()" [loading]="creating()" [disabled]="!isValid()" size="small"></p-button>
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

    funnels = signal<Funnel[]>([]);
    loading = signal(false);
    creating = signal(false);

    types = [
        { label: 'Page Path', value: 'path' },
        { label: 'Custom Event', value: 'event' }
    ];

    newFunnel: { name: string; steps: FunnelStep[] } = {
        name: '',
        steps: [
            { type: 'path', value: '' },
            { type: 'path', value: '' }
        ]
    };

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
        this.newFunnel.steps.push({ type: 'path', value: '' });
    }

    removeStep(index: number) {
        if (this.newFunnel.steps.length > 2) {
            this.newFunnel.steps.splice(index, 1);
        }
    }

    createFunnel() {
        if (!this.siteId || !this.isValid()) return;

        this.creating.set(true);
        this.analyticsService.createFunnel(this.siteId, this.newFunnel).subscribe({
            next: () => {
                this.creating.set(false);
                this.newFunnel = {
                    name: '',
                    steps: [
                        { type: 'path', value: '' },
                        { type: 'path', value: '' }
                    ]
                };
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
        return this.newFunnel.name.length > 0 && this.newFunnel.steps.length >= 2 && this.newFunnel.steps.every((s) => s.value.length > 0);
    }

    onHide() {
        this.visible = false;
        this.visibleChange.emit(false);
    }
}
