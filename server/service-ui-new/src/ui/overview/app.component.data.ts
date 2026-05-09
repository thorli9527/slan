import { panelFromRoute } from '../app-routing';
import { AppComponentSecurity } from '../network/app.component.security';
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
import { slug } from '../app.utils';

export abstract class AppComponentData extends AppComponentSecurity {
  protected override applyRouteFromLocation(): void {
    const path = window.location.pathname;
    if (path === '/' || path === '/overview') {
      this.active = 'overview';
      this.workspaceRouteMode = 'list';
      return;
    }
    if (path === '/devices') {
      this.active = 'devices';
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
      return;
    }
    if (window.location.pathname === '/spaces') {
      this.workspaceRouteMode = 'list';
      this.active = 'workspaces';
    }
  }

  protected override async loadDashboard(userId = ''): Promise<void> {
    try {
      const [devices, workspaces, aliases, invites, quota] = await Promise.all([
        this.api.get<{ items: ApiDevice[] }>(`/api/devices/visible${userId ? `?userId=${encodeURIComponent(userId)}` : ''}`),
        this.api.get<{ items: ApiWorkspace[] }>(`/api/networks${userId ? `?userId=${encodeURIComponent(userId)}` : ''}`),
        this.api.get<{ items: ApiUserAlias[] }>(`/api/user-aliases${userId ? `?ownerUserId=${encodeURIComponent(userId)}` : ''}`),
        this.api.get<{ items: WorkspaceDeviceInviteRow[] }>(`/api/device-invites${userId ? `?userId=${encodeURIComponent(userId)}` : ''}`),
        userId ? this.api.get<ApiDeviceQuota>(`/api/users/${encodeURIComponent(userId)}/entitlement`) : Promise.resolve(null),
      ]);
      this.devices = devices.items.map((device) => this.mapDevice(device));
      this.deviceQuota = quota;
      this.userAliases = aliases.items.map((item) => ({ email: item.email, alias: item.alias }));
      this.workspaceDeviceInvites = invites.items;
      this.workspaces = workspaces.items.map((workspace) => ({
        networkId: workspace.networkId,
        workspaceId: workspace.networkId,
        name: workspace.name,
        code: workspace.code,
        template: workspace.templateKey ?? 'custom',
        status: workspace.status,
        members: 0,
        devices: 0,
        zone: `${workspace.code || slug(workspace.name)}.${workspace.networkId || 'network'}.${userId || 'user'}.sub.slan.com`,
      }));
      await Promise.all(this.workspaces.map((workspace) => this.loadWorkspaceDevices(workspace.workspaceId)));
    } catch {
      // The checked-in UI remains previewable without a running API.
    }
  }

  protected override async loadWorkspaceDevices(workspaceId: string): Promise<void> {
    try {
      const response = await this.api.get<{ items: ApiWorkspaceDevice[] }>(`/api/networks/${encodeURIComponent(workspaceId)}/devices`);
      const deviceIds = response.items.map((item) => item.deviceId);
      this.workspaceDeviceIdsByWorkspace = { ...this.workspaceDeviceIdsByWorkspace, [workspaceId]: deviceIds };
      this.workspaceDeviceJoinMethods = {
        ...this.workspaceDeviceJoinMethods,
        ...Object.fromEntries(response.items.map((item) => [`${workspaceId}|${item.deviceId}`, '手动添加'])),
      };
      this.workspaces = this.workspaces.map((workspace) => workspace.workspaceId === workspaceId ? { ...workspace, devices: deviceIds.length } : workspace);
      if (this.selectedWorkspaceId === workspaceId) {
        const selected = this.workspaces.find((workspace) => workspace.workspaceId === workspaceId);
        if (selected) {
          this.selectedWorkspaceId = selected.workspaceId;
        }
      }
    } catch {
      // Preview seed data remains available without the API.
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
  }

  private async loadDNSZones(workspaceId: string): Promise<void> {
    try {
      const response = await this.api.get<{ items: ApiDNSZone[] }>(`/api/networks/${encodeURIComponent(workspaceId)}/dns/zones`);
      this.dnsZones = [...this.dnsZones.filter((zone) => zone.workspaceId !== workspaceId), ...response.items.map((zone) => this.mapDNSZone(zone))];
    } catch {
      // Preview seed data remains available without the API.
    }
  }

  private async loadDNSRecords(workspaceId: string): Promise<void> {
    try {
      const response = await this.api.get<{ items: ApiDNSRecord[] }>(`/api/networks/${encodeURIComponent(workspaceId)}/dns/records`);
      this.dnsRecords = [...this.dnsRecords.filter((record) => record.workspaceId !== workspaceId), ...response.items.map((record) => this.mapDNSRecord(record))];
    } catch {
      // Preview seed data remains available without the API.
    }
  }

  private async loadPublicMappings(workspaceId: string): Promise<void> {
    try {
      const response = await this.api.get<{ items: ApiPublicMapping[] }>(`/api/networks/${encodeURIComponent(workspaceId)}/public-mappings`);
      this.publicMappings = [...this.publicMappings.filter((mapping) => mapping.workspaceId !== workspaceId), ...response.items.map((mapping) => this.mapPublicMapping(mapping))];
    } catch {
      // Preview seed data remains available without the API.
    }
  }

  private async loadSecurityResources(workspaceId: string): Promise<void> {
    try {
      const groups = await this.api.get<{ items: ApiSecurityGroup[] }>(`/api/networks/${encodeURIComponent(workspaceId)}/security-groups`);
      this.securityGroups = [
        ...this.securityGroups.filter((group) => group.workspaceId !== workspaceId),
        ...groups.items.map((group) => this.mapSecurityGroup(group)),
      ];
      const group = groups.items[0];
      if (!group) {
        return;
      }
      this.selectedSecurityGroupId = group.securityGroupId;
      await this.loadSecurityRules(group.securityGroupId);
    } catch {
      // Preview seed data remains available without the API.
    }
  }

  protected override async loadSecurityRules(securityGroupId: string): Promise<void> {
    try {
      const rules = await this.api.get<{ items: ApiSecurityRule[] }>(`/api/security-groups/${encodeURIComponent(securityGroupId)}/rules`);
      this.securityRules = rules.items.map((rule) => this.mapSecurityRule(rule));
    } catch {
      // Preview seed data remains available without the API.
    }
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
    const value = record.targetDeviceId ? this.dnsRecordValue({ networkId, workspaceId: networkId, name: record.name, fqdn: record.fqdn, recordType: record.recordType, value: '', deviceId: record.targetDeviceId, port, expose: false }) : (record.targetIp || record.cname || '');
    return { recordId: record.recordId, zoneId: record.zoneId, networkId, workspaceId: networkId, name: record.name, fqdn: record.fqdn, recordType: record.recordType, value, deviceId: record.targetDeviceId ?? '', port, expose: false };
  }

  protected override mapPublicMapping(mapping: ApiPublicMapping): PublicMappingRow {
    return { mappingId: mapping.mappingId, networkId: mapping.networkId, workspaceId: mapping.networkId, alias: mapping.alias, publicDomain: mapping.publicDomain, sourceRecord: mapping.sourceRecord, deviceId: mapping.deviceId, protocol: mapping.protocol, port: mapping.port, externalPort: mapping.externalPort, accessMode: 'public', tlsMode: 'off', status: mapping.status };
  }

  protected override mapSecurityGroup(group: ApiSecurityGroup): SecurityGroupRow {
    return { securityGroupId: group.securityGroupId, networkId: group.networkId, workspaceId: group.networkId, name: group.name, defaultPolicy: group.defaultPolicy, status: group.status };
  }

  protected override mapSecurityRule(rule: ApiSecurityRule): SecurityRuleRow {
    const port = rule.portFrom === 0 && rule.portTo === 0 ? 'all' : rule.portFrom === rule.portTo ? String(rule.portFrom) : `${rule.portFrom},${rule.portTo}`;
    return { ruleId: rule.ruleId, direction: rule.direction, priority: rule.priority, action: rule.action, protocol: rule.protocol, port, subjectType: rule.peerType, subjectValue: rule.peerValue };
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
