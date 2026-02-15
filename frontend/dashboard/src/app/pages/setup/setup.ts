import { Component, inject, signal } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { Router } from '@angular/router';
import { FormControl, Validators, ReactiveFormsModule } from '@angular/forms';
import { compatForm } from '@angular/forms/signals/compat';
import { finalize } from 'rxjs/operators';
import { TranslocoPipe } from '@jsverse/transloco';

// PrimeNG Imports
import { PasswordModule } from 'primeng/password';
import { ButtonModule } from 'primeng/button';
import { InputTextModule } from 'primeng/inputtext';

// Corrected path to Core
import { Brand } from '@components/brand/brand';

@Component({
    selector: 'app-setup',
    standalone: true,
    imports: [Brand, ReactiveFormsModule, PasswordModule, ButtonModule, InputTextModule, TranslocoPipe],
    templateUrl: './setup.html',
    styleUrl: './setup.css'
})
export class Setup {
    private http = inject(HttpClient);
    private router = inject(Router);

    protected isLoading = signal(false);
    protected errorMessage = signal<string | null>(null);

    private readonly setupModel = signal({
        email: new FormControl('', { nonNullable: true, validators: [Validators.required, Validators.email] }),
        password: new FormControl('', { nonNullable: true, validators: [Validators.required, Validators.minLength(8)] })
    });
    protected readonly setupForm = compatForm(this.setupModel);

    onSubmit(event?: Event): void {
        event?.preventDefault();
        if (this.setupForm().invalid()) {
            this.setupForm.email().markAsTouched();
            this.setupForm.password().markAsTouched();
            return;
        }

        this.isLoading.set(true);
        this.errorMessage.set(null);

        const email = this.setupForm.email().value();
        const password = this.setupForm.password().value();

        this.http
            .post('/api/initial-user', { email, password })
            .pipe(finalize(() => this.isLoading.set(false)))
            .subscribe({
                next: () => {
                    this.router.navigate(['/login']);
                },
                error: (err) => {
                    console.error('Failed to create initial user:', err);
                    this.errorMessage.set('setup.errors.unexpected');
                }
            });
    }
}
