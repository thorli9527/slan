import { AppComponentOverview } from './overview/app.component.overview';
import { ApiHttpError } from './app-api.service';
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
import type { ClientLoginTarget } from './app-auth-flow';
import { shortCodeFromEmail } from './app.utils';
import { WEB_API } from './api-paths';

export class AppComponentAuth extends AppComponentOverview {
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
    const target = clientLoginTarget();
    if (!target) {
      return false;
    }
    try {
      const auth = readStoredBrowserAuth();
      if (!auth) {
        return false;
      }
      this.authMessage = '正在检查浏览器登录状态...';
      this.notifyStateChanged();
      const renewedAuth = await this.renewBrowserAuth(auth);
      this.authMessage = '正在同步客户端登录...';
      this.notifyStateChanged();
      if (!(await this.syncClientLogin(renewedAuth, target))) {
        clearBrowserAuth();
        this.mode = 'login';
        this.notifyStateChanged();
        return false;
      }
      await this.applyAuth(renewedAuth);
      this.navigateToDefaultHome();
      return true;
    } catch (error) {
      this.authMessage = `浏览器登录态恢复失败：${error instanceof Error ? error.message : String(error)}`;
      clearBrowserAuth();
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
      this.authMessage = `${mode === 'login' ? '登录' : '注册'}失败：${this.authErrorMessage(mode, error)}`;
      this.notifyStateChanged();
    }
  }

  private authErrorMessage(mode: 'login' | 'register', error: unknown): string {
    if (error instanceof ApiHttpError) {
      if (mode === 'login') {
        if (error.status === 404 || error.code === 'NOT_FOUND') {
          return '账号不存在，请先切换到注册。';
        }
        if (error.status === 400 || error.code === 'BAD_REQUEST') {
          return '邮箱或密码错误，请重新输入。';
        }
        if (error.status === 429 || error.code === 'RATE_LIMITED') {
          return '登录尝试过多，请稍后再试。';
        }
      }
      if (mode === 'register') {
        if (error.status === 409 || error.code === 'CONFLICT') {
          return '账号已存在，请切换到登录。';
        }
        if (error.status === 400 || error.code === 'BAD_REQUEST') {
          return '请检查邮箱、名称和密码是否填写正确。';
        }
        if (error.status === 429 || error.code === 'RATE_LIMITED') {
          return '请求过于频繁，请稍后再试。';
        }
      }
      return error.message;
    }
    return error instanceof Error ? error.message : String(error);
  }

  logout(): void {
    this.resetTransientUiState();
    this.currentSessionToken = '';
    this.currentRefreshToken = '';
    clearBrowserAuth();
    this.mode = 'login';
    this.notifyStateChanged();
  }

  private async applyAuth(auth: ApiAuthResponse, persist = true): Promise<void> {
    this.currentUser = auth.user.email;
    this.currentUserId = auth.user.userId;
    this.currentUserShortCode = shortCodeFromEmail(auth.user.email);
    this.currentSessionToken = auth.session.token;
    this.currentRefreshToken = auth.session.refreshToken || '';
    if (persist) {
      persistBrowserAuth(auth);
    }
    this.authMessage = '';
    this.mode = 'home';
    this.notifyStateChanged();
    await this.loadDashboard(auth.user.userId);
    this.applyRouteFromLocation();
    this.notifyStateChanged();
  }

  private async completeDeviceLogin(auth: ApiAuthResponse, target: ClientLoginTarget): Promise<void> {
    await this.api.post(WEB_API.completeDeviceLogin(target.deviceId), {
      accessToken: auth.session.token,
      userId: auth.user.userId,
      email: auth.user.email,
      action: target.authMode,
    });
  }

  private async renewBrowserAuth(auth: ApiAuthResponse): Promise<ApiAuthResponse> {
    const response = await this.api.postAuthorized<{ auth: ApiAuthResponse }>(
      WEB_API.authRenew,
      auth.session.token,
      {
        refreshToken: auth.session.refreshToken || '',
      },
    );
    return response.auth;
  }

  private async prepareDeviceLogin(target: ClientLoginTarget): Promise<void> {
    await this.api.post(WEB_API.prepareDeviceLogin, {
      deviceId: target.deviceId,
      platform: 'desktop',
      osName: 'browser',
      osVersion: navigator.platform || 'browser',
      name: target.deviceId,
      alias: target.deviceId,
      publicKey: 'pk_' + 'a'.repeat(64),
    });
  }

  private async syncClientLogin(auth: ApiAuthResponse, target = clientLoginTarget()): Promise<boolean> {
    if (!target) {
      return true;
    }
    try {
      try {
        await this.completeDeviceLogin(auth, target);
      } catch (error) {
        if (!(error instanceof ApiHttpError) || error.status !== 404) {
          throw error;
        }
        await this.prepareDeviceLogin(target);
        await this.completeDeviceLogin(auth, target);
      }
      this.authMessage = '';
      this.notifyStateChanged();
      return true;
    } catch (error) {
      if (error instanceof ApiHttpError && error.status === 409) {
        clearBrowserAuth();
      }
      this.authMessage = `客户端登录同步失败：${this.clientLoginSyncErrorMessage(error)}`;
      this.notifyStateChanged();
      return false;
    }
  }

  private clientLoginSyncErrorMessage(error: unknown): string {
    if (error instanceof ApiHttpError) {
      if (error.status === 409 || error.code === 'CONFLICT') {
        return '客户端设备身份冲突。请回到当前客户端重新点击登录，或先在控制台删除旧设备记录后重新绑定。';
      }
      if (error.status === 401 || error.code === 'UNAUTHORIZED') {
        return '浏览器登录已过期，请重新登录后再同步客户端。';
      }
      if (error.status === 404 || error.code === 'NOT_FOUND') {
        return '客户端设备注册信息不存在，请回到客户端重新点击登录。';
      }
      if (error.status === 429 || error.code === 'RATE_LIMITED') {
        return '客户端登录请求过于频繁，请稍后再试。';
      }
      return error.message;
    }
    return error instanceof Error ? error.message : String(error);
  }

  private navigateToDefaultHome(): void {
    replaceUrl(this.homePathFromCurrentRoute(), sanitizedHomeParams());
    this.applyRouteFromLocation();
  }

  private homePathFromCurrentRoute(): string {
    const path = window.location.pathname;
    if (path.startsWith('/space/') || path === '/devices' || path === '/user-aliases' || path === '/spaces') {
      return path;
    }
    return '/overview';
  }

  openPasswordDialog(): void {
    this.closeInlinePopovers();
    this.closeDeviceExposureDialogState();
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
      await this.api.patch(WEB_API.userPassword(this.effectiveUserId), {
        actorUserId: this.effectiveUserId,
        oldPassword: this.oldPassword,
        newPassword: this.newPassword,
      });
    } catch {
      if (!this.isDemoMode) {
        this.passwordMessage = '修改密码失败';
        this.notifyStateChanged();
        return;
      }
    }
    this.authPassword = this.newPassword;
    this.closePasswordDialog();
  }
}
