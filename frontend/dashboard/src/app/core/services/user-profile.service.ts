import { Injectable, inject, signal, computed } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { catchError, tap, throwError } from 'rxjs';
import { AuthService } from '@services/auth.service';

export interface UserProfile {
    id: string;
    email: string;
    display_name: string;
    avatar_url: string;
}

@Injectable({ providedIn: 'root' })
export class UserProfileService {
    private http = inject(HttpClient);
    private auth = inject(AuthService);

    readonly profile = signal<UserProfile | null>(null);
    readonly displayName = computed(() => {
        const profile = this.profile();
        if (!profile) return 'User';
        return profile.display_name || profile.email.split('@')[0] || 'User';
    });
    readonly avatarUrl = computed(() => this.profile()?.avatar_url || '');

    loadProfile() {
        return this.http.get<UserProfile>('/api/user/profile').pipe(
            tap((profile) => {
                this.profile.set(profile);
                this.auth.markAuthenticated();
            }),
            catchError((error) => {
                if (error?.status === 401) {
                    this.auth.markUnauthenticated();
                }
                return throwError(() => error);
            })
        );
    }
}
