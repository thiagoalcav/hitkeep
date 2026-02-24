import { ChangeDetectionStrategy, Component, effect, input, output, signal } from "@angular/core";
import { FormControl, ReactiveFormsModule } from "@angular/forms";
import { compatForm } from "@angular/forms/signals/compat";
import { TranslocoPipe } from "@jsverse/transloco";
import { ButtonModule } from "primeng/button";
import { SelectModule } from "primeng/select";
import { TooltipModule } from "primeng/tooltip";

export interface RangeOption {
    label: string;
    value: string;
}

export interface RangeSelectEvent {
    value: RangeOption;
}

@Component({
    selector: "app-range-toolbar",
    imports: [ReactiveFormsModule, SelectModule, ButtonModule, TooltipModule, TranslocoPipe],
    templateUrl: "./range-toolbar.html",
    styleUrl: "./range-toolbar.css",
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class RangeToolbar {
    timeRanges = input.required<RangeOption[]>();
    selectedRange = input.required<RangeOption>();
    loading = input<boolean>(false);

    selectedRangeChange = output<RangeOption>();
    rangeChange = output<RangeSelectEvent>();
    refresh = output<void>();

    private readonly rangeModel = signal({
        selectedValue: new FormControl("", { nonNullable: true })
    });
    protected readonly rangeForm = compatForm(this.rangeModel);

    constructor() {
        effect(() => {
            const selected = this.selectedRange();
            this.rangeForm.selectedValue().control().setValue(selected.value, { emitEvent: false });
        });
    }

    protected handleRangeChange(event: { value: string }) {
        const selected = this.timeRanges().find((option) => option.value === event.value);
        if (!selected) {
            return;
        }
        this.selectedRangeChange.emit(selected);
        this.rangeChange.emit({ value: selected });
    }
}
