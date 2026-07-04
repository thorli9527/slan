import { AppComponentSecurity } from '../network/app.component.security';
import { ApiManagedDeviceSession, ApiManagedUserSession, DeviceBootstrapKeyRow, DeviceRow } from '../app.models';
import { WEB_API } from '../api-paths';

export abstract class AppComponentOverview extends AppComponentSecurity {
  setActive(id: string): void {
    this.active = id;
    if (id === 'overview') {
      this.workspaceRouteMode = 'list';
      history.pushState({}, '', '/overview');
      return;
    }
    if (id === 'devices') {
      this.workspaceRouteMode = 'list';
      this.devicePanel = 'list';
      history.pushState({}, '', '/devices');
      return;
    }
    if (id === 'deviceGroups') {
      this.active = 'devices';
      this.workspaceRouteMode = 'list';
      this.devicePanel = 'groups';
      history.pushState({}, '', '/devices/groups');
      return;
    }
    if (id === 'userAliases') {
      this.workspaceRouteMode = 'list';
      history.pushState({}, '', '/user-aliases');
      return;
    }
    if (id === 'workspaces') {
      this.workspaceRouteMode = 'list';
      history.pushState({}, '', '/spaces');
    }
  }

  override isWorkspaceCodeDuplicated(code: string, exceptWorkspaceId = ''): boolean {
    return this.workspaces.some((workspace) => workspace.workspaceId !== exceptWorkspaceId && workspace.code === code);
  }

  formatTime(value?: number): string {
    if (!value) {
      return '-';
    }
    return new Date(value * 1000).toLocaleString('zh-CN', { hour12: false });
  }

  effectiveBootstrapStatus(item: DeviceBootstrapKeyRow): string {
    if (item.revokedAt) {
      return 'revoked';
    }
    if (item.usedAt) {
      return 'used';
    }
    if (item.expiresAt && item.expiresAt < Math.floor(Date.now() / 1000)) {
      return 'expired';
    }
    return item.status || 'active';
  }

  bootstrapStatusLabel(item: DeviceBootstrapKeyRow): string {
    switch (this.effectiveBootstrapStatus(item)) {
      case 'revoked':
        return '已撤销';
      case 'used':
        return '已使用';
      case 'expired':
        return '已过期';
      case 'active':
      default:
        return '有效';
    }
  }

  bootstrapExpiryText(item: DeviceBootstrapKeyRow): string {
    if (!item.expiresAt) {
      return '-';
    }
    const remain = item.expiresAt - Math.floor(Date.now() / 1000);
    if (remain <= 0) {
      return `${this.formatTime(item.expiresAt)} / 已过期`;
    }
    if (remain < 3600) {
      return `${this.formatTime(item.expiresAt)} / ${Math.ceil(remain / 60)} 分钟后过期`;
    }
    if (remain < 86400) {
      return `${this.formatTime(item.expiresAt)} / ${Math.ceil(remain / 3600)} 小时后过期`;
    }
    return `${this.formatTime(item.expiresAt)} / ${Math.ceil(remain / 86400)} 天后过期`;
  }

  async revokeUserManagedSession(item: ApiManagedUserSession): Promise<void> {
    const previous = this.userSessions;
    const revokedAt = Math.floor(Date.now() / 1000);
    this.userSessions = previous.map((current) => current.sessionId === item.sessionId ? { ...current, status: 'revoked', revokedAt } : current);
    try {
      const updated = await this.api.post<ApiManagedUserSession>(
        WEB_API.userSessionRevoke(this.effectiveUserId, item.sessionId, this.effectiveUserId),
        {},
      );
      this.userSessions = this.userSessions.map((current) => current.sessionId === item.sessionId ? updated : current);
    } catch {
      if (!this.isDemoMode) {
        this.userSessions = previous;
        this.authMessage = '撤销用户 token 失败';
        this.notifyStateChanged();
        return;
      }
    }
    this.notifyStateChanged();
  }

  deviceManagedSessions(device: DeviceRow): ApiManagedDeviceSession[] {
    return this.deviceSessionsByDeviceId[device.deviceId] ?? [];
  }

  async revokeDeviceManagedSession(device: DeviceRow, item: ApiManagedDeviceSession): Promise<void> {
    const previous = this.deviceSessionsByDeviceId;
    const revokedAt = Math.floor(Date.now() / 1000);
    this.deviceSessionsByDeviceId = {
      ...previous,
      [device.deviceId]: (previous[device.deviceId] ?? []).map((current) =>
        current.sessionId === item.sessionId ? { ...current, status: 'revoked', revokedAt } : current,
      ),
    };
    try {
      const updated = await this.api.post<ApiManagedDeviceSession>(
        WEB_API.deviceSessionRevoke(device.deviceId, item.sessionId, this.effectiveUserId),
        {},
      );
      this.deviceSessionsByDeviceId = {
        ...this.deviceSessionsByDeviceId,
        [device.deviceId]: this.deviceManagedSessions(device).map((current) => current.sessionId === item.sessionId ? updated : current),
      };
    } catch {
      if (!this.isDemoMode) {
        this.deviceSessionsByDeviceId = previous;
        this.deviceListMessage = '撤销设备 token 失败';
        this.notifyStateChanged();
        return;
      }
    }
    this.notifyStateChanged();
  }
}
