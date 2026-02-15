import { Component, inject, signal } from '@angular/core';
import { CommonModule } from '@angular/common';
import { Router, RouterLink } from '@angular/router';
import { FormControl, ReactiveFormsModule, Validators } from '@angular/forms';
import { compatForm } from '@angular/forms/signals/compat';
import { finalize } from 'rxjs/operators';
import { TranslocoPipe } from '@jsverse/transloco';

// PrimeNG Imports
import { PasswordModule } from 'primeng/password';
import { ButtonModule } from 'primeng/button';
import { InputTextModule } from 'primeng/inputtext';
import { CheckboxModule } from 'primeng/checkbox';

// Corrected path to Core
import { Brand } from '@components/brand/brand';
import { AuthService } from '@services/auth.service';
import { UserPreferencesService } from '@services/user-preferences.service';

@Component({
    selector: 'app-login',
    standalone: true,
    imports: [Brand, CommonModule, ReactiveFormsModule, PasswordModule, ButtonModule, InputTextModule, CheckboxModule, RouterLink, TranslocoPipe],
    templateUrl: './login.html',
    styleUrl: './login.css'
})
export class Login {
    private router = inject(Router);
    private auth = inject(AuthService);
    private preferences = inject(UserPreferencesService);
    protected isLoading = signal(false);
    protected errorMessage = signal<string | null>(null);
    protected currentYear = new Date().getFullYear();

    private readonly loginModel = signal({
        email: new FormControl('', { nonNullable: true, validators: [Validators.required, Validators.email] }),
        password: new FormControl('', { nonNullable: true, validators: [Validators.required] }),
        rememberMe: new FormControl(false, { nonNullable: true })
    });
    protected readonly loginForm = compatForm(this.loginModel);

    onSubmit(event?: Event): void {
        event?.preventDefault();
        if (this.loginForm().invalid()) {
            this.loginForm.email().markAsTouched();
            this.loginForm.password().markAsTouched();
            return;
        }

        this.isLoading.set(true);
        this.errorMessage.set(null);

        const email = this.loginForm.email().value();
        const password = this.loginForm.password().value();
        const rememberMe = this.loginForm.rememberMe().value();

        this.auth
            .login({ email, password, remember_me: rememberMe })
            .pipe(finalize(() => this.isLoading.set(false)))
            .subscribe({
                next: () => {
                    this.redirectAfterLogin();
                },
                error: (err) => {
                    console.error('Login failed:', err);
                    if (err.status === 401) {
                        this.errorMessage.set('login.errors.invalidCredentials');
                    } else {
                        this.errorMessage.set('login.errors.unexpected');
                    }
                }
            });
    }

    private redirectAfterLogin() {
        this.preferences.load().subscribe({
            next: () => {
                this.router.navigate(['/dashboard']);
            },
            error: () => {
                this.router.navigate(['/dashboard']);
            }
        });
    }
}
