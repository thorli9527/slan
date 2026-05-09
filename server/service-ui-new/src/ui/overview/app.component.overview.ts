import { AppComponentState } from '../app.component.state';
import { WorkspaceDeviceInviteRow } from '../app.models';

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
}
