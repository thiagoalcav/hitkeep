import { ChangeDetectionStrategy, Component, input, output } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { ButtonModule } from 'primeng/button';
import { SelectModule } from 'primeng/select';
import { TooltipModule } from 'primeng/tooltip';

export interface RangeOption {
    label: string;
    value: string;
}

export interface RangeSelectEvent {
    value: RangeOption;
}

@Component({
    selector: 'app-range-toolbar',
    imports: [FormsModule, SelectModule, ButtonModule, TooltipModule],
    templateUrl: './range-toolbar.html',
    styleUrl: './range-toolbar.css',
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class RangeToolbar {
    timeRanges = input.required<RangeOption[]>();
    selectedRange = input.required<RangeOption>();
    loading = input<boolean>(false);

    selectedRangeChange = output<RangeOption>();
    rangeChange = output<RangeSelectEvent>();
    refresh = output<void>();

    protected handleRangeChange(event: RangeSelectEvent) {
        this.selectedRangeChange.emit(event.value);
        this.rangeChange.emit(event);
    }
}
