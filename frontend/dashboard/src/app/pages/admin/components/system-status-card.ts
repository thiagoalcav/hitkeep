import { ChangeDetectionStrategy, Component, computed, input, output } from '@angular/core';
import { ButtonModule } from 'primeng/button';
import { TooltipModule } from 'primeng/tooltip';

type ButtonSeverity = 'secondary' | 'success' | 'info' | 'warn' | 'danger' | 'help' | 'contrast';

@Component({
    selector: 'app-system-status-card',
    imports: [ButtonModule, TooltipModule],
    templateUrl: './system-status-card.html',
    styleUrl: './system-status-card.css',
    changeDetection: ChangeDetectionStrategy.OnPush,
    host: {
        '[class.system-status-card-host--wide]': 'wide()'
    }
})
export class SystemStatusCard {
    title = input.required<string>();
    titleId = input.required<string>();
    loading = input(false);
    wide = input(false);
    refreshable = input(true);
    refreshDisabled = input(false);
    refreshLabel = input.required<string>();
    metricLabel = input('');
    metricValue = input('');
    actionLabel = input('');
    actionIcon = input('pi pi-bolt');
    actionSeverity = input<ButtonSeverity>('secondary');
    actionLoading = input(false);
    actionDisabled = input(false);

    refreshClicked = output<void>();
    actionClicked = output<void>();

    protected hasMetric = computed(() => this.metricLabel().length > 0 || this.metricValue().length > 0);
    protected hasAction = computed(() => this.actionLabel().length > 0);
}
