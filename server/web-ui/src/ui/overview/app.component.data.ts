import { panelFromRoute } from '../app-routing';
import { AppComponentState } from '../app.component.state';
import {
  ApiDevice,
  ApiDeviceBootstrapKey,
  ApiDeviceGroup,
  ApiDeviceGroupMember,
  ApiManagedDeviceSession,
  ApiManagedUserSession,
  ClientDownload,
  ApiDNSRecord,
  ApiDNSZone,
  ApiSecurityGroup,
  ApiSecurityRule,
  ApiWorkspace,
  ApiWorkspaceDevice,
  ApiDeviceQuota,
  DeviceExposureRow,
  DeviceRow,
  DNSRow,
  DNSZoneRow,
  MemberRow,
  NavItem,
  RuleSubjectType,
  SecurityGroupRow,
  SecurityRuleRow,
  SecurityRuleTemplate,
  WorkspaceDeviceInviteRow,
  WorkspacePanel,
  WorkspaceRow,
} from '../app.models';
import { slug } from '../app.utils';
import { WEB_API } from '../api-paths';

export abstract class AppComponentData extends AppComponentState {
  protected override async loadClientDownloads(): Promise<void> {
    try {
      const response = await this.api.get<{ items: ClientDownload[] }>(WEB_API.clientDownloads);
      this.clientDownloads = response.items ?? [];
    } catch {
      this.clientDownloads = [];
    }
  }

  protected override applyRouteFromLocation(): void {
    const path = window.location.pathname;
    if (path === '/' || path === '/overview') {
      this.closeInlinePopovers();
      this.closeOverlayDialogs();
      this.active = 'overview';
      this.workspaceRouteMode = 'list';
      return;
    }
    if (path === '/devices') {
      this.closeInlinePopovers();
      this.closeOverlayDialogs();
      this.active = 'devices';
      this.devicePanel = 'list';
      this.workspaceRouteMode = 'list';
      return;
    }
    if (path === '/devices/groups') {
      this.closeInlinePopovers();
      this.closeOverlayDialogs();
      this.active = 'devices';
      this.devicePanel = 'groups';
      this.workspaceRouteMode = 'list';
      return;
    }
    const match = path.match(/^\/space\/([^/]+)(?:\/(.+))?$/);
    if (match) {
      this.closeInlinePopovers();
      this.closeOverlayDialogs();
      this.selectedWorkspaceId = decodeURIComponent(match[1]);
      this.editingWorkspaceName = this.selectedWorkspace.name;
      const routePanel = panelFromRoute(match[2] ?? '');
      this.workspacePanel = routePanel.panel;
      this.selectedZoneId = routePanel.selectedZoneId ?? this.selectedZoneId;
      this.selectedSecurityGroupId = routePanel.selectedSecurityGroupId ?? this.selectedSecurityGroupId;
      this.syncWorkspaceSelectionState();
      this.workspaceRouteMode = 'detail';
      this.active = 'workspaces';
      void this.loadWorkspaceResources(this.selectedWorkspaceId);
      return;
    }
    if (window.location.pathname === '/spaces') {
      this.closeInlinePopovers();
      this.closeOverlayDialogs();
      this.workspaceRouteMode = 'list';
      this.active = 'workspaces';
    }
  }

  protected override async loadDashboard(userId = ''): Promise<void> {
    try {
      const [devices, workspaces, invites, bootstrap, sessions, quota] = await Promise.all([
        this.api.get<{ items: ApiDevice[] }>(WEB_API.devicesVisible(userId)),
        this.api.get<{ items: ApiWorkspace[] }>(WEB_API.networks(userId)),
        this.api.get<{ items: WorkspaceDeviceInviteRow[] }>(WEB_API.deviceInvites(userId)),
        this.api.get<{ items: ApiDeviceBootstrapKey[] }>(WEB_API.deviceBootstrapKeys(userId)),
        userId ? this.api.get<{ items: ApiManagedUserSession[] }>(WEB_API.userSessions(userId)) : Promise.resolve({ items: [] }),
        userId ? this.api.get<ApiDeviceQuota>(WEB_API.userEntitlement(userId)) : Promise.resolve(null),
      ]);
      this.devices = (devices.items ?? []).map((device) => this.mapDevice(device));
      this.deviceQuota = quota;
      this.workspaceDeviceInvites = invites.items ?? [];
      this.deviceBootstrapKeys = bootstrap.items ?? [];
      this.userSessions = sessions.items ?? [];
      await this.loadDeviceGroups(userId);
      this.workspaces = (workspaces.items ?? []).map((workspace) => ({
        networkId: workspace.networkId,
        workspaceId: workspace.networkId,
        name: workspace.name,
        intraGroupPolicy: workspace.intraGroupPolicy === 'deny' ? 'deny' : 'allow',
        default: !!workspace.default,
        members: workspace.members ?? 0,
        devices: workspace.devices ?? 0,
        zone: workspace.zone || `${slug(workspace.name)}.${workspace.networkId || 'network'}.${userId || 'user'}.sub.staticlss.com`,
      }));
      await Promise.all(this.workspaces.map((workspace) => this.loadWorkspaceDevices(workspace.workspaceId)));
      await this.loadCurrentUserDeviceSessions();
      this.syncWorkspaceSelectionState();
    } catch {
      if (!this.isDemoMode) {
        this.devices = [];
        this.deviceQuota = null;
        this.workspaceDeviceInvites = [];
        this.deviceBootstrapKeys = [];
        this.userSessions = [];
        this.deviceSessionsByDeviceId = {};
        this.deviceGroups = [];
        this.deviceGroupIdsByDevice = {};
        this.workspaceDeviceGroupsByWorkspace = {};
        this.workspaceDeviceGroupIdsByDeviceByWorkspace = {};
        this.workspaceDeviceIdsByWorkspace = {};
        this.workspaceDeviceJoinMethods = {};
        this.workspaces = [];
        this.syncWorkspaceSelectionState();
      }
    }
  }

  protected async loadDeviceGroups(userId: string): Promise<void> {
    if (!userId) {
      return;
    }
    try {
      const response = await this.api.get<{ items: ApiDeviceGroup[]; members: ApiDeviceGroupMember[] }>(WEB_API.deviceGroups(userId));
      this.deviceGroups = (response.items ?? []).map((group) => ({
        groupId: group.groupId,
        name: group.name,
        description: group.description ?? '',
        createdAt: group.createdAt,
        updatedAt: group.updatedAt,
      }));
      const next: Record<string, string[]> = {};
      for (const member of response.members ?? []) {
        next[member.deviceId] = [...(next[member.deviceId] ?? []), member.groupId];
      }
      this.deviceGroupIdsByDevice = next;
    } catch {
      if (!this.isDemoMode) {
        this.deviceGroups = [];
        this.deviceGroupIdsByDevice = {};
      }
    }
  }

  protected async loadWorkspaceDeviceGroups(workspaceId: string): Promise<void> {
    try {
      const response = await this.api.get<{ items: ApiDeviceGroup[]; members: ApiDeviceGroupMember[] }>(WEB_API.networkDeviceGroups(workspaceId));
      this.workspaceDeviceGroupsByWorkspace = {
        ...this.workspaceDeviceGroupsByWorkspace,
        [workspaceId]: (response.items ?? []).map((group) => ({
          groupId: group.groupId,
          name: group.name,
          description: group.description ?? '',
          createdAt: group.createdAt,
          updatedAt: group.updatedAt,
        })),
      };
      const next: Record<string, string[]> = {};
      for (const member of response.members ?? []) {
        next[member.deviceId] = [...(next[member.deviceId] ?? []), member.groupId];
      }
      this.workspaceDeviceGroupIdsByDeviceByWorkspace = {
        ...this.workspaceDeviceGroupIdsByDeviceByWorkspace,
        [workspaceId]: next,
      };
    } catch {
      if (!this.isDemoMode) {
        this.workspaceDeviceGroupsByWorkspace = {
          ...this.workspaceDeviceGroupsByWorkspace,
          [workspaceId]: [],
        };
        this.workspaceDeviceGroupIdsByDeviceByWorkspace = {
          ...this.workspaceDeviceGroupIdsByDeviceByWorkspace,
          [workspaceId]: {},
        };
      }
    }
  }

  protected async loadCurrentUserDeviceSessions(): Promise<void> {
    const devices = this.currentUserDevices;
    if (!devices.length) {
      this.deviceSessionsByDeviceId = {};
      return;
    }
    const entries = await Promise.all(devices.map(async (device) => {
      try {
        const response = await this.api.get<{ items: ApiManagedDeviceSession[] }>(WEB_API.deviceSessions(device.deviceId));
        return [device.deviceId, response.items ?? []] as const;
      } catch {
        return [device.deviceId, []] as const;
      }
    }));
    this.deviceSessionsByDeviceId = Object.fromEntries(entries);
  }

  protected override async loadWorkspaceDevices(workspaceId: string): Promise<void> {
    try {
      const response = await this.api.get<{ items: ApiWorkspaceDevice[] }>(WEB_API.networkDevices(workspaceId));
      const items = response.items ?? [];
      const deviceIds = items.map((item) => item.deviceId);
      this.workspaceDeviceIdsByWorkspace = { ...this.workspaceDeviceIdsByWorkspace, [workspaceId]: deviceIds };
      this.workspaceDeviceJoinMethods = {
        ...this.workspaceDeviceJoinMethods,
        ...Object.fromEntries(items.map((item) => [`${workspaceId}|${item.deviceId}`, '手动添加'])),
      };
      this.workspaces = this.workspaces.map((workspace) => workspace.workspaceId === workspaceId ? { ...workspace, devices: deviceIds.length } : workspace);
      if (this.selectedWorkspaceId === workspaceId) {
        const selected = this.workspaces.find((workspace) => workspace.workspaceId === workspaceId);
        if (selected) {
          this.selectedWorkspaceId = selected.workspaceId;
        }
      }
    } catch {
      if (!this.isDemoMode) {
        const nextJoinMethods = { ...this.workspaceDeviceJoinMethods };
        Object.keys(nextJoinMethods)
          .filter((key) => key.startsWith(`${workspaceId}|`))
          .forEach((key) => delete nextJoinMethods[key]);
        this.workspaceDeviceIdsByWorkspace = { ...this.workspaceDeviceIdsByWorkspace, [workspaceId]: [] };
        this.workspaceDeviceJoinMethods = nextJoinMethods;
        this.workspaces = this.workspaces.map((workspace) => workspace.workspaceId === workspaceId ? { ...workspace, devices: 0 } : workspace);
      }
    }
  }

  protected override async loadWorkspaceResources(workspaceId: string): Promise<void> {
    await Promise.all([
      this.loadWorkspaceDeviceGroups(workspaceId),
      this.loadDNSZones(workspaceId),
      this.loadDNSRecords(workspaceId),
      this.loadSecurityResources(workspaceId),
    ]);
    this.syncWorkspaceSelectionState();
    this.notifyStateChanged();
  }

  private async loadDNSZones(workspaceId: string): Promise<void> {
    try {
      const response = await this.api.get<{ items: ApiDNSZone[] }>(WEB_API.dnsZones(workspaceId));
      this.dnsZones = [...this.dnsZones.filter((zone) => zone.workspaceId !== workspaceId), ...(response.items ?? []).map((zone) => this.mapDNSZone(zone))];
    } catch {
      if (!this.isDemoMode) {
        this.dnsZones = this.dnsZones.filter((zone) => zone.workspaceId !== workspaceId);
      }
    }
  }

  private async loadDNSRecords(workspaceId: string): Promise<void> {
    try {
      const response = await this.api.get<{ items: ApiDNSRecord[] }>(WEB_API.dnsRecords(workspaceId));
      this.dnsRecords = [...this.dnsRecords.filter((record) => record.workspaceId !== workspaceId), ...(response.items ?? []).map((record) => this.mapDNSRecord(record))];
    } catch {
      if (!this.isDemoMode) {
        this.dnsRecords = this.dnsRecords.filter((record) => record.workspaceId !== workspaceId);
      }
    }
  }

  private async loadSecurityResources(workspaceId: string): Promise<void> {
    try {
      const groups = await this.api.get<{ items: ApiSecurityGroup[] }>(WEB_API.securityGroups(workspaceId));
      const groupItems = groups.items ?? [];
      if (groupItems.length === 0) {
        const removedGroupIds = new Set(
          this.securityGroups
            .filter((group) => group.workspaceId === workspaceId)
            .map((group) => group.securityGroupId),
        );
        this.securityGroups = this.securityGroups.filter((group) => group.workspaceId !== workspaceId);
        this.securityRules = this.securityRules.filter((rule) => !removedGroupIds.has(rule.securityGroupId ?? ''));
        this.selectedSecurityGroupId = '';
        this.notifyStateChanged();
        return;
      }
      this.securityGroups = [
        ...this.securityGroups.filter((group) => group.workspaceId !== workspaceId),
        ...groupItems.map((group) => this.mapSecurityGroup(group)),
      ];
      const requestedSecurityGroupId = this.selectedSecurityGroupId;
      const group = groupItems.find((item) => item.securityGroupId === requestedSecurityGroupId) ?? groupItems[0];
      if (!group) {
        return;
      }
      this.selectedSecurityGroupId = group.securityGroupId;
      await this.loadSecurityRules(group.securityGroupId);
    } catch {
      if (!this.isDemoMode) {
        this.securityGroups = this.securityGroups.filter((group) => group.workspaceId !== workspaceId);
      }
    }
  }

  protected override async loadSecurityRules(securityGroupId: string): Promise<void> {
    try {
      const rules = await this.api.get<{ items: ApiSecurityRule[] }>(WEB_API.securityRules(securityGroupId));
      const mappedRules = (rules.items ?? []).map((rule) => this.mapSecurityRule(rule));
      this.securityRules = [
        ...this.securityRules.filter((rule) => rule.securityGroupId !== securityGroupId),
        ...mappedRules,
      ];
      this.notifyStateChanged();
    } catch {
      if (!this.isDemoMode) {
        this.securityRules = this.securityRules.filter((rule) => rule.securityGroupId !== securityGroupId);
        this.notifyStateChanged();
      }
    }
  }

  protected override mapDevice(device: ApiDevice): DeviceRow {
    return {
      deviceId: device.deviceId,
      ownerId: device.ownerId,
      platform: device.platform,
      osVersion: device.osVersion ?? '',
      alias: device.alias?.trim() || '',
      ip: device.globalIp,
      owner: device.ownerEmail || device.ownerId,
      status: device.status,
      networkEnabled: device.networkEnabled ?? false,
      createdAt: device.createdAt,
    };
  }

  protected override mapDNSZone(zone: ApiDNSZone): DNSZoneRow {
    return { zoneId: zone.zoneId, networkId: zone.networkId, workspaceId: zone.networkId, zone: zone.zoneName, recordType: 'A', value: '', status: zone.status };
  }

  protected override mapDNSRecord(record: ApiDNSRecord): DNSRow {
    const networkId = record.networkId;
    const targetType = record.recordType === 'CNAME' ? 'cname' : 'device';
    const value = record.targetDeviceId ? this.dnsRecordValue({ networkId, workspaceId: networkId, name: record.name, fqdn: record.fqdn, recordType: record.recordType, value: '', deviceId: record.targetDeviceId, ttl: record.ttl, targetType }) : (record.cname || '');
    return {
      recordId: record.recordId,
      zoneId: record.zoneId,
      networkId,
      workspaceId: networkId,
      name: record.name,
      fqdn: record.fqdn,
      recordType: record.recordType,
      value,
      deviceId: record.targetDeviceId ?? '',
      ttl: record.ttl,
      targetType,
    };
  }

  protected override mapSecurityGroup(group: ApiSecurityGroup): SecurityGroupRow {
    return {
      securityGroupId: group.securityGroupId,
      networkId: group.networkId,
      workspaceId: group.networkId,
      name: group.name,
      description: group.description,
      createdAt: group.createdAt,
    };
  }

  protected override mapSecurityRule(rule: ApiSecurityRule): SecurityRuleRow {
    const port = rule.portFrom === 0 && rule.portTo === 0 ? 'all' : rule.portFrom === rule.portTo ? String(rule.portFrom) : `${rule.portFrom},${rule.portTo}`;
    return {
      ruleId: rule.ruleId,
      securityGroupId: rule.securityGroupId,
      direction: rule.direction,
      priority: rule.priority,
      action: rule.action,
      protocol: rule.protocol,
      port,
      subjectType: rule.peerType,
      subjectValue: rule.peerValue,
      description: rule.description,
      enabled: rule.enabled,
    };
  }

  protected override upsertWorkspaceDeviceInvite(invite: WorkspaceDeviceInviteRow): void {
    this.workspaceDeviceInvites = [
      invite,
      ...this.workspaceDeviceInvites.filter((item) => item.inviteCode !== invite.inviteCode),
    ];
  }

  protected override markInviteAccepted(inviteCode: string, workspaceId: string, deviceId: string): void {
    const now = Math.floor(Date.now() / 1000);
    this.workspaceDeviceInvites = this.workspaceDeviceInvites.map((invite) => {
      if (invite.inviteCode !== inviteCode) {
        return invite;
      }
      return {
        ...invite,
        workspaceId,
        status: 'accepted',
        acceptedDeviceId: deviceId,
        acceptedUserId: this.devices.find((device) => device.deviceId === deviceId)?.owner ?? this.currentUser,
        acceptedAt: now,
      };
    });
  }
}
