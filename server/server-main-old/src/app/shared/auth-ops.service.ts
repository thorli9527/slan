import { Injectable, inject, signal } from '@angular/core';
import { OpsLoginResponse } from './models';
import { OpsApiService } from './ops-api.service';

@Injectable({ providedIn: 'root' })
export class AuthOpsService {
  private readonly api = inject(OpsApiService);
  readonly token = signal(localStorage.getItem('slan.opsToken') || '');
  readonly adminId = signal(localStorage.getItem('slan.opsAdminId') || '');
  readonly adminName = signal(localStorage.getItem('slan.opsAdminName') || '');

  login(loginName: string, password: string): Promise<OpsLoginResponse> {
    return this.api.request<OpsLoginResponse>('/login', '', {
      method: 'POST',
      body: JSON.stringify({ loginName, password }),
    }, false);
  }

  logout(token: string): Promise<unknown> {
    return this.api.request('/logout', token, { method: 'POST' });
  }

  changeOwnPassword(token: string, password: string): Promise<unknown> {
    return this.api.request('/me/password', token, {
      method: 'PUT',
      body: JSON.stringify({ password }),
    });
  }

  setSession(token: string, adminId = '', adminName = ''): void {
    localStorage.setItem('slan.opsToken', token);
    this.token.set(token);
    if (adminId) {
      localStorage.setItem('slan.opsAdminId', adminId);
      this.adminId.set(adminId);
    } else {
      localStorage.removeItem('slan.opsAdminId');
      this.adminId.set('');
    }
    if (adminName) {
      localStorage.setItem('slan.opsAdminName', adminName);
      this.adminName.set(adminName);
    } else {
      localStorage.removeItem('slan.opsAdminName');
      this.adminName.set('');
    }
  }

  clearSession(): void {
    localStorage.removeItem('slan.opsToken');
    localStorage.removeItem('slan.opsAdminId');
    localStorage.removeItem('slan.opsAdminName');
    this.token.set('');
    this.adminId.set('');
    this.adminName.set('');
  }
}
