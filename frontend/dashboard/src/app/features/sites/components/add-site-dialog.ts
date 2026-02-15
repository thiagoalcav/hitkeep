import { Component, inject, model, signal } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ReactiveFormsModule, FormControl, Validators, AbstractControl, ValidationErrors } from '@angular/forms';
import { compatForm } from '@angular/forms/signals/compat';
import { TranslocoPipe } from '@jsverse/transloco';
import { DialogModule } from 'primeng/dialog';
import { ButtonModule } from 'primeng/button';
import { InputTextModule } from 'primeng/inputtext';
import { MessageModule } from 'primeng/message';
import { SiteService } from '@features/sites/services/site.service';
@Component({
    selector: 'app-add-site-dialog',
    standalone: true,
    imports: [CommonModule, ReactiveFormsModule, DialogModule, ButtonModule, InputTextModule, MessageModule, TranslocoPipe],
    template: `
        <p-dialog [header]="'sites.addDialog.title' | transloco" [(visible)]="visible" [modal]="true" [style]="{ width: '450px', maxWidth: '90vw' }" (onHide)="resetForm()">
            <form (submit)="onSubmit($event)" class="flex flex-col gap-6 pt-2" novalidate>
                <!-- Instructions -->
                <div class="bg-[var(--p-surface-50)] dark:bg-[var(--p-surface-800)] p-3 rounded-md border border-[var(--p-surface-border)] flex gap-3">
                    <i class="pi pi-info-circle text-[var(--p-primary-color)] mt-0.5"></i>
                    <div class="text-sm text-[var(--p-text-muted-color)] leading-relaxed">
                        {{ 'sites.addDialog.instructionsLine1' | transloco: { apex: 'example.com', subdomain: 'blog.example.com' } }}<br />
                        {{ 'sites.addDialog.instructionsLine2' | transloco: { www: 'www' } }}
                    </div>
                </div>

                <div class="flex flex-col gap-2">
                    <label for="domain" class="font-semibold text-sm text-[var(--p-text-color)]">{{ 'sites.addDialog.domainLabel' | transloco }}</label>
                    <input pInputText id="domain" [formControl]="form.domain().control()" [placeholder]="'sites.addDialog.domainPlaceholder' | transloco" class="w-full" (blur)="sanitizeInput()" [class.ng-invalid]="isInvalid()" [class.ng-dirty]="form.domain().dirty()" />

                    <!-- Validation Messages -->
                    @if (isInvalid()) {
                        @if (form.domain().control().hasError('required')) {
                            <small class="text-red-500">{{ 'sites.addDialog.errors.domainRequired' | transloco }}</small>
                        }
                        @if (form.domain().control().hasError('pattern')) {
                            <small class="text-red-500">{{ 'sites.addDialog.errors.domainInvalid' | transloco }}</small>
                        }
                        @if (form.domain().control().hasError('containsProtocol')) {
                            <small class="text-red-500">{{ 'sites.addDialog.errors.removeProtocol' | transloco }}</small>
                        }
                        @if (form.domain().control().hasError('containsWww')) {
                            <small class="text-red-500">{{ 'sites.addDialog.errors.removeWww' | transloco }}</small>
                        }
                    }
                    @if (createError()) {
                        <small class="text-red-500">{{ createError() | transloco }}</small>
                    }
                </div>
            </form>

            <ng-template pTemplate="footer">
                <p-button [label]="'common.actions.cancel' | transloco" (onClick)="visible.set(false)" styleClass="p-button-text" />
                <p-button [label]="'sites.addDialog.addAction' | transloco" (onClick)="onSubmit()" [loading]="isSubmitting()" [disabled]="isSubmitting() || form().invalid()" />
            </ng-template>
        </p-dialog>
    `
})
export class AddSiteDialog {
    visible = model<boolean>(false);
    private siteService = inject(SiteService);
    protected isSubmitting = signal(false);
    protected createError = signal<string | null>(null);
    private readonly formModel = signal({
        domain: new FormControl('', { nonNullable: true, validators: [Validators.required, this.domainValidator] })
    });
    protected readonly form = compatForm(this.formModel);
    private domainValidator(control: AbstractControl): ValidationErrors | null {
        const value = control.value as string;
        if (!value) return null;

        if (value.startsWith('http://') || value.startsWith('https://')) {
            return { containsProtocol: true };
        }

        if (value.startsWith('www.')) {
            return { containsWww: true };
        }

        const domainRegex = /^[a-zA-Z0-9][a-zA-Z0-9-]{1,61}[a-zA-Z0-9](?:\.[a-zA-Z]{2,})+$/;
        if (!domainRegex.test(value)) {
            return { pattern: true };
        }

        return null;
    }
    sanitizeInput() {
        let val = this.form.domain().value();
        val = val.toLowerCase().trim();

        val = val.replace(/^https?:\/\//, '');
        val = val.replace(/\/$/, '');

        this.form.domain().control().setValue(val);
    }
    protected isInvalid() {
        return this.form.domain().invalid() && (this.form.domain().dirty() || this.form.domain().touched());
    }
    resetForm() {
        this.form.domain().control().reset('');
        this.createError.set(null);
        this.isSubmitting.set(false);
    }
    onSubmit(event?: Event) {
        event?.preventDefault();
        if (this.form().invalid()) {
            this.form.domain().markAsTouched();
            return;
        }

        const domain = this.form.domain().value();
        if (!domain) {
            this.isSubmitting.set(false);
            return;
        }
        this.isSubmitting.set(true);
        this.createError.set(null);

        this.siteService.createSite(domain).subscribe({
            next: () => {
                this.visible.set(false);
            },
            error: () => {
                this.createError.set('sites.addDialog.errors.createFailed');
                this.isSubmitting.set(false);
            }
        });
    }
}
