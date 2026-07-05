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
import { compactUuid } from '../app.utils';
export abstract class AppComponentDevices extends AppComponentUserAlias {
  setDevicePanel(panel: 'list' | 'groups'): void {
    this.devicePanel = panel;
    this.deviceListMessage = '';
  }

  async addDevice(): Promise<void> {
    this.deviceListMessage = '';
    try {
      await this.api.post(WEB_API.devicesRegister, {
        userId: this.effectiveUserId,
        actorUserId: this.effectiveUserId,
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
      if (!this.isDemoMode) {
        this.deviceListMessage = '新增设备失败';
        this.notifyStateChanged();
        return;
      }
    }
    const nextIp = `10.0.0.${this.devices.length + 1}`;
    this.devices = [
      ...this.devices,
      { deviceId: this.deviceId, platform: this.devicePlatform, osVersion: this.deviceOSVersion, alias: this.deviceAlias, ip: nextIp, owner: this.currentUser, status: 'active' },
    ];
    this.notifyStateChanged();
  }

  async toggleDeviceGroup(device: DeviceRow, groupId: string): Promise<void> {
    this.deviceListMessage = '';
    const current = this.deviceGroupIdsByDevice[device.deviceId] ?? [];
    const nextGroupIds = current.includes(groupId) ? current.filter((item) => item !== groupId) : [...current, groupId];
    const previous = this.deviceGroupIdsByDevice;
    const next = { ...this.deviceGroupIdsByDevice };
    if (nextGroupIds.length) {
      next[device.deviceId] = nextGroupIds;
    } else {
      delete next[device.deviceId];
    }
    this.deviceGroupIdsByDevice = next;
    try {
      await this.api.put(WEB_API.deviceGroupsForDevice(this.effectiveUserId, device.deviceId, this.effectiveUserId), {
        actorUserId: this.effectiveUserId,
        groupIds: nextGroupIds,
      });
      await this.loadDeviceGroups(this.effectiveUserId);
    } catch {
      if (this.isDemoMode) {
        return;
      }
      this.deviceGroupIdsByDevice = previous;
      this.deviceListMessage = '切换设备分组失败';
      this.notifyStateChanged();
    }
  }

  openDeviceGroupBindingDialog(group: DeviceGroupRow): void {
    this.closeInlinePopovers();
    this.bindingDeviceGroup = group;
    this.deviceGroupBindingIds = this.currentUserDevices
      .filter((device) => this.deviceInGroup(device, group.groupId))
      .map((device) => device.deviceId);
    this.bindDeviceQuery = '';
    this.deviceGroupBindingMessage = '';
    this.showDeviceGroupBindingDialog = true;
    this.notifyStateChanged();
  }

  closeDeviceGroupBindingDialog(): void {
    this.showDeviceGroupBindingDialog = false;
    this.bindingDeviceGroup = null;
    this.deviceGroupBindingIds = [];
    this.bindDeviceQuery = '';
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
        this.api.put(WEB_API.deviceGroupsForDevice(this.effectiveUserId, device.deviceId, this.effectiveUserId), {
          actorUserId: this.effectiveUserId,
          groupIds: next[device.deviceId] ?? [],
        }),
      ));
      await this.loadDeviceGroups(this.effectiveUserId);
      this.closeDeviceGroupBindingDialog();
    } catch {
      if (this.isDemoMode) {
        this.closeDeviceGroupBindingDialog();
        return;
      }
      this.deviceGroupIdsByDevice = previous;
      this.deviceGroupBindingMessage = '保存失败，请重试';
      this.notifyStateChanged();
    }
  }

  openDeviceGroupPickerDialog(device: DeviceRow): void {
    this.closeInlinePopovers();
    this.bindingGroupDevice = device;
    this.selectedDeviceGroupIds = [...(this.deviceGroupIdsByDevice[device.deviceId] ?? [])];
    this.deviceGroupBindingMessage = '';
    this.showDeviceGroupPickerDialog = true;
    this.notifyStateChanged();
  }

  closeDeviceGroupPickerDialog(): void {
    this.showDeviceGroupPickerDialog = false;
    this.bindingGroupDevice = null;
    this.selectedDeviceGroupIds = [];
    this.deviceGroupBindingMessage = '';
    this.notifyStateChanged();
  }

  togglePickerDeviceGroup(group: DeviceGroupRow): void {
    const selected = new Set(this.selectedDeviceGroupIds);
    if (selected.has(group.groupId)) {
      selected.delete(group.groupId);
    } else {
      selected.add(group.groupId);
    }
    this.selectedDeviceGroupIds = [...selected];
  }

  async saveDeviceGroupPickerDialog(): Promise<void> {
    const device = this.bindingGroupDevice;
    if (!device) {
      return;
    }
    const previous = this.deviceGroupIdsByDevice;
    const nextGroupIds = [...this.selectedDeviceGroupIds];
    const next = { ...previous };
    if (nextGroupIds.length) {
      next[device.deviceId] = nextGroupIds;
    } else {
      delete next[device.deviceId];
    }
    this.deviceGroupIdsByDevice = next;
    try {
      await this.api.put(WEB_API.deviceGroupsForDevice(this.effectiveUserId, device.deviceId, this.effectiveUserId), {
        actorUserId: this.effectiveUserId,
        groupIds: nextGroupIds,
      });
      await this.loadDeviceGroups(this.effectiveUserId);
      this.closeDeviceGroupPickerDialog();
    } catch {
      if (this.isDemoMode) {
        this.closeDeviceGroupPickerDialog();
        return;
      }
      this.deviceGroupIdsByDevice = previous;
      this.deviceGroupBindingMessage = '保存失败，请重试';
      this.notifyStateChanged();
    }
  }

  openDeviceGroupDialog(group?: DeviceGroupRow): void {
    this.closeInlinePopovers();
    this.deviceListMessage = '';
    this.editingDeviceGroup = group?.groupId ? group : null;
    this.deviceGroupDialogMode = group?.groupId ? 'edit' : 'create';
    this.deviceGroupName = group?.name ?? '';
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
    const description = this.deviceGroupDescription.trim();
    if (this.editingDeviceGroup) {
      const groupId = this.editingDeviceGroup.groupId;
      const previousGroups = this.deviceGroups;
      this.deviceGroups = previousGroups.map((group) => group.groupId === groupId ? { ...group, name, description } : group);
      try {
        const group = await this.api.patch<DeviceGroupRow>(WEB_API.deviceGroup(this.effectiveUserId, groupId, this.effectiveUserId), {
          actorUserId: this.effectiveUserId,
          name,
          description,
        });
        this.deviceGroups = this.deviceGroups.map((item) => item.groupId === group.groupId ? { ...item, ...group } : item);
        this.closeDeviceGroupDialog();
      } catch {
        if (this.isDemoMode) {
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
        const group = await this.api.post<DeviceGroupRow>(WEB_API.deviceGroups(this.effectiveUserId), {
          actorUserId: this.effectiveUserId,
          name,
          description,
        });
        this.deviceGroups = [...this.deviceGroups.filter((item) => item.groupId !== localGroup.groupId), { ...group }];
        this.closeDeviceGroupDialog();
      } catch {
        if (this.isDemoMode) {
          this.closeDeviceGroupDialog();
          return;
        }
        this.deviceGroups = this.deviceGroups.filter((item) => item.groupId !== localGroup.groupId);
        this.deviceGroupDialogMessage = '保存失败，请重试';
        this.notifyStateChanged();
      }
    }
  }

  async quickCreateDeviceGroup(group: Pick<DeviceGroupRow, 'name' | 'description'>): Promise<void> {
    this.deviceListMessage = '';
    const name = group.name.trim();
    if (!name) {
      return;
    }
    if (this.deviceGroups.some((item) => item.name.trim() === name)) {
      this.deviceListMessage = `分组“${name}”已存在`;
      this.notifyStateChanged();
      return;
    }
    const description = group.description.trim();
    const localGroup: DeviceGroupRow = {
      groupId: compactUuid(),
      name,
      description,
      createdAt: Math.floor(Date.now() / 1000),
    };
    this.deviceGroups = [...this.deviceGroups, localGroup];
    this.notifyStateChanged();
    try {
      const created = await this.api.post<DeviceGroupRow>(WEB_API.deviceGroups(this.effectiveUserId), {
        actorUserId: this.effectiveUserId,
        name,
        description,
      });
      this.deviceGroups = [
        ...this.deviceGroups.filter((item) => item.groupId !== localGroup.groupId),
        { ...created },
      ];
      this.notifyStateChanged();
    } catch {
      if (this.isDemoMode) {
        return;
      }
      this.deviceGroups = this.deviceGroups.filter((item) => item.groupId !== localGroup.groupId);
      this.deviceListMessage = `创建分组“${name}”失败`;
      this.notifyStateChanged();
    }
  }

  async removeDeviceGroup(group: DeviceGroupRow): Promise<void> {
    const previousGroups = this.deviceGroups;
    const previousGroupIdsByDevice = this.deviceGroupIdsByDevice;
    this.deviceGroups = this.deviceGroups.filter((item) => item.groupId !== group.groupId);
    this.deviceGroupIdsByDevice = Object.fromEntries(
      Object.entries(this.deviceGroupIdsByDevice)
        .map(([deviceId, groupIds]) => [deviceId, groupIds.filter((groupId) => groupId !== group.groupId)] as const)
        .filter(([, groupIds]) => groupIds.length > 0),
    );
    try {
      await this.api.delete(WEB_API.deviceGroup(this.effectiveUserId, group.groupId, this.effectiveUserId));
    } catch {
      if (this.isDemoMode) {
        return;
      }
      this.deviceGroups = previousGroups;
      this.deviceGroupIdsByDevice = previousGroupIdsByDevice;
      this.deviceListMessage = '删除设备分组失败';
      this.notifyStateChanged();
      return;
    }
    this.notifyStateChanged();
  }

  openDeviceAliasDialog(device: DeviceRow): void {
    this.closeInlinePopovers();
    if (this.showDeviceAliasDialog && this.editingDevice?.deviceId === device.deviceId) {
      this.closeDeviceAliasDialog();
      return;
    }
    this.editingDevice = device;
    this.deviceListMessage = '';
    this.deviceAliasDialogMessage = '';
    this.deviceAliasValue = device.alias;
    this.showDeviceAliasDialog = true;
  }

  closeDeviceAliasDialog(): void {
    this.showDeviceAliasDialog = false;
    this.editingDevice = null;
    this.deviceAliasDialogMessage = '';
  }

  async saveDeviceAliasDialog(): Promise<void> {
    if (!this.editingDevice || !this.deviceAliasValue.trim()) {
      return;
    }
    const alias = this.deviceAliasValue.trim();
    const deviceID = this.editingDevice.deviceId;
    try {
      const updated = await this.api.patch<ApiDevice>(WEB_API.device(deviceID), {
        actorUserId: this.effectiveUserId,
        alias,
      });
      this.devices = this.devices.map((item) => item.deviceId === deviceID ? this.mapDevice(updated) : item);
    } catch {
      if (!this.isDemoMode) {
        this.deviceAliasDialogMessage = '更新设备别名失败';
        this.notifyStateChanged();
        return;
      }
      this.devices = this.devices.map((item) => item.deviceId === deviceID ? { ...item, alias } : item);
    }
    this.closeDeviceAliasDialog();
    this.notifyStateChanged();
  }
  async openInviteDialog(): Promise<void> {
    this.closeInlinePopovers();
    this.deviceListMessage = '';
    const network = this.workspaces.find((item) => item.workspaceId === this.selectedWorkspaceId) ?? this.workspaces[0];
    if (!network) {
      this.workspaceInviteCode = '';
      this.inviteQrDataUrl = '';
      this.joinInviteMessage = '请先创建网络。';
      this.showInviteDialog = true;
      this.notifyStateChanged();
      return;
    }
    this.selectedWorkspaceId = network.workspaceId;
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
        networkId: network.workspaceId,
        inviterUserId: this.effectiveUserId,
        ttlSeconds: 86400,
      });
      this.workspaceInviteCode = invite.inviteCode;
      this.upsertWorkspaceDeviceInvite(invite);
    } catch {
      if (!this.isDemoMode) {
        this.workspaceInviteCode = '';
        this.inviteQrDataUrl = '';
        this.joinInviteMessage = '生成接入码失败';
        this.showInviteDialog = true;
        this.notifyStateChanged();
        return;
      }
      const randomPart = Math.random().toString(36).slice(2, 8).toUpperCase();
      this.workspaceInviteCode = `JOIN-${randomPart}`;
      this.upsertWorkspaceDeviceInvite({
        inviteId: compactUuid(),
        networkId: network.workspaceId,
        workspaceId: network.workspaceId,
        inviterUserId: this.effectiveUserId,
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
    this.closeInlinePopovers();
    this.deviceListMessage = '';
    const network = this.workspaces.find((item) => item.workspaceId === this.bootstrapNetworkId) ?? this.workspaces[0];
    if (!network) {
      this.bootstrapMessage = '请先创建网络。';
      this.showBootstrapDialog = true;
      this.notifyStateChanged();
      return;
    }
    this.bootstrapNetworkId = network.workspaceId;
    this.bootstrapInstallationKey = '';
    this.bootstrapQrDataUrl = '';
    this.bootstrapInstallCommand = '';
    this.bootstrapMessage = '';
    try {
      const key = await this.api.post<ApiDeviceBootstrapKey>(WEB_API.deviceBootstrapKeys(), {
        userId: this.effectiveUserId,
        actorUserId: this.effectiveUserId,
        networkId: network.networkId,
        ttlSeconds: this.bootstrapTTLSeconds,
      });
      this.bootstrapInstallationKey = key.installationKey || key.token || key.key || '';
      this.deviceBootstrapKeys = [
        key,
        ...this.deviceBootstrapKeys.filter((item) => item.id !== key.id),
      ];
    } catch {
      if (!this.isDemoMode) {
        this.bootstrapMessage = '生成安装引导失败';
        this.showBootstrapDialog = true;
        this.notifyStateChanged();
        return;
      }
      const now = Math.floor(Date.now() / 1000);
      this.bootstrapInstallationKey = `ik_${Array.from({ length: 64 }, () => Math.floor(Math.random() * 16).toString(16)).join('')}`;
      this.deviceBootstrapKeys = [
        {
          id: compactUuid(),
          key: this.bootstrapInstallationKey,
          installationKey: this.bootstrapInstallationKey,
          createdByUserId: this.effectiveUserId,
          networkId: network.networkId,
          expiresAt: now + this.bootstrapTTLSeconds,
          status: 'active',
          createdAt: now,
        },
        ...this.deviceBootstrapKeys,
      ];
    }
    const server = window.location.origin.replace(/^https?:\/\/web\./, 'http://api.');
    this.bootstrapInstallCommand = `curl -fsSL ${server}/downloads/clients/install.sh | sudo bash -s -- --server ${server} --installation-key ${this.bootstrapInstallationKey}`;
    this.bootstrapQrDataUrl = await QRCode.toDataURL(this.bootstrapInstallCommand, {
      errorCorrectionLevel: 'M',
      margin: 2,
      scale: 6,
      color: {
        dark: '#111827',
        light: '#ffffff',
      },
    });
    this.showBootstrapDialog = true;
    this.notifyStateChanged();
  }

  async revokeBootstrapKey(item: ApiDeviceBootstrapKey): Promise<void> {
    const previous = this.deviceBootstrapKeys;
    const revokedAt = Math.floor(Date.now() / 1000);
    this.deviceBootstrapKeys = previous.map((current) =>
      current.id === item.id ? { ...current, status: 'revoked', revokedAt } : current,
    );
    try {
      const updated = await this.api.post<ApiDeviceBootstrapKey>(WEB_API.deviceBootstrapKeyRevoke(item.id, this.effectiveUserId), {});
      this.deviceBootstrapKeys = this.deviceBootstrapKeys.map((current) => current.id === item.id ? updated : current);
    } catch {
      if (!this.isDemoMode) {
        this.deviceBootstrapKeys = previous;
        this.bootstrapMessage = '撤销安装引导失败';
        this.notifyStateChanged();
        return;
      }
    }
    this.notifyStateChanged();
  }

  closeBootstrapDialog(): void {
    this.showBootstrapDialog = false;
    this.bootstrapMessage = '';
  }

  closeInviteDialog(): void {
    this.showInviteDialog = false;
    this.joinInviteMessage = '';
  }

  openJoinDialog(device: DeviceRow): void {
    this.closeInlinePopovers();
    this.deviceListMessage = '';
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
        actorUserId: this.effectiveUserId,
      });
      if (result.invite) {
        this.upsertWorkspaceDeviceInvite(result.invite);
      }
      await this.loadDashboard(this.currentUserId);
    } catch {
      if (!this.isDemoMode) {
        this.joinInviteMessage = '邀请码确认失败';
        this.notifyStateChanged();
        return;
      }
      this.markInviteAccepted(this.joinInviteCode, '', this.deviceId);
    }
    this.showJoinDialog = false;
  }

  openDeviceExposureDialog(device: DeviceRow): void {
    this.closeInlinePopovers();
    this.selectedExposureDevice = device;
    this.exposureUser = '';
    this.showDeviceExposureDialog = true;
  }

  closeDeviceExposureDialog(): void {
    this.closeDeviceExposureDialogState();
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
      await this.api.delete(WEB_API.device(device.deviceId, this.effectiveUserId));
      await this.loadDashboard(this.currentUserId);
      return;
    } catch {
      if (!this.isDemoMode) {
        this.deviceListMessage = '删除设备失败';
        this.notifyStateChanged();
        return;
      }
    }
    this.devices = this.devices.filter((item) => item.deviceId !== device.deviceId);
    this.notifyStateChanged();
  }

  removeMember(member: MemberRow): void {
    if (member.role === 'owner') {
      return;
    }
    this.members = this.members.filter((item) => item !== member);
  }

}
