import { Injectable, computed, inject, signal } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { Observable, finalize, tap } from 'rxjs';

export type AuthStatus = 'unknown' | 'authenticated' | 'unauthenticated';

@Injectable({ providedIn: 'root' })
export class AuthService {
    private http = inject(HttpClient);
    readonly status = signal<AuthStatus>('unknown');
    readonly isAuthenticated = computed(() => this.status() === 'authenticated');

    login(credentials: { email: string; password: string; remember_me?: boolean }): Observable<void> {
        return this.http.post<void>('/api/login', credentials).pipe(tap(() => this.status.set('authenticated')));
    }

    logout(): Observable<void> {
        return this.http.post<void>('/api/logout', {}).pipe(finalize(() => this.status.set('unauthenticated')));
    }

    requestPasswordReset(email: string): Observable<void> {
        return this.http.post<void>('/api/auth/forgot-password', { email });
    }

    resetPassword(token: string, password: string): Observable<void> {
        return this.http.post<void>('/api/auth/reset-password', { token, password });
    }

    changePassword(current: string, newPass: string): Observable<void> {
        return this.http.post<void>('/api/user/password', {
            current_password: current,
            new_password: newPass
        });
    }

    markAuthenticated() {
        this.status.set('authenticated');
    }

    markUnauthenticated() {
        this.status.set('unauthenticated');
    }
}
