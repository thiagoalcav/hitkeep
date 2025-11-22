import { Component, inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import { HttpClient } from '@angular/common/http';
import { Router } from '@angular/router';
import {
  FormBuilder,
  FormGroup,
  Validators,
  ReactiveFormsModule
} from '@angular/forms';
import { finalize } from 'rxjs/operators';

// PrimeNG Imports
import { PasswordModule } from 'primeng/password';
import { ButtonModule } from 'primeng/button';
import { InputTextModule } from 'primeng/inputtext';

// Corrected path to Core
import { Brand } from '../../core/components/brand/brand';

@Component({
  selector: 'app-login',
  standalone: true,
  imports: [
    Brand,
    CommonModule,
    ReactiveFormsModule,
    PasswordModule,
    ButtonModule,
    InputTextModule
  ],
  templateUrl: './login.html',
  styleUrl: './login.css',
})
export class Login {
  private fb = inject(FormBuilder);
  private http = inject(HttpClient);
  private router = inject(Router);

  protected isLoading = false;
  protected errorMessage: string | null = null;
  protected currentYear = new Date().getFullYear();

  protected loginForm: FormGroup = this.fb.group({
    email: ['', [Validators.required, Validators.email]],
    password: ['', [Validators.required]]
  });

  onSubmit(): void {
    if (this.loginForm.invalid) {
      this.loginForm.markAllAsTouched();
      return;
    }

    this.isLoading = true;
    this.errorMessage = null;

    const { email, password } = this.loginForm.value;

    this.http.post('/api/login', { email, password })
      .pipe(
        finalize(() => this.isLoading = false)
      )
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
