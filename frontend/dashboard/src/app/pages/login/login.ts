import { Component, inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import { Router, RouterLink } from '@angular/router';
import { FormBuilder, FormGroup, Validators, ReactiveFormsModule } from '@angular/forms';
import { finalize } from 'rxjs/operators';

// PrimeNG Imports
import { PasswordModule } from 'primeng/password';
import { ButtonModule } from 'primeng/button';
import { InputTextModule } from 'primeng/inputtext';
import { CheckboxModule } from 'primeng/checkbox';

// Corrected path to Core
import { Brand } from '@components/brand/brand';
import { AuthService } from '@services/auth.service';

@Component({
    selector: 'app-login',
    standalone: true,
    imports: [Brand, CommonModule, ReactiveFormsModule, PasswordModule, ButtonModule, InputTextModule, CheckboxModule, RouterLink],
    templateUrl: './login.html',
    styleUrl: './login.css'
})
export class Login {
    private fb = inject(FormBuilder);
    private router = inject(Router);
    private auth = inject(AuthService);

    protected isLoading = false;
    protected errorMessage: string | null = null;
    protected currentYear = new Date().getFullYear();

    protected loginForm: FormGroup = this.fb.group({
        email: ['', [Validators.required, Validators.email]],
        password: ['', [Validators.required]],
        rememberMe: [false]
    });

    onSubmit(): void {
        if (this.loginForm.invalid) {
            this.loginForm.markAllAsTouched();
            return;
        }

        this.isLoading = true;
        this.errorMessage = null;

        const { email, password, rememberMe } = this.loginForm.value;

        this.auth
            .login({ email, password, remember_me: rememberMe })
            .pipe(finalize(() => (this.isLoading = false)))
            .subscribe({
                next: () => {
                    this.router.navigate(['/dashboard']);
                },
                error: (err) => {
                    console.error('Login failed:', err);
                    if (err.status === 401) {
                        this.errorMessage = 'Invalid email or password.';
                    } else {
                        this.errorMessage = 'An unexpected error occurred. Please try again.';
                    }
                }
            });
    }
}
