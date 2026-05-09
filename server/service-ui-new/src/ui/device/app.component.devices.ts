import QRCode from 'qrcode';
import { AppComponentUserAlias } from '../user-alias/app.component.user-alias';
import {
  ApiDevice,
  ApiDNSRecord,
  ApiDNSZone,
  ApiPublicMapping,
  ApiSecurityGroup,
  ApiSecurityRule,
  ApiUserAlias,
  ApiWorkspace,
  ApiWorkspaceDevice,
  DeviceExposureRow,
  DeviceRow,
  DNSRow,
  DNSZoneRow,
  MemberRow,
  NavItem,
  PublicMappingRow,
  RuleSubjectType,
  SecurityGroupRow,
  SecurityRuleRow,
  SecurityRuleTemplate,
  UserAliasRow,
  WorkspaceDeviceInviteRow,
  WorkspacePanel,
  WorkspaceRow,
} from '../app.models';


export abstract class AppComponentDevices extends AppComponentUserAlias {
  async addDevice(): Promise<void> {
    try {
      await this.api.post('/api/devices/register', {
        userId: this.currentUserId || 'user-000001',
        deviceId: this.deviceId,
        name: this.deviceAlias,
        platform: this.devicePlatform,
        osName: this.devicePlatform,
        osVersion: this.deviceOSVersion,
        alias: this.deviceAlias,
      });
      await this.loadDashboard(this.currentUserId);
      return;
    } catch {
      // Keep the page usable when the API service is not running.
    }
    const nextIp = `10.0.0.${this.devices.length + 1}`;
    this.devices = [
      ...this.devices,
      { deviceId: this.deviceId, platform: this.devicePlatform, osVersion: this.deviceOSVersion, alias: this.deviceAlias, ip: nextIp, owner: this.currentUser, status: 'active' },
    ];
  }
  openDeviceAliasDialog(device: DeviceRow): void {
    if (this.showDeviceAliasDialog && this.editingDevice?.deviceId === device.deviceId) {
      this.closeDeviceAliasDialog();
      return;
    }
    this.editingDevice = device;
    this.deviceAliasValue = device.alias;
    this.showDeviceAliasDialog = true;
  }

  closeDeviceAliasDialog(): void {
    this.showDeviceAliasDialog = false;
    this.editingDevice = null;
  }

  async saveDeviceAliasDialog(): Promise<void> {
    if (!this.editingDevice || !this.deviceAliasValue.trim()) {
      return;
    }
    const alias = this.deviceAliasValue.trim();
    const deviceID = this.editingDevice.deviceId;
    try {
      const updated = await this.api.patch<ApiDevice>(`/api/devices/${encodeURIComponent(deviceID)}`, {
        actorUserId: this.currentUserId || this.editingDevice.owner,
        alias,
      });
      this.devices = this.devices.map((item) => item.deviceId === deviceID ? this.mapDevice(updated) : item);
    } catch {
      this.devices = this.devices.map((item) => item.deviceId === deviceID ? { ...item, alias } : item);
    }
    this.closeDeviceAliasDialog();
  }



  async openInviteDialog(): Promise<void> {
    if (!this.canCreateDeviceInvite) {
      this.workspaceInviteCode = '';
      this.inviteQrDataUrl = '';
      this.joinInviteMessage = `当前套餐 ${this.currentPlanName} 最多 ${this.currentDeviceTotalLimit} 台设备，已无剩余设备额度。`;
      this.showInviteDialog = true;
      return;
    }
    try {
      const invite = await this.api.post<WorkspaceDeviceInviteRow>('/api/device-invites', {
        inviterUserId: this.currentUserId || 'user-000001',
        ttlSeconds: 86400,
      });
      this.workspaceInviteCode = invite.inviteCode;
      this.upsertWorkspaceDeviceInvite(invite);
    } catch {
      const randomPart = Math.random().toString(36).slice(2, 8).toUpperCase();
      this.workspaceInviteCode = `JOIN-${randomPart}`;
      this.upsertWorkspaceDeviceInvite({
        inviteId: `device-invite-${this.workspaceInviteCode}`,
        inviterUserId: this.currentUserId || 'user-000001',
        inviteCode: this.workspaceInviteCode,
        status: 'pending',
        createdAt: Math.floor(Date.now() / 1000),
        expiresAt: Math.floor(Date.now() / 1000) + 86400,
      });
    }
    this.inviteQrDataUrl = await QRCode.toDataURL(this.workspaceInviteCode, {
      errorCorrectionLevel: 'M',
      margin: 2,
      scale: 8,
      color: {
        dark: '#111827',
        light: '#ffffff',
      },
    });
    this.showInviteDialog = true;
  }

  closeInviteDialog(): void {
    this.showInviteDialog = false;
  }

  openJoinDialog(device: DeviceRow): void {
    this.deviceId = device.deviceId;
    this.joinInviteCode = '';
    this.joinInviteMessage = '';
    this.showJoinDialog = true;
  }

  closeJoinDialog(): void {
    this.showJoinDialog = false;
    this.joinInviteMessage = '';
  }

  async joinByInviteCode(): Promise<void> {
    if (!this.joinInviteCode.trim() || !this.deviceId) {
      return;
    }
    const invite = this.selectedJoinInvite;
    if (invite && this.effectiveInviteStatus(invite) !== 'pending') {
      this.joinInviteMessage = '该邀请码已失效，不能继续接入。';
      return;
    }
    try {
      const result = await this.api.post<{ invite?: WorkspaceDeviceInviteRow }>('/api/device-invites/accept', {
        inviteCode: this.joinInviteCode,
        deviceId: this.deviceId,
        actorUserId: this.currentUserId || this.devices.find((device) => device.deviceId === this.deviceId)?.owner || 'user-000001',
      });
      if (result.invite) {
        this.upsertWorkspaceDeviceInvite(result.invite);
      }
      await this.loadDashboard(this.currentUserId);
    } catch {
      this.markInviteAccepted(this.joinInviteCode, '', this.deviceId);
    }
    this.showJoinDialog = false;
  }

  openDeviceExposureDialog(device: DeviceRow): void {
    this.selectedExposureDevice = device;
    this.exposureUser = 'bob@vlan.com';
    this.showDeviceExposureDialog = true;
  }

  closeDeviceExposureDialog(): void {
    this.showDeviceExposureDialog = false;
  }

  addDeviceExposure(): void {
    if (!this.selectedExposureDevice || !this.exposureUser.trim()) {
      return;
    }
    this.deviceExposures = [
      ...this.deviceExposures,
      { deviceId: this.selectedExposureDevice.deviceId, user: this.exposureUser.trim(), alias: this.exposureUser.split('@')[0] || '用户', status: 'active' },
    ];
  }

  removeDeviceExposure(exposure: DeviceExposureRow): void {
    this.deviceExposures = this.deviceExposures.filter((item) => item !== exposure);
  }

  async removeDevice(device: DeviceRow): Promise<void> {
    try {
      await this.api.delete(`/api/devices/${encodeURIComponent(device.deviceId)}?actorUserId=${encodeURIComponent(this.currentUserId || device.owner)}`);
      await this.loadDashboard(this.currentUserId);
      return;
    } catch {
      // Local preview fallback.
    }
    this.devices = this.devices.filter((item) => item.deviceId !== device.deviceId);
  }

  removeMember(member: MemberRow): void {
    if (member.role === 'owner') {
      return;
    }
    this.members = this.members.filter((item) => item !== member);
  }

}
