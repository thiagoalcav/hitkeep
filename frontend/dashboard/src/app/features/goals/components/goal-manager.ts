import { ChangeDetectionStrategy, Component, computed, inject, input, model, output, signal } from "@angular/core";

import { rxResource, toSignal } from "@angular/core/rxjs-interop";
import { FormControl, ReactiveFormsModule, Validators } from "@angular/forms";
import { compatForm } from "@angular/forms/signals/compat";
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
import { AnalyticsService } from "@services/analytics.service";
import { Goal } from "@models/analytics.types";

@Component({
    selector: "app-goal-manager",
    imports: [ReactiveFormsModule, DialogModule, ButtonModule, DividerModule, IconFieldModule, InputIconModule, InputGroupModule, InputGroupAddonModule, InputTextModule, SelectButtonModule, TableModule, TagModule, TranslocoPipe],
    changeDetection: ChangeDetectionStrategy.OnPush,
    template: `
        <p-dialog [header]="'goals.manager.dialogTitle' | transloco" [(visible)]="visible" [modal]="true" [style]="{ width: '640px', maxWidth: '90vw' }" [draggable]="false" [resizable]="false">
            <p class="text-sm text-muted-color mb-4">{{ "goals.manager.dialogDescription" | transloco }}</p>

            <!-- Existing goals list -->
            <div class="flex justify-end mb-3">
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
                            <th style="width: 4rem"></th>
                        </tr>
                    </ng-template>
                    <ng-template pTemplate="body" let-goal>
                        <tr>
                            <td class="font-medium">{{ goal.name }}</td>
                            <td>
                                <p-tag [value]="goalTypeLabel(goal.type)" [severity]="goal.type === 'event' ? 'info' : 'warn'" [rounded]="true" />
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

            <p-divider />

            <!-- Add goal form -->
            <h4 class="font-semibold text-sm mt-0 mb-4">{{ "goals.manager.addTitle" | transloco }}</h4>

            <div class="flex flex-col gap-4">
                <div class="flex flex-col gap-1">
                    <label for="g-name" class="text-xs font-medium">{{ "common.columns.name" | transloco }}</label>
                    <input pInputText id="g-name" [formControl]="newGoalForm.name().control()" [placeholder]="'goals.manager.namePlaceholder' | transloco" class="w-full" />
                </div>

                <div class="flex flex-col gap-1">
                    <label class="text-xs font-medium">{{ "common.columns.type" | transloco }}</label>
                    <p-selectbutton [options]="types()" [formControl]="newGoalForm.type().control()" optionLabel="label" optionValue="value" size="small" />
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
                        <small class="text-xs text-muted-color">
                            {{ "goals.manager.eventNameHelpPrefix" | transloco }} <code>hk.event('name')</code>{{ "goals.manager.eventNameHelpSuffix" | transloco }}
                        </small>
                    }
                </div>
            </div>

            <ng-template pTemplate="footer">
                <p-button [label]="'goals.manager.createAction' | transloco" icon="pi pi-plus" (onClick)="createGoal()" [loading]="creating()" [disabled]="!canCreate()" size="small" />
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

    creating = signal(false);

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
            { label: this.transloco.translate("goals.manager.typePagePath"), value: "path", icon: "pi pi-link" },
            { label: this.transloco.translate("goals.manager.typeCustomEvent"), value: "event", icon: "pi pi-bolt" }
        ];
    });

    private readonly newGoalModel = signal({
        name: new FormControl("", { nonNullable: true, validators: [Validators.required] }),
        type: new FormControl<"path" | "event">("path", { nonNullable: true, validators: [Validators.required] }),
        value: new FormControl("", { nonNullable: true, validators: [Validators.required] })
    });
    protected readonly newGoalForm = compatForm(this.newGoalModel);

    protected readonly canCreate = computed(() => {
        const name = this.newGoalForm.name().value().trim();
        const value = this.newGoalForm.value().value().trim();
        return !this.newGoalForm().invalid() && name.length > 0 && value.length > 0;
    });

    protected goalTypeLabel(type: Goal["type"]): string {
        if (type === "event") {
            return this.transloco.translate("goals.manager.typeCustomEvent");
        }
        return this.transloco.translate("goals.manager.typePagePath");
    }

    createGoal() {
        const siteId = this.siteId();
        if (!siteId || !this.canCreate()) return;
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
                this.goalsResource.reload();
                this.goalsChanged.emit();
            },
            error: () => this.creating.set(false)
        });
    }

    deleteGoal(id: string) {
        const siteId = this.siteId();
        if (!siteId) return;
        this.analyticsService.deleteGoal(siteId, id).subscribe({
            next: () => {
                this.goalsResource.reload();
                this.goalsChanged.emit();
            }
        });
    }
}
