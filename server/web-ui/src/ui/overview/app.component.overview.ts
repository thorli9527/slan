import { AppComponentState } from '../app.component.state';
import { DeviceBootstrapKeyRow, WorkspaceDeviceInviteRow } from '../app.models';

export abstract class AppComponentOverview extends AppComponentState {
  enabledDeviceCount(): number {
    return this.currentUserDevices.filter((device) => device.status === 'active').length;
  }

  inviteStatusLabel(status: string): string {
    if (status === 'revoked') {
      return '已作废';
    }
    if (status === 'accepted') {
      return '已接入';
    }
    if (status === 'expired') {
      return '已过期';
    }
    return '待接入';
  }

  override effectiveInviteStatus(invite: WorkspaceDeviceInviteRow): string {
    if (invite.status === 'pending' && invite.expiresAt > 0 && invite.expiresAt < Math.floor(Date.now() / 1000)) {
      return 'expired';
    }
    return invite.status;
  }

  copyInviteCode(invite: WorkspaceDeviceInviteRow): void {
    void navigator.clipboard?.writeText(invite.inviteCode);
  }

  formatTime(value?: number): string {
    if (!value) {
      return '-';
    }
    return new Date(value * 1000).toLocaleString('zh-CN', { hour12: false });
  }

  effectiveBootstrapStatus(item: DeviceBootstrapKeyRow): string {
    if (item.status === 'revoked') {
      return 'revoked';
    }
    if (item.usedAt) {
      return 'used';
    }
    if (item.expiresAt > 0 && item.expiresAt < Math.floor(Date.now() / 1000)) {
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
      default:
        return '生效中';
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
}
