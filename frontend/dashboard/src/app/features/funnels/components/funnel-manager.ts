import { ChangeDetectionStrategy, Component, computed, inject, input, model, output, signal } from "@angular/core";

import { CdkDragDrop, DragDropModule, moveItemInArray } from "@angular/cdk/drag-drop";
import { FormControl, ReactiveFormsModule, Validators } from "@angular/forms";
import { compatForm } from "@angular/forms/signals/compat";
import { rxResource, toSignal } from "@angular/core/rxjs-interop";
import { TranslocoPipe, TranslocoService } from "@jsverse/transloco";
import { DialogModule } from "primeng/dialog";
import { ButtonModule } from "primeng/button";
import { DividerModule } from "primeng/divider";
import { IconFieldModule } from "primeng/iconfield";
import { InputIconModule } from "primeng/inputicon";
import { InputGroupModule } from "primeng/inputgroup";
import { InputGroupAddonModule } from "primeng/inputgroupaddon";
import { InputTextModule } from "primeng/inputtext";
import { SelectButtonModule } from "primeng/selectbutton";
import { TableModule } from "primeng/table";
import { TagModule } from "primeng/tag";
import { TooltipModule } from "primeng/tooltip";
import { AnalyticsService } from "@services/analytics.service";
import { Funnel, FunnelStep } from "@models/analytics.types";

interface FunnelStepControl {
    typeControl: FormControl<FunnelStep["type"]>;
    valueControl: FormControl<string>;
}

@Component({
    selector: "app-funnel-manager",
    imports: [DragDropModule, ReactiveFormsModule, DialogModule, ButtonModule, DividerModule, IconFieldModule, InputIconModule, InputGroupModule, InputGroupAddonModule, InputTextModule, SelectButtonModule, TableModule, TagModule, TooltipModule, TranslocoPipe],
    changeDetection: ChangeDetectionStrategy.OnPush,
    template: `
        <p-dialog [header]="'funnels.manager.dialogTitle' | transloco" [(visible)]="visible" [modal]="true" [style]="{ width: '800px', maxWidth: '90vw' }" [draggable]="false" [resizable]="false">
            <p class="text-sm text-muted-color mb-4">{{ "funnels.manager.dialogDescription" | transloco }}</p>

            <!-- Existing funnels list -->
            <div class="flex justify-end mb-3">
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
                        <tr>
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
                                    <p-button icon="pi pi-chart-bar" (onClick)="viewFunnel.emit(funnel)" styleClass="p-button-text p-button-sm" [rounded]="true" [pTooltip]="'funnels.manager.viewStats' | transloco"></p-button>
                                    <p-button icon="pi pi-trash" (onClick)="deleteFunnel(funnel.id)" styleClass="p-button-text p-button-danger p-button-sm" [rounded]="true"></p-button>
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

            <!-- Create funnel form -->
            <h4 class="font-semibold text-sm mt-0 mb-4">{{ "funnels.manager.createTitle" | transloco }}</h4>

            <div class="flex flex-col gap-4">
                <div class="flex flex-col gap-1">
                    <label for="f-name" class="text-xs font-medium">{{ "common.columns.name" | transloco }}</label>
                    <input pInputText id="f-name" [formControl]="newFunnelForm.name().control()" [placeholder]="'funnels.manager.namePlaceholder' | transloco" class="w-full" />
                </div>

                <div class="flex flex-col gap-1">
                    <label class="text-xs font-medium mb-1">{{ "funnels.manager.stepsLabel" | transloco }}</label>

                    <div cdkDropList (cdkDropListDropped)="reorderStep($event)" class="flex flex-col gap-3">
                        @for (step of stepControls(); track $index; let i = $index) {
                            <div cdkDrag class="flex gap-2 items-center">
                                <i cdkDragHandle class="pi pi-bars text-muted-color cursor-grab shrink-0"></i>
                                <span class="flex w-7 h-7 shrink-0 items-center justify-center rounded-full text-xs font-bold"
                                      [class]="i === 0 ? 'bg-primary text-primary-contrast' : 'bg-surface-200 dark:bg-surface-700 text-surface-600 dark:text-surface-300'">
                                    {{ i + 1 }}
                                </span>
                                <p-selectbutton [options]="types()" [formControl]="step.typeControl" optionLabel="label" optionValue="value" size="small" />
                                <p-inputgroup>
                                    <p-inputgroup-addon>
                                        <i [class]="step.typeControl.value === 'event' ? 'pi pi-bolt' : 'pi pi-link'"></i>
                                    </p-inputgroup-addon>
                                    <input
                                        pInputText
                                        [formControl]="step.valueControl"
                                        [placeholder]="step.typeControl.value === 'path' ? ('funnels.manager.stepPathPlaceholder' | transloco) : ('funnels.manager.stepEventPlaceholder' | transloco)"
                                    />
                                </p-inputgroup>
                                <p-button icon="pi pi-times" (onClick)="removeStep(i)" severity="danger" [text]="true" [rounded]="true" size="small" [disabled]="stepControls().length <= 2" />
                                <div *cdkDragPlaceholder class="rounded-md border-2 border-dashed border-primary/30 bg-primary/5 h-10 w-full"></div>
                            </div>
                        }
                    </div>

                    <p-button [label]="'funnels.manager.addStep' | transloco" icon="pi pi-plus" (onClick)="addStep()" [text]="true" size="small" class="mt-1" />
                </div>
            </div>

            <ng-template pTemplate="footer">
                <p-button [label]="'funnels.manager.createAction' | transloco" icon="pi pi-plus" (onClick)="createFunnel()" [loading]="creating()" [disabled]="!canCreate()" size="small" />
            </ng-template>
        </p-dialog>
    `
})
export class FunnelManager {
    visible = model(false);
    readonly siteId = input<string | null>(null);
    readonly funnelsChanged = output();
    readonly viewFunnel = output<Funnel>();

    private analyticsService = inject(AnalyticsService);
    private transloco = inject(TranslocoService);
    private activeLanguage = toSignal(this.transloco.langChanges$, { initialValue: this.transloco.getActiveLang() });

    creating = signal(false);

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
            { label: this.transloco.translate("funnels.manager.typePagePath"), value: "path" },
            { label: this.transloco.translate("funnels.manager.typeCustomEvent"), value: "event" }
        ];
    });

    private readonly newFunnelModel = signal({
        name: new FormControl("", { nonNullable: true, validators: [Validators.required] })
    });
    protected readonly newFunnelForm = compatForm(this.newFunnelModel);
    protected readonly stepControls = signal<FunnelStepControl[]>([this.createStepControl(), this.createStepControl()]);

    protected readonly canCreate = computed(() => {
        const name = this.newFunnelForm.name().value().trim();
        const steps = this.stepControls();
        return !this.newFunnelForm().invalid() && name.length > 0 && steps.length >= 2 && steps.every((step) => step.valueControl.value.trim().length > 0);
    });

    addStep() {
        this.stepControls.update((steps) => [...steps, this.createStepControl()]);
    }

    removeStep(index: number) {
        this.stepControls.update((steps) => (steps.length > 2 ? steps.filter((_, i) => i !== index) : steps));
    }

    reorderStep(event: CdkDragDrop<FunnelStepControl[]>) {
        this.stepControls.update((steps) => {
            const reordered = [...steps];
            moveItemInArray(reordered, event.previousIndex, event.currentIndex);
            return reordered;
        });
    }

    createFunnel() {
        const siteId = this.siteId();
        if (!siteId || !this.canCreate()) return;
        const payload: { name: string; steps: FunnelStep[] } = {
            name: this.newFunnelForm.name().value().trim(),
            steps: this.stepControls().map((step) => ({
                type: step.typeControl.value,
                value: step.valueControl.value.trim()
            }))
        };

        this.creating.set(true);
        this.analyticsService.createFunnel(siteId, payload).subscribe({
            next: () => {
                this.creating.set(false);
                this.newFunnelForm.name().control().reset("");
                this.stepControls.set([this.createStepControl(), this.createStepControl()]);
                this.funnelsResource.reload();
                this.funnelsChanged.emit();
            },
            error: () => this.creating.set(false)
        });
    }

    deleteFunnel(id: string) {
        const siteId = this.siteId();
        if (!siteId) return;
        this.analyticsService.deleteFunnel(siteId, id).subscribe({
            next: () => {
                this.funnelsResource.reload();
                this.funnelsChanged.emit();
            }
        });
    }

    private createStepControl(type: FunnelStep["type"] = "path", value = ""): FunnelStepControl {
        return {
            typeControl: new FormControl(type, { nonNullable: true, validators: [Validators.required] }),
            valueControl: new FormControl(value, { nonNullable: true, validators: [Validators.required] })
        };
    }
}
