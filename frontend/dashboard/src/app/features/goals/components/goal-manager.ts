import { Component, Input, Output, EventEmitter, inject, signal, effect, OnChanges } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { DialogModule } from 'primeng/dialog';
import { ButtonModule } from 'primeng/button';
import { InputTextModule } from 'primeng/inputtext';
import { SelectModule } from 'primeng/select';
import { TableModule } from 'primeng/table';
import { AnalyticsService } from '../../../core/services/analytics.service';
import { Goal } from '../../../core/models/analytics.types';

@Component({
  selector: 'app-goal-manager',
  standalone: true,
  imports: [CommonModule, FormsModule, DialogModule, ButtonModule, InputTextModule, SelectModule, TableModule],
  template: `
    <p-dialog header="Manage Goals" [(visible)]="visible" [modal]="true" [style]="{width: '600px', maxWidth: '90vw'}" [draggable]="false" [resizable]="false" (onHide)="onHide()">
        <p class="text-sm text-muted-color mb-4">Define conversion goals to track specific user actions or page visits.</p>

        <!-- List Goals -->
        <p-table [value]="goals()" [loading]="loading()" styleClass="p-datatable-sm mb-6" [rowHover]="true">
            <ng-template pTemplate="header">
                <tr>
                    <th>Name</th>
                    <th style="width: 100px">Type</th>
                    <th>Value</th>
                    <th style="width: 4rem"></th>
                </tr>
            </ng-template>
            <ng-template pTemplate="body" let-goal>
                <tr>
                    <td class="font-medium">{{goal.name}}</td>
                    <td>
                        <span [class]="goalTypeClass(goal.type)">
                            {{goal.type}}
                        </span>
                    </td>
                    <td class="font-mono text-sm text-muted-color truncate max-w-[150px]" [title]="goal.value">{{goal.value}}</td>
                    <td>
                        <p-button icon="pi pi-trash" (onClick)="deleteGoal(goal.id)" styleClass="p-button-text p-button-danger p-button-sm" [rounded]="true"></p-button>
                    </td>
                </tr>
            </ng-template>
            <ng-template pTemplate="emptymessage">
                <tr>
                    <td colspan="4" class="text-center text-muted-color py-4">No goals defined yet.</td>
                </tr>
            </ng-template>
        </p-table>

        <!-- Add Goal Form -->
<div class="flex flex-col gap-4 p-4 border border-surface-200 dark:border-surface-700 rounded-lg bg-surface-50 dark:bg-surface-900">
    <div class="flex items-center justify-between">
        <h4 class="font-semibold text-sm m-0">Add New Goal</h4>
    </div>
    
    <div class="grid grid-cols-1 gap-3">
        <div class="flex flex-col gap-1">
            <label for="g-name" class="text-xs font-medium">Name</label>
            <input pInputText id="g-name" [(ngModel)]="newGoal.name" placeholder="e.g. Signup Success" class="p-inputtext-sm w-full" />
        </div>
        
        <div class="grid grid-cols-1 md:grid-cols-3 gap-3">
            <div class="flex flex-col gap-1">
                <label for="g-type" class="text-xs font-medium">Type</label>
                <p-select [options]="types" [(ngModel)]="newGoal.type" optionLabel="label" optionValue="value" styleClass="w-full p-inputtext-sm" appendTo="body" />
            </div>
            <div class="flex flex-col gap-1 md:col-span-2">
                <label for="g-value" class="text-xs font-medium">
                    {{ newGoal.type === 'path' ? 'URL Path' : 'Event Name' }}
                </label>
                <div class="relative">
                    <input pInputText id="g-value" [(ngModel)]="newGoal.value" 
                           [placeholder]="newGoal.type === 'path' ? '/thank-you' : 'signup_completed'" 
                           class="p-inputtext-sm w-full" />
                </div>
                <small class="text-xs text-muted-color">
                    @if (newGoal.type === 'path') {
                        Triggers when a user visits a specific URL.
                    } @else {
                        Triggers when you call <code>hk.event('name')</code>.
                    }
                </small>
            </div>
        </div>
    </div>
    
    <div class="flex justify-end mt-2">
        <p-button label="Create Goal" icon="pi pi-plus" (onClick)="createGoal()" [loading]="creating()" [disabled]="!isValid()" size="small"></p-button>
    </div>
</div>
    </p-dialog>
  `
})
export class GoalManager implements OnChanges {
  @Input() visible = false;
  @Output() visibleChange = new EventEmitter<boolean>();
  @Input() siteId: string | null = null;
  @Output() goalsChanged = new EventEmitter<void>();

  private analyticsService = inject(AnalyticsService);

  goals = signal<Goal[]>([]);
  loading = signal(false);
  creating = signal(false);

  types = [
    { label: 'Page Path', value: 'path' },
    { label: 'Custom Event', value: 'event' }
  ];

  newGoal: { name: string; type: 'path' | 'event'; value: string } = {
    name: '',
    type: 'path',
    value: ''
  };
  protected goalTypeClass(type: Goal['type']) {
    const base = 'text-xs font-bold px-2 py-1 rounded';
    return type === 'event'
      ? `${base} bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300`
      : `${base} bg-orange-100 text-orange-700 dark:bg-orange-900 dark:text-orange-300`;
  }

  ngOnChanges() {
    if (this.visible && this.siteId) {
      this.loadGoals();
    }
  }

  loadGoals() {
    if (!this.siteId) return;
    this.loading.set(true);
    this.analyticsService.getGoals(this.siteId).subscribe({
      next: (goals) => {
        this.goals.set(goals || []);
        this.loading.set(false);
      },
      error: () => this.loading.set(false)
    });
  }

  createGoal() {
    if (!this.siteId || !this.isValid()) return;

    this.creating.set(true);
    this.analyticsService.createGoal(this.siteId, this.newGoal).subscribe({
      next: () => {
        this.creating.set(false);
        this.newGoal = { name: '', type: 'path', value: '' };
        this.loadGoals();
        this.goalsChanged.emit();
      },
      error: () => this.creating.set(false)
    });
  }

  deleteGoal(id: string) {
    if (!this.siteId) return;
    // Optimistic update could be done here, but let's just reload for safety
    this.analyticsService.deleteGoal(this.siteId, id).subscribe({
      next: () => {
        this.loadGoals();
        this.goalsChanged.emit();
      }
    });
  }

  isValid() {
    return this.newGoal.name.length > 0 && this.newGoal.value.length > 0;
  }

  onHide() {
    this.visible = false;
    this.visibleChange.emit(false);
  }
}
