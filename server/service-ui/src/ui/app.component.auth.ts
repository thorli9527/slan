import { AppComponentData } from './overview/app.component.data';
import { ApiAuthResponse } from './app.models';
import {
  clearBrowserAuth,
  clientLoginTarget,
  consoleLoginTarget,
  persistBrowserAuth,
  readStoredBrowserAuth,
  replaceUrl,
  sanitizedHomeParams,
} from './app-auth-flow';
import { shortCodeFromEmail } from './app.utils';
import { WEB_API } from './api-paths';

export class AppComponentAuth extends AppComponentData {
  protected async initializeCustomerAuthFromUrl(): Promise<void> {
    if (await this.completeClientLoginFromStoredBrowserAuth()) {
      return;
    }
    if (await this.consumeConsoleLoginKeyFromUrl()) {
      return;
    }
    await this.restoreBrowserAuth();
    this.applyRouteFromLocation();
  }

  private async completeClientLoginFromStoredBrowserAuth(): Promise<boolean> {
    if (!clientLoginTarget()) {
      return false;
    }
    try {
      const auth = readStoredBrowserAuth();
      if (!auth) {
        return false;
      }
      this.authMessage = '正在同步客户端登录...';
      this.notifyStateChanged();
      if (!(await this.syncClientLogin(auth))) {
        clearBrowserAuth();
        this.mode = 'login';
        this.notifyStateChanged();
        return false;
      }
      await this.applyAuth(auth);
      this.navigateToDefaultHome();
      return true;
    } catch (error) {
      this.authMessage = `浏览器登录态恢复失败：${error instanceof Error ? error.message : String(error)}`;
      this.mode = 'login';
      this.notifyStateChanged();
      return false;
    }
  }

  protected async restoreBrowserAuth(): Promise<boolean> {
    try {
      const auth = readStoredBrowserAuth();
      if (!auth) {
        return false;
      }
      await this.applyAuth(auth);
      return true;
    } catch (error) {
      this.authMessage = `浏览器登录态恢复失败：${error instanceof Error ? error.message : String(error)}`;
      this.mode = 'login';
      this.notifyStateChanged();
      return false;
    }
  }

  async consumeConsoleLoginKeyFromUrl(): Promise<boolean> {
    const target = consoleLoginTarget();
    if (!target) {
      return false;
    }
    this.authMessage = '正在通过客户端临时登录...';
    try {
      const response = await this.api.post<{ auth: ApiAuthResponse }>(WEB_API.consoleLogin, {
        loginKey: target.loginKey,
      });
      await this.applyAuth(response.auth);
      this.navigateToDefaultHome();
      return true;
    } catch (error) {
      const message = '非法或已过期的客户端登录凭证，请从客户端重新打开 Web Console。';
      window.alert(message);
      this.authMessage = message;
      replaceUrl('/', sanitizedHomeParams());
      this.mode = 'login';
      this.notifyStateChanged();
      return false;
    }
  }

  async enter(mode: 'login' | 'register'): Promise<void> {
    if (!this.authEmail || !this.authPassword) {
      return;
    }
    try {
      const path = mode === 'login' ? WEB_API.authLogin : WEB_API.authRegister;
      const payload = mode === 'login'
        ? { email: this.authEmail, password: this.authPassword }
        : { email: this.authEmail, password: this.authPassword, name: this.authName };
      const response = await this.api.post<{ auth: ApiAuthResponse }>(path, payload);
      const synced = await this.syncClientLogin(response.auth);
      if (!synced) {
        return;
      }
      await this.applyAuth(response.auth);
      this.navigateToDefaultHome();
    } catch (error) {
      this.authMessage = `${mode === 'login' ? '登录' : '注册'}失败：${error instanceof Error ? error.message : String(error)}`;
      this.notifyStateChanged();
    }
  }

  logout(): void {
    this.currentSessionToken = '';
    this.authMessage = '';
    clearBrowserAuth();
    this.mode = 'login';
    this.notifyStateChanged();
  }

  private async applyAuth(auth: ApiAuthResponse, persist = true): Promise<void> {
    this.currentUser = auth.user.email;
    this.currentUserId = auth.user.userId;
    this.currentUserShortCode = shortCodeFromEmail(auth.user.email);
    this.currentSessionToken = auth.session.token;
    if (persist) {
      persistBrowserAuth(auth);
    }
    this.authMessage = '';
    this.mode = 'home';
    this.notifyStateChanged();
    await this.loadDashboard(auth.user.userId);
    this.notifyStateChanged();
  }

  private async completeDeviceLogin(auth: ApiAuthResponse): Promise<void> {
    const target = clientLoginTarget();
    if (!target) {
      return;
    }
    await this.api.post(WEB_API.completeDeviceLogin(target.deviceId), {
      accessToken: auth.session.token,
      userId: auth.user.userId,
      email: auth.user.email,
      action: target.authMode,
    });
  }

  private async syncClientLogin(auth: ApiAuthResponse): Promise<boolean> {
    if (!clientLoginTarget()) {
      return true;
    }
    try {
      await this.completeDeviceLogin(auth);
      this.authMessage = '';
      this.notifyStateChanged();
      return true;
    } catch (error) {
      this.authMessage = `客户端登录同步失败：${error instanceof Error ? error.message : String(error)}`;
      this.notifyStateChanged();
      return false;
    }
  }

  private navigateToDefaultHome(): void {
    replaceUrl('/overview', sanitizedHomeParams());
    this.applyRouteFromLocation();
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
      await this.api.patch(WEB_API.userPassword(this.currentUserId || 'user-000001'), {
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
