import { Component, Output, EventEmitter, inject, signal, OnChanges, computed, input, model } from "@angular/core";

import { toSignal } from "@angular/core/rxjs-interop";
import { FormControl, ReactiveFormsModule, Validators } from "@angular/forms";
import { compatForm } from "@angular/forms/signals/compat";
import { TranslocoPipe, TranslocoService } from "@jsverse/transloco";
import { DialogModule } from "primeng/dialog";
import { ButtonModule } from "primeng/button";
import { InputTextModule } from "primeng/inputtext";
import { SelectModule } from "primeng/select";
import { TableModule } from "primeng/table";
import { AnalyticsService } from "@services/analytics.service";
import { Goal } from "@models/analytics.types";

@Component({
    selector: "app-goal-manager",
    standalone: true,
    imports: [ReactiveFormsModule, DialogModule, ButtonModule, InputTextModule, SelectModule, TableModule, TranslocoPipe],
    template: `
        <p-dialog [header]="'goals.manager.dialogTitle' | transloco" [(visible)]="visible" [modal]="true" [style]="{ width: '600px', maxWidth: '90vw' }" [draggable]="false" [resizable]="false" (onHide)="onHide()">
            <p class="text-sm text-muted-color mb-4">{{ "goals.manager.dialogDescription" | transloco }}</p>

            <!-- List Goals -->
            <div class="flex justify-end mb-3">
                <span class="p-input-icon-left hk-crud-search">
                    <i class="pi pi-search"></i>
                    <input pInputText #goalSearch [placeholder]="'common.searchPlaceholder' | transloco" (input)="goalsTable.filterGlobal($any($event.target).value, 'contains')" class="w-full" />
                </span>
            </div>

            <div class="hk-crud-table-wrap mb-6">
                <p-table #goalsTable [value]="goals()" [loading]="loading()" styleClass="hk-crud-table p-datatable-sm" [rowHover]="true" [globalFilterFields]="['name', 'type', 'value']" [sortField]="'name'" [sortOrder]="1">
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
                            <th style="width: 4rem"></th>
                        </tr>
                    </ng-template>
                    <ng-template pTemplate="body" let-goal>
                        <tr>
                            <td class="font-medium">{{ goal.name }}</td>
                            <td>
                                <span [class]="goalTypeClass(goal.type)">
                                    {{ goalTypeLabel(goal.type) }}
                                </span>
                            </td>
                            <td class="font-mono text-sm text-muted-color truncate max-w-[150px]" [title]="goal.value">{{ goal.value }}</td>
                            <td>
                                <p-button icon="pi pi-trash" (onClick)="deleteGoal(goal.id)" styleClass="p-button-text p-button-danger p-button-sm" [rounded]="true"></p-button>
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

            <!-- Add Goal Form -->
            <div class="flex flex-col gap-4 p-4 border border-surface-200 dark:border-surface-700 rounded-lg bg-surface-50 dark:bg-surface-900">
                <div class="flex items-center justify-between">
                    <h4 class="font-semibold text-sm m-0">{{ "goals.manager.addTitle" | transloco }}</h4>
                </div>

                <div class="grid grid-cols-1 gap-3">
                    <div class="flex flex-col gap-1">
                        <label for="g-name" class="text-xs font-medium">{{ "common.columns.name" | transloco }}</label>
                        <input pInputText id="g-name" [formControl]="newGoalForm.name().control()" [placeholder]="'goals.manager.namePlaceholder' | transloco" class="p-inputtext-sm w-full" />
                    </div>

                    <div class="grid grid-cols-1 md:grid-cols-3 gap-3">
                        <div class="flex flex-col gap-1">
                            <label for="g-type" class="text-xs font-medium">{{ "common.columns.type" | transloco }}</label>
                            <p-select [options]="types()" [formControl]="newGoalForm.type().control()" optionLabel="label" optionValue="value" styleClass="w-full p-inputtext-sm" appendTo="body" />
                        </div>
                        <div class="flex flex-col gap-1 md:col-span-2">
                            <label for="g-value" class="text-xs font-medium">
                                {{ newGoalForm.type().value() === "path" ? ("goals.manager.urlPathLabel" | transloco) : ("goals.manager.eventNameLabel" | transloco) }}
                            </label>
                            <div class="relative">
                                <input
                                    pInputText
                                    id="g-value"
                                    [formControl]="newGoalForm.value().control()"
                                    [placeholder]="newGoalForm.type().value() === 'path' ? ('goals.manager.urlPathPlaceholder' | transloco) : ('goals.manager.eventNamePlaceholder' | transloco)"
                                    class="p-inputtext-sm w-full"
                                />
                            </div>
                            <small class="text-xs text-muted-color">
                                @if (newGoalForm.type().value() === "path") {
                                    {{ "goals.manager.urlPathHelp" | transloco }}
                                } @else {
                                    {{ "goals.manager.eventNameHelpPrefix" | transloco }} <code>hk.event('name')</code>{{ "goals.manager.eventNameHelpSuffix" | transloco }}
                                }
                            </small>
                        </div>
                    </div>
                </div>

                <div class="flex justify-end mt-2">
                    <p-button [label]="'goals.manager.createAction' | transloco" icon="pi pi-plus" (onClick)="createGoal()" [loading]="creating()" [disabled]="newGoalForm().invalid() || !isValid()" size="small"></p-button>
                </div>
            </div>
        </p-dialog>
    `
})
export class GoalManager implements OnChanges {
    visible = model(false);
    readonly siteId = input<string | null>(null);
    @Output() goalsChanged = new EventEmitter<void>();

    private analyticsService = inject(AnalyticsService);
    private transloco = inject(TranslocoService);
    private activeLanguage = toSignal(this.transloco.langChanges$, { initialValue: this.transloco.getActiveLang() });

    goals = signal<Goal[]>([]);
    loading = signal(false);
    creating = signal(false);

    protected readonly types = computed(() => {
        this.activeLanguage();
        return [
            { label: this.transloco.translate("goals.manager.typePagePath"), value: "path" },
            { label: this.transloco.translate("goals.manager.typeCustomEvent"), value: "event" }
        ];
    });

    private readonly newGoalModel = signal({
        name: new FormControl("", { nonNullable: true, validators: [Validators.required] }),
        type: new FormControl<"path" | "event">("path", { nonNullable: true, validators: [Validators.required] }),
        value: new FormControl("", { nonNullable: true, validators: [Validators.required] })
    });
    protected readonly newGoalForm = compatForm(this.newGoalModel);

    protected goalTypeClass(type: Goal["type"]) {
        const base = "text-xs font-bold px-2 py-1 rounded";
        return type === "event" ? `${base} bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300` : `${base} bg-orange-100 text-orange-700 dark:bg-orange-900 dark:text-orange-300`;
    }

    protected goalTypeLabel(type: Goal["type"]): string {
        if (type === "event") {
            return this.transloco.translate("goals.manager.typeCustomEvent");
        }
        return this.transloco.translate("goals.manager.typePagePath");
    }

    ngOnChanges() {
        if (this.visible() && this.siteId()) {
            this.loadGoals();
        }
    }

    loadGoals() {
        const siteId = this.siteId();
        if (!siteId) return;
        this.loading.set(true);
        this.analyticsService.getGoals(siteId).subscribe({
            next: (goals) => {
                this.goals.set(goals || []);
                this.loading.set(false);
            },
            error: () => this.loading.set(false)
        });
    }

    createGoal() {
        const siteId = this.siteId();
        if (!siteId || !this.isValid()) return;
        const payload = {
            name: this.newGoalForm.name().value().trim(),
            type: this.newGoalForm.type().value(),
            value: this.newGoalForm.value().value().trim()
        };

        this.creating.set(true);
        this.analyticsService.createGoal(siteId, payload).subscribe({
            next: () => {
                this.creating.set(false);
                this.newGoalForm.name().control().reset("");
                this.newGoalForm.type().control().reset("path");
                this.newGoalForm.value().control().reset("");
                this.loadGoals();
                this.goalsChanged.emit();
            },
            error: () => this.creating.set(false)
        });
    }

    deleteGoal(id: string) {
        const siteId = this.siteId();
        if (!siteId) return;
        // Optimistic update could be done here, but let's just reload for safety
        this.analyticsService.deleteGoal(siteId, id).subscribe({
            next: () => {
                this.loadGoals();
                this.goalsChanged.emit();
            }
        });
    }

    isValid() {
        return this.newGoalForm.name().value().trim().length > 0 && this.newGoalForm.value().value().trim().length > 0;
    }

    onHide() {
        this.visible.set(false);
    }
}
