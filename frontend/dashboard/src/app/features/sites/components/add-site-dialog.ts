import { Component, inject, model, signal } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ReactiveFormsModule, FormBuilder, Validators, AbstractControl, ValidationErrors } from '@angular/forms';
import { DialogModule } from 'primeng/dialog';
import { ButtonModule } from 'primeng/button';
import { InputTextModule } from 'primeng/inputtext';
import { MessageModule } from 'primeng/message';
import { SiteService } from '../services/site.service';
@Component({
  selector: 'app-add-site-dialog',
  standalone: true,
  imports: [CommonModule, ReactiveFormsModule, DialogModule, ButtonModule, InputTextModule, MessageModule],
  template: `
    <p-dialog
      header="Add Site"
      [(visible)]="visible"
      [modal]="true"
      [style]="{ width: '450px', maxWidth: '90vw' }"
      (onHide)="resetForm()">
      <form [formGroup]="form" (ngSubmit)="onSubmit()" class="flex flex-col gap-6 pt-2">

        <!-- Instructions -->
        <div class="bg-[var(--p-surface-50)] dark:bg-[var(--p-surface-800)] p-3 rounded-md border border-[var(--p-surface-border)] flex gap-3">
          <i class="pi pi-info-circle text-[var(--p-primary-color)] mt-0.5"></i>
          <div class="text-sm text-[var(--p-text-muted-color)] leading-relaxed">
            Enter your root or subdomain (e.g., <strong>example.com</strong>, <strong>blog.example.com</strong>).<br>
            We automatically track <strong>www</strong> with your apex domain.
          </div>
        </div>

        <div class="flex flex-col gap-2">
          <label for="domain" class="font-semibold text-sm text-[var(--p-text-color)]">Domain Name</label>
          <input
            pInputText
            id="domain"
            formControlName="domain"
            placeholder="example.com"
            class="w-full"
            (blur)="sanitizeInput()"
            [class.ng-invalid]="isInvalid()"
            [class.ng-dirty]="form.get('domain')?.dirty" />

          <!-- Validation Messages -->
          @if (isInvalid()) {
            @if (form.get('domain')?.hasError('required')) {
              <small class="text-red-500">Domain is required.</small>
            }
            @if (form.get('domain')?.hasError('pattern')) {
              <small class="text-red-500">Invalid domain format.</small>
            }
            @if (form.get('domain')?.hasError('containsProtocol')) {
              <small class="text-red-500">Please remove http:// or https://</small>
            }
            @if (form.get('domain')?.hasError('containsWww')) {
              <small class="text-red-500">Please remove 'www.'. Enter the root domain only.</small>
            }
          }
          @if (createError()) {
            <small class="text-red-500">{{ createError() }}</small>
          }
        </div>
      </form>

      <ng-template pTemplate="footer">
        <p-button label="Cancel" (onClick)="visible.set(false)" styleClass="p-button-text" />
        <p-button label="Add Site" (onClick)="onSubmit()" [loading]="isSubmitting()" [disabled]="form.invalid" />
      </ng-template>
    </p-dialog>
  `
})
export class AddSiteDialog {
  visible = model<boolean>(false);
  private fb = inject(FormBuilder);
  private siteService = inject(SiteService);
  protected isSubmitting = signal(false);
  protected createError = signal<string | null>(null);
  protected form = this.fb.group({
    domain: ['', [
      Validators.required,
      this.domainValidator
    ]]
  });
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
    let val = this.form.get('domain')?.value || '';
    val = val.toLowerCase().trim();

    val = val.replace(/^https?:\/\//, '');
    val = val.replace(/\/$/, '');

    this.form.get('domain')?.setValue(val);
  }
  protected isInvalid() {
    return this.form.get('domain')?.invalid && (this.form.get('domain')?.dirty || this.form.get('domain')?.touched);
  }
  resetForm() {
    this.form.reset();
    this.createError.set(null);
    this.isSubmitting.set(false);
  }
  onSubmit() {
    if (this.form.invalid) {
      this.form.markAllAsTouched();
      return;
    }

    const domainControl = this.form.get('domain');
    const domain = domainControl?.value ?? '';
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
        this.createError.set('Failed to create site. Domain might already exist.');
        this.isSubmitting.set(false);
      }
    });
  }
}
