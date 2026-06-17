import QRCode from 'qrcode';
import { AppComponentUserAlias } from '../user-alias/app.component.user-alias';
import {
  ApiDevice,
  ApiDeviceBootstrapKey,
  ApiDNSRecord,
  ApiDNSZone,
  ApiPublicMapping,
  ApiSecurityGroup,
  ApiSecurityRule,
  ApiUserAlias,
  ApiWorkspace,
  ApiWorkspaceDevice,
  DeviceExposureRow,
  DeviceGroupRow,
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
import { WEB_API } from '../api-paths';
import { DEFAULT_USER_ID } from '../app.seed-data';
import { compactUuid } from '../app.utils';
export abstract class AppComponentDevices extends AppComponentUserAlias {
  setDevicePanel(panel: 'list' | 'groups'): void {
    this.devicePanel = panel;
  }

  async addDevice(): Promise<void> {
    try {
      await this.api.post(WEB_API.devicesRegister, {
        userId: this.currentUserId || DEFAULT_USER_ID,
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

  async toggleDeviceGroup(device: DeviceRow, groupId: string): Promise<void> {
    const current = this.deviceGroupIdsByDevice[device.deviceId] ?? [];
    const nextGroupIds = current.includes(groupId) ? current.filter((item) => item !== groupId) : [...current, groupId];
    const next = { ...this.deviceGroupIdsByDevice };
    if (nextGroupIds.length) {
      next[device.deviceId] = nextGroupIds;
    } else {
      delete next[device.deviceId];
    }
    this.deviceGroupIdsByDevice = next;
    try {
      await this.api.put(WEB_API.deviceGroupsForDevice(this.currentUserId || DEFAULT_USER_ID, device.deviceId), { groupIds: nextGroupIds });
      await this.loadDeviceGroups(this.currentUserId || DEFAULT_USER_ID);
    } catch {
      // Preview mode keeps the local assignment.
    }
  }

  openDeviceGroupBindingDialog(group: DeviceGroupRow): void {
    this.bindingDeviceGroup = group;
    this.deviceGroupBindingIds = this.currentUserDevices
      .filter((device) => this.deviceInGroup(device, group.groupId))
      .map((device) => device.deviceId);
    this.deviceGroupBindingMessage = '';
    this.showDeviceGroupBindingDialog = true;
    this.notifyStateChanged();
  }

  closeDeviceGroupBindingDialog(): void {
    this.showDeviceGroupBindingDialog = false;
    this.bindingDeviceGroup = null;
    this.deviceGroupBindingIds = [];
    this.deviceGroupBindingMessage = '';
    this.notifyStateChanged();
  }

  toggleBindingDevice(device: DeviceRow): void {
    const selected = new Set(this.deviceGroupBindingIds);
    if (selected.has(device.deviceId)) {
      selected.delete(device.deviceId);
    } else {
      selected.add(device.deviceId);
    }
    this.deviceGroupBindingIds = [...selected];
  }

  async saveDeviceGroupBindingDialog(): Promise<void> {
    const group = this.bindingDeviceGroup;
    if (!group) {
      return;
    }
    const selected = new Set(this.deviceGroupBindingIds);
    const previous = this.deviceGroupIdsByDevice;
    const next: Record<string, string[]> = { ...previous };
    for (const device of this.currentUserDevices) {
      const current = previous[device.deviceId] ?? [];
      const nextGroupIds = selected.has(device.deviceId)
        ? Array.from(new Set([...current, group.groupId]))
        : current.filter((groupId) => groupId !== group.groupId);
      if (nextGroupIds.length) {
        next[device.deviceId] = nextGroupIds;
      } else {
        delete next[device.deviceId];
      }
    }
    this.deviceGroupIdsByDevice = next;
    try {
      await Promise.all(this.currentUserDevices.map((device) =>
        this.api.put(WEB_API.deviceGroupsForDevice(this.currentUserId || DEFAULT_USER_ID, device.deviceId), { groupIds: next[device.deviceId] ?? [] }),
      ));
      await this.loadDeviceGroups(this.currentUserId || DEFAULT_USER_ID);
      this.closeDeviceGroupBindingDialog();
    } catch {
      if (!this.currentUserId) {
        this.closeDeviceGroupBindingDialog();
        return;
      }
      this.deviceGroupIdsByDevice = previous;
      this.deviceGroupBindingMessage = '保存失败，请重试';
      this.notifyStateChanged();
    }
  }

  openDeviceGroupDialog(group?: DeviceGroupRow): void {
    this.editingDeviceGroup = group?.groupId ? group : null;
    this.deviceGroupDialogMode = group?.groupId ? 'edit' : 'create';
    this.deviceGroupName = group?.name ?? '开发部';
    this.deviceGroupDescription = group?.description ?? '';
    this.deviceGroupDialogMessage = '';
    this.showDeviceGroupDialog = true;
  }

  closeDeviceGroupDialog(): void {
    this.showDeviceGroupDialog = false;
    this.editingDeviceGroup = null;
    this.deviceGroupDialogMessage = '';
    this.notifyStateChanged();
  }

  async saveDeviceGroupDialog(): Promise<void> {
    const name = this.deviceGroupName.trim();
    if (!name) {
      this.deviceGroupDialogMessage = '请输入分组名称';
      return;
    }
    const duplicate = this.deviceGroups.some((group) =>
      group.name.trim() === name && group.groupId !== this.editingDeviceGroup?.groupId
    );
    if (duplicate) {
      this.deviceGroupDialogMessage = '分组名称不能重复';
      return;
    }
    const description = '';
    if (this.editingDeviceGroup) {
      const groupId = this.editingDeviceGroup.groupId;
      const previousGroups = this.deviceGroups;
      this.deviceGroups = previousGroups.map((group) => group.groupId === groupId ? { ...group, name, description } : group);
      try {
        const group = await this.api.patch<DeviceGroupRow>(WEB_API.deviceGroup(this.currentUserId || DEFAULT_USER_ID, groupId), { name });
        this.deviceGroups = this.deviceGroups.map((item) => item.groupId === group.groupId ? { ...item, ...group, description: '' } : item);
        this.closeDeviceGroupDialog();
      } catch {
        if (!this.currentUserId) {
          this.closeDeviceGroupDialog();
          return;
        }
        this.deviceGroups = previousGroups;
        this.deviceGroupDialogMessage = '保存失败，请重试';
        this.notifyStateChanged();
      }
    } else {
      const localGroup = { groupId: compactUuid(), name, description, createdAt: Math.floor(Date.now() / 1000) };
      this.deviceGroups = [...this.deviceGroups, localGroup];
      try {
        const group = await this.api.post<DeviceGroupRow>(WEB_API.deviceGroups(this.currentUserId || DEFAULT_USER_ID), { name });
        this.deviceGroups = [...this.deviceGroups.filter((item) => item.groupId !== localGroup.groupId), { ...group, description: '' }];
        this.closeDeviceGroupDialog();
      } catch {
        if (!this.currentUserId) {
          this.closeDeviceGroupDialog();
          return;
        }
        this.deviceGroups = this.deviceGroups.filter((item) => item.groupId !== localGroup.groupId);
        this.deviceGroupDialogMessage = '保存失败，请重试';
        this.notifyStateChanged();
      }
    }
  }

  async removeDeviceGroup(group: DeviceGroupRow): Promise<void> {
    this.deviceGroups = this.deviceGroups.filter((item) => item.groupId !== group.groupId);
    this.deviceGroupIdsByDevice = Object.fromEntries(
      Object.entries(this.deviceGroupIdsByDevice)
        .map(([deviceId, groupIds]) => [deviceId, groupIds.filter((groupId) => groupId !== group.groupId)] as const)
        .filter(([, groupIds]) => groupIds.length > 0),
    );
    try {
      await this.api.delete(WEB_API.deviceGroup(this.currentUserId || DEFAULT_USER_ID, group.groupId));
    } catch {
      // Preview mode keeps the local delete.
    }
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
      const updated = await this.api.patch<ApiDevice>(WEB_API.device(deviceID), {
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
      this.notifyStateChanged();
      return;
    }
    try {
      const invite = await this.api.post<WorkspaceDeviceInviteRow>(WEB_API.deviceInvites(), {
        inviterUserId: this.currentUserId || DEFAULT_USER_ID,
        ttlSeconds: 86400,
      });
      this.workspaceInviteCode = invite.inviteCode;
      this.upsertWorkspaceDeviceInvite(invite);
    } catch {
      const randomPart = Math.random().toString(36).slice(2, 8).toUpperCase();
      this.workspaceInviteCode = `JOIN-${randomPart}`;
      this.upsertWorkspaceDeviceInvite({
        inviteId: compactUuid(),
        inviterUserId: this.currentUserId || DEFAULT_USER_ID,
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
    this.notifyStateChanged();
  }

  async openBootstrapDialog(): Promise<void> {
    const network = this.workspaces[0];
    if (!network) {
      this.bootstrapMessage = '请先创建网络。';
      this.showBootstrapDialog = true;
      this.notifyStateChanged();
      return;
    }
    try {
      const key = await this.api.post<ApiDeviceBootstrapKey>(WEB_API.deviceBootstrapKeys(), {
        userId: this.currentUserId || DEFAULT_USER_ID,
        networkId: network.networkId,
        ttlSeconds: 1800,
      });
      this.bootstrapSessionKey = key.key || '';
    } catch {
      this.bootstrapSessionKey = `sk_${Array.from({ length: 64 }, () => Math.floor(Math.random() * 16).toString(16)).join('')}`;
    }
    const server = window.location.origin.replace(/^https?:\/\/web\./, 'http://api.');
    this.bootstrapInstallCommand = `curl -fsSL ${server}/downloads/clients/install.sh | sudo bash -s -- --server ${server} --session-key ${this.bootstrapSessionKey}`;
    this.bootstrapQrDataUrl = await QRCode.toDataURL(this.bootstrapInstallCommand, {
      errorCorrectionLevel: 'M',
      margin: 2,
      scale: 6,
      color: {
        dark: '#111827',
        light: '#ffffff',
      },
    });
    this.bootstrapMessage = '';
    this.showBootstrapDialog = true;
    this.notifyStateChanged();
  }

  closeBootstrapDialog(): void {
    this.showBootstrapDialog = false;
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
      const result = await this.api.post<{ invite?: WorkspaceDeviceInviteRow }>(WEB_API.deviceInviteAccept, {
        inviteCode: this.joinInviteCode,
        deviceId: this.deviceId,
        actorUserId: this.currentUserId || this.devices.find((device) => device.deviceId === this.deviceId)?.owner || DEFAULT_USER_ID,
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
    this.exposureUser = 'bob@staticlss.com';
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
      await this.api.delete(WEB_API.device(device.deviceId, this.currentUserId || device.owner));
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
