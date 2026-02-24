import { ChangeDetectionStrategy, Component, inject, signal } from "@angular/core";
import { CommonModule } from "@angular/common";
import { FormControl, FormGroup, ReactiveFormsModule, Validators } from "@angular/forms";
import { finalize } from "rxjs";
import { TranslocoPipe, TranslocoService } from "@jsverse/transloco";

import { ButtonModule } from "primeng/button";
import { InputTextModule } from "primeng/inputtext";
import { TableModule } from "primeng/table";

import { IPExclusion } from "@models/analytics.types";
import { ExclusionsService } from "@services/exclusions.service";
import { RelativeDateTime } from "@components/relative-date-time/relative-date-time";

const ipOrCIDRPattern = /^(([0-9]{1,3}\.){3}[0-9]{1,3}(\/(3[0-2]|[12]?[0-9]))?|([0-9A-Fa-f:]+)(\/(12[0-8]|1[01][0-9]|[1-9]?[0-9]))?)$/;

@Component({
    selector: "app-admin-global-exclusion-settings",
    standalone: true,
    imports: [CommonModule, ReactiveFormsModule, ButtonModule, InputTextModule, TableModule, RelativeDateTime, TranslocoPipe],
    templateUrl: "./admin-global-exclusion-settings.html",
    styleUrl: "./admin-global-exclusion-settings.css",
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class AdminGlobalExclusionSettings {
    private exclusionsService = inject(ExclusionsService);
    private transloco = inject(TranslocoService);

    protected readonly exclusions = signal<IPExclusion[]>([]);
    protected readonly isLoading = signal(false);
    protected readonly isSaving = signal(false);
    protected readonly error = signal<string | null>(null);
    protected readonly isCurrentIPLoading = signal(false);
    protected readonly currentIPCIDR = signal("");

    protected readonly form = new FormGroup({
        cidr: new FormControl("", { nonNullable: true, validators: [Validators.required, Validators.pattern(ipOrCIDRPattern)] }),
        description: new FormControl("", { nonNullable: true, validators: [Validators.maxLength(255)] })
    });

    constructor() {
        this.loadCurrentIP();
        this.loadExclusions();
    }

    protected addRule(): void {
        if (this.form.invalid) {
            this.form.markAllAsTouched();
            return;
        }

        this.error.set(null);
        this.isSaving.set(true);

        this.exclusionsService
            .createInstanceExclusion({
                cidr: this.form.controls.cidr.value.trim(),
                description: this.form.controls.description.value.trim()
            })
            .pipe(finalize(() => this.isSaving.set(false)))
            .subscribe({
                next: (rule) => {
                    this.exclusions.update((current) => [rule, ...current]);
                    this.form.reset({ cidr: "", description: "" });
                },
                error: () => {
                    this.error.set("admin.exclusions.errors.createFailed");
                }
            });
    }

    protected deleteRule(rule: IPExclusion): void {
        const message = this.transloco.translate("admin.exclusions.confirmDelete", { cidr: rule.cidr });
        if (!window.confirm(message)) {
            return;
        }

        this.error.set(null);
        this.exclusionsService.deleteInstanceExclusion(rule.id).subscribe({
            next: () => {
                this.exclusions.update((current) => current.filter((entry) => entry.id !== rule.id));
            },
            error: () => {
                this.error.set("admin.exclusions.errors.deleteFailed");
            }
        });
    }

    protected reload(): void {
        this.loadExclusions();
    }

    protected copyCurrentIP(): void {
        const cidr = this.currentIPCIDR();
        if (!cidr || typeof navigator === "undefined" || !navigator.clipboard) {
            return;
        }

        navigator.clipboard.writeText(cidr).catch(() => {
            this.error.set("admin.exclusions.errors.copyFailed");
        });
    }

    private loadCurrentIP(): void {
        this.isCurrentIPLoading.set(true);
        this.currentIPCIDR.set("");

        this.exclusionsService
            .getCurrentIP()
            .pipe(finalize(() => this.isCurrentIPLoading.set(false)))
            .subscribe({
                next: (currentIP) => {
                    this.currentIPCIDR.set(currentIP.cidr);
                },
                error: () => {
                    this.currentIPCIDR.set("");
                }
            });
    }

    private loadExclusions(): void {
        this.isLoading.set(true);
        this.error.set(null);

        this.exclusionsService
            .listInstanceExclusions()
            .pipe(finalize(() => this.isLoading.set(false)))
            .subscribe({
                next: (rules) => {
                    this.exclusions.set(rules);
                },
                error: () => {
                    this.error.set("admin.exclusions.errors.loadFailed");
                }
            });
    }
}
