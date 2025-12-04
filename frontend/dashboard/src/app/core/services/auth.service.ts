import { Injectable, inject } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { Observable } from 'rxjs';

@Injectable({ providedIn: 'root' })
export class AuthService {
  private http = inject(HttpClient);

  login(credentials: { email: string; password: string; remember_me?: boolean }): Observable<any> {
    return this.http.post('/api/login', credentials);
  }

  logout(): Observable<any> {
    return this.http.post('/api/logout', {});
  }

  requestPasswordReset(email: string): Observable<any> {
    return this.http.post('/api/auth/forgot-password', { email });
  }

  resetPassword(token: string, password: string): Observable<any> {
    return this.http.post('/api/auth/reset-password', { token, password });
  }

  changePassword(current: string, newPass: string): Observable<any> {
    return this.http.post('/api/user/password', {
      current_password: current,
      new_password: newPass
    });
  }
}
