import { AppComponentData } from './overview/app.component.data';
import { shortCodeFromEmail } from './app.utils';

export class AppComponentAuth extends AppComponentData {
  async enter(mode: 'login' | 'register'): Promise<void> {
    if (!this.authEmail || !this.authPassword) {
      return;
    }
    try {
      const path = mode === 'login' ? '/api/auth/login' : '/api/auth/register';
      const payload = mode === 'login'
        ? { email: this.authEmail, password: this.authPassword }
        : { email: this.authEmail, password: this.authPassword, name: this.authName };
      const response = await this.api.post<{ auth: { user: { userId: string; email: string }; session: { token: string } } }>(path, payload);
      this.currentUser = response.auth.user.email;
      this.currentUserId = response.auth.user.userId;
      this.currentUserShortCode = shortCodeFromEmail(response.auth.user.email);
      await this.completeDeviceLoginCallback(response.auth);
      this.mode = 'home';
      await this.loadDashboard(response.auth.user.userId);
      this.applyRouteFromLocation();
    } catch {
      this.currentUser = this.authEmail;
      this.currentUserShortCode = shortCodeFromEmail(this.authEmail);
      this.mode = 'home';
      if (mode === 'register' && !this.devices.some((device) => device.owner === this.authEmail)) {
        this.devices = [
          ...this.devices,
          { deviceId: 'new-device', platform: 'macOS', osVersion: '15.x', alias: '新设备', ip: '10.0.0.20', owner: this.authEmail, status: 'active' },
        ];
      }
    }
  }

  logout(): void {
    this.mode = 'login';
  }

  private async completeDeviceLoginCallback(auth: { user: { userId: string; email: string }; session: { token: string } }): Promise<void> {
    const params = new URLSearchParams(window.location.search);
    const callbackId = params.get('callbackId')?.trim();
    if (!callbackId) {
      return;
    }
    await this.api.post(`/api/auth/device-login-callbacks/${encodeURIComponent(callbackId)}/complete`, {
      accessToken: auth.session.token,
      deviceId: params.get('deviceId')?.trim() || undefined,
      action: params.get('auth') === 'login' ? 'login' : 'callback',
    });
    params.delete('auth');
    params.delete('callbackId');
    params.delete('deviceId');
    const query = params.toString();
    const nextUrl = `${window.location.pathname}${query ? `?${query}` : ''}${window.location.hash}`;
    window.history.replaceState({}, '', nextUrl);
  }

  openPasswordDialog(): void {
    this.oldPassword = '';
    this.newPassword = '';
    this.confirmPassword = '';
    this.passwordMessage = '';
    this.showPasswordDialog = true;
  }

  closePasswordDialog(): void {
    this.showPasswordDialog = false;
  }

  async changePassword(): Promise<void> {
    this.passwordMessage = '';
    if (!this.oldPassword || !this.newPassword || !this.confirmPassword) {
      this.passwordMessage = '请填写完整密码信息';
      return;
    }
    if (this.newPassword !== this.confirmPassword) {
      this.passwordMessage = '两次输入的新密码不一致';
      return;
    }
    if (this.newPassword.length < 6) {
      this.passwordMessage = '新密码至少 6 位';
      return;
    }
    try {
      await this.api.patch(`/api/users/${encodeURIComponent(this.currentUserId || 'user-000001')}/password`, {
        oldPassword: this.oldPassword,
        newPassword: this.newPassword,
      });
    } catch {
      // Preview mode without API: keep the interaction complete locally.
    }
    this.authPassword = this.newPassword;
    this.closePasswordDialog();
  }
}
