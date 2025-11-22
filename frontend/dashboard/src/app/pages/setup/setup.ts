import { Component, inject } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { Router } from '@angular/router';
import {
  FormBuilder,
  FormGroup,
  Validators,
  ReactiveFormsModule
} from '@angular/forms';
import { finalize } from 'rxjs/operators';
import { CommonModule } from '@angular/common';

// PrimeNG Imports
import { PasswordModule } from 'primeng/password';
import { ButtonModule } from 'primeng/button';
import { InputTextModule } from 'primeng/inputtext';

// Corrected path to Core
import { Brand } from '../../core/components/brand/brand';

@Component({
  selector: 'app-setup',
  standalone: true,
  imports: [
    Brand,
    CommonModule,
    ReactiveFormsModule,
    PasswordModule,
    ButtonModule,
    InputTextModule
  ],
  templateUrl: './setup.html',
  styleUrl: './setup.css',
})
export class Setup {

  private fb = inject(FormBuilder);
  private http = inject(HttpClient);
  private router = inject(Router);

  protected isLoading = false;
  protected errorMessage: string | null = null;

  protected setupForm: FormGroup = this.fb.group({
    email: ['', [Validators.required, Validators.email]],
    password: ['', [Validators.required, Validators.minLength(8)]]
  });

  onSubmit(): void {
    if (this.setupForm.invalid) {
      this.setupForm.markAllAsTouched();
      return;
    }

    this.isLoading = true;
    this.errorMessage = null;

    const { email, password } = this.setupForm.value;

    this.http.post('/api/initial-user', { email, password })
      .pipe(
        finalize(() => this.isLoading = false)
      )
      .subscribe({
        next: () => {
          this.router.navigate(['/login']);
        },
        error: (err) => {
          console.error('Failed to create initial user:', err);
          this.errorMessage = err.error?.message || 'An unexpected error occurred. Please try again.';
        }
      });
  }
}
