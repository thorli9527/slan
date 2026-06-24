import { panelFromRoute } from '../app-routing';
import { AppComponentSecurity } from '../network/app.component.security';
import {
  ApiDevice,
  ApiDeviceBootstrapKey,
  ApiDeviceGroup,
  ApiDeviceGroupMember,
  ClientDownload,
  ApiDNSRecord,
  ApiDNSZone,
  ApiPublicMapping,
  ApiSecurityGroup,
  ApiSecurityRule,
  ApiUserAlias,
  ApiWorkspace,
  ApiWorkspaceDevice,
  ApiDeviceQuota,
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
import { compactUuid, slug } from '../app.utils';
import { WEB_API } from '../api-paths';

export abstract class AppComponentData extends AppComponentSecurity {
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
      this.active = 'overview';
      this.workspaceRouteMode = 'list';
      return;
    }
    if (path === '/devices') {
      this.active = 'devices';
      this.devicePanel = 'list';
      this.workspaceRouteMode = 'list';
      return;
    }
    if (path === '/devices/groups') {
      this.active = 'devices';
      this.devicePanel = 'groups';
      this.workspaceRouteMode = 'list';
      return;
    }
    if (path === '/user-aliases') {
      this.active = 'userAliases';
      this.workspaceRouteMode = 'list';
      return;
    }
    const match = path.match(/^\/space\/([^/]+)(?:\/(.+))?$/);
    if (match) {
      this.selectedWorkspaceId = decodeURIComponent(match[1]);
      this.editingWorkspaceName = this.selectedWorkspace.name;
      const routePanel = panelFromRoute(match[2] ?? '');
      this.workspacePanel = routePanel.panel === 'publicMappings' && !this.publicMappingsEnabled ? 'zones' : routePanel.panel;
      this.selectedZoneId = routePanel.selectedZoneId ?? this.selectedZoneId;
      this.selectedSecurityGroupId = routePanel.selectedSecurityGroupId ?? this.selectedSecurityGroupId;
      this.workspaceRouteMode = 'detail';
      this.active = 'workspaces';
      void this.loadWorkspaceResources(this.selectedWorkspaceId);
      return;
    }
    if (window.location.pathname === '/spaces') {
      this.workspaceRouteMode = 'list';
      this.active = 'workspaces';
    }
  }

  protected override async loadDashboard(userId = ''): Promise<void> {
    try {
      const [devices, workspaces, aliases, invites, bootstrap, quota] = await Promise.all([
        this.api.get<{ items: ApiDevice[] }>(WEB_API.devicesVisible(userId)),
        this.api.get<{ items: ApiWorkspace[] }>(WEB_API.networks(userId)),
        this.api.get<{ items: ApiUserAlias[] }>(WEB_API.userAliasesForUser(userId)),
        this.api.get<{ items: WorkspaceDeviceInviteRow[] }>(WEB_API.deviceInvites(userId)),
        this.api.get<{ items: ApiDeviceBootstrapKey[] }>(WEB_API.deviceBootstrapKeys(userId)),
        userId ? this.api.get<ApiDeviceQuota>(WEB_API.userEntitlement(userId)) : Promise.resolve(null),
      ]);
      this.devices = (devices.items ?? []).map((device) => this.mapDevice(device));
      this.deviceQuota = quota;
      this.userAliases = (aliases.items ?? []).map((item) => ({ email: item.email, alias: item.alias }));
      this.workspaceDeviceInvites = invites.items ?? [];
      this.deviceBootstrapKeys = bootstrap.items ?? [];
      await this.loadDeviceGroups(userId);
      this.workspaces = (workspaces.items ?? []).map((workspace) => ({
        networkId: workspace.networkId,
        workspaceId: workspace.networkId,
        name: workspace.name,
        code: workspace.code,
        template: workspace.templateKey ?? 'custom',
        intraGroupPolicy: workspace.intraGroupPolicy === 'deny' ? 'deny' : 'allow',
        default: !!workspace.default,
        members: workspace.members ?? 0,
        devices: workspace.devices ?? 0,
        zone: workspace.zone || `${workspace.code || slug(workspace.name)}.${workspace.networkId || 'network'}.${userId || 'user'}.sub.staticlss.com`,
      }));
      await Promise.all(this.workspaces.map((workspace) => this.loadWorkspaceDevices(workspace.workspaceId)));
    } catch {
      if (!this.isDemoMode) {
        this.devices = [];
        this.deviceQuota = null;
        this.userAliases = [];
        this.workspaceDeviceInvites = [];
        this.deviceBootstrapKeys = [];
        this.deviceGroups = [];
        this.deviceGroupIdsByDevice = {};
        this.workspaceDeviceIdsByWorkspace = {};
        this.workspaceDeviceJoinMethods = {};
        this.workspaces = [];
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
    const tasks = [
      this.loadDNSZones(workspaceId),
      this.loadDNSRecords(workspaceId),
      this.loadSecurityResources(workspaceId),
    ];
    if (this.publicMappingsEnabled) {
      tasks.push(this.loadPublicMappings(workspaceId));
    }
    await Promise.all(tasks);
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

  private async loadPublicMappings(workspaceId: string): Promise<void> {
    try {
      const response = await this.api.get<{ items: ApiPublicMapping[] }>(WEB_API.publicMappings(workspaceId));
      this.publicMappings = [...this.publicMappings.filter((mapping) => mapping.workspaceId !== workspaceId), ...(response.items ?? []).map((mapping) => this.mapPublicMapping(mapping))];
    } catch {
      if (!this.isDemoMode) {
        this.publicMappings = this.publicMappings.filter((mapping) => mapping.workspaceId !== workspaceId);
      }
    }
  }

  private async loadSecurityResources(workspaceId: string): Promise<void> {
    try {
      const groups = await this.api.get<{ items: ApiSecurityGroup[] }>(WEB_API.securityGroups(workspaceId));
      const groupItems = groups.items ?? [];
      if (groupItems.length === 0) {
        this.ensureDefaultSecuritySeed(workspaceId);
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
        ...(mappedRules.length > 0 ? mappedRules : this.defaultSecurityRules(securityGroupId)),
      ];
      this.notifyStateChanged();
    } catch {
      if (!this.isDemoMode) {
        this.securityRules = this.securityRules.filter((rule) => rule.securityGroupId !== securityGroupId);
        this.notifyStateChanged();
      }
    }
  }

  private ensureDefaultSecuritySeed(workspaceId: string): void {
    const existing = this.securityGroups.find((group) => group.workspaceId === workspaceId);
    const group = existing ?? {
      securityGroupId: compactUuid(),
      networkId: workspaceId,
      workspaceId,
      name: '',
      createdAt: Math.floor(Date.now() / 1000),
    };
    if (!existing) {
      this.securityGroups = [...this.securityGroups, group];
    }
    this.selectedSecurityGroupId = group.securityGroupId;
    this.securityRules = [
      ...this.securityRules.filter((rule) => rule.securityGroupId !== group.securityGroupId),
      ...this.defaultSecurityRules(group.securityGroupId),
    ];
  }

  private defaultSecurityRules(securityGroupId: string): SecurityRuleRow[] {
    return [
      { ruleId: compactUuid(), securityGroupId, direction: 'ingress', priority: 100, action: 'allow', protocol: 'tcp', port: '22', subjectType: 'workspace', subjectValue: 'self', enabled: true },
      { ruleId: compactUuid(), securityGroupId, direction: 'egress', priority: 100, action: 'allow', protocol: 'all', port: 'all', subjectType: 'all', subjectValue: 'all', enabled: true },
    ];
  }

  protected override mapDevice(device: ApiDevice): DeviceRow {
    return {
      deviceId: device.deviceId,
      platform: device.platform,
      osVersion: device.osVersion ?? '',
      alias: device.alias || device.name || device.deviceId,
      ip: device.globalIp,
      owner: device.ownerEmail || device.ownerId,
      status: device.status,
    };
  }

  protected override mapDNSZone(zone: ApiDNSZone): DNSZoneRow {
    return { zoneId: zone.zoneId, networkId: zone.networkId, workspaceId: zone.networkId, zone: zone.zoneName, recordType: 'A', value: '', expose: zone.exposeGlobal, status: zone.status };
  }

  protected override mapDNSRecord(record: ApiDNSRecord): DNSRow {
    const port = record.port ?? '';
    const networkId = record.networkId;
    const targetType = record.targetDeviceId ? 'device' : record.targetIp ? 'ip' : record.cname ? 'cname' : 'device';
    const value = record.targetDeviceId ? this.dnsRecordValue({ networkId, workspaceId: networkId, name: record.name, fqdn: record.fqdn, recordType: record.recordType, value: '', deviceId: record.targetDeviceId, port, ttl: record.ttl, targetType, expose: false }) : (record.targetIp || record.cname || '');
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
      port,
      ttl: record.ttl,
      targetType,
      expose: false,
    };
  }

  protected override mapPublicMapping(mapping: ApiPublicMapping): PublicMappingRow {
    const targetType = mapping.sourceRecord && mapping.sourceRecord !== mapping.deviceId && mapping.sourceRecord !== mapping.internalIp
      ? 'record'
      : mapping.deviceId
        ? 'device'
        : mapping.internalIp
          ? 'ip'
          : 'device';
    return {
      mappingId: mapping.mappingId,
      networkId: mapping.networkId,
      workspaceId: mapping.networkId,
      alias: mapping.alias,
      publicDomain: mapping.publicDomain,
      sourceRecord: mapping.sourceRecord,
      deviceId: mapping.deviceId,
      internalIp: mapping.internalIp,
      targetType,
      protocol: mapping.protocol,
      internalPort: mapping.internalPort ? String(mapping.internalPort) : mapping.port,
      port: mapping.port,
      externalPort: mapping.externalPort,
      accessMode: mapping.accessMode || 'public',
      tlsMode: mapping.tlsMode || 'auto',
      status: mapping.status,
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
