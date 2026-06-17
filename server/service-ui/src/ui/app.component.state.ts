import { AppApiClient } from './app-api.service';
import {
  DEFAULT_MAC_DEVICE_ID,
  DEFAULT_NETWORK_ID,
  INITIAL_DEVICE_EXPOSURES,
  INITIAL_DEVICE_GROUP_IDS,
  INITIAL_DEVICE_GROUPS,
  INITIAL_DEVICES,
  INITIAL_DNS_RECORDS,
  INITIAL_DNS_ZONES,
  DEVICE_GROUP_PRESETS,
  INITIAL_MEMBERS,
  INITIAL_PUBLIC_MAPPINGS,
  INITIAL_SECURITY_GROUPS,
  INITIAL_SECURITY_RULES,
  INITIAL_USER_ALIASES,
  INITIAL_WORKSPACE_DEVICE_INVITES,
  INITIAL_WORKSPACE_DEVICE_IDS,
  INITIAL_WORKSPACES,
  NAV_GROUPS,
  ROOT_DOMAIN,
  SECURITY_RULE_TEMPLATES,
  WORKSPACE_PRESETS,
} from './app.seed-data';
import {
  ApiDevice,
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
} from './app.models';
import { shortCodeFromEmail, slug } from './app.utils';

export abstract class AppComponentState {
  readonly rootDomain = ROOT_DOMAIN;
  readonly workspacePresets = WORKSPACE_PRESETS;
  readonly securityRuleTemplates: SecurityRuleTemplate[] = SECURITY_RULE_TEMPLATES;
  readonly navGroups = NAV_GROUPS;
  readonly publicMappingsEnabled = false;
  readonly downloadPlatforms = [
    { platform: 'macos', label: 'Mac', hint: 'macOS 13 及以上' },
    { platform: 'windows', label: 'Windows', hint: 'Windows 10/11 x64' },
    { platform: 'ios', label: 'iOS', hint: 'iPhone / iPad' },
    { platform: 'linux', label: 'Linux', hint: 'x64 / arm64' },
    { platform: 'android', label: 'Android', hint: 'Android 10 及以上' },
  ];

  constructor(protected readonly api: AppApiClient) {}

  protected async loadClientDownloads(): Promise<void> {}
  protected notifyStateChanged(): void {}

  mode: 'login' | 'register' | 'home' = 'login';
  active = 'overview';
  currentUser = '';
  currentUserId = '';
  currentUserShortCode = '';
  currentSessionToken = '';
  authMessage = '';
  deviceQuota: ApiDeviceQuota | null = null;

  authEmail = 'alice@staticlss.com';
  authPassword = '123456';
  authName = 'Alice';
  showPasswordDialog = false;
  oldPassword = '';
  newPassword = '';
  confirmPassword = '';
  passwordMessage = '';

  deviceId = DEFAULT_MAC_DEVICE_ID;
  deviceAlias = '办公 Mac';
  devicePlatform = 'macOS';
  deviceOSVersion = '15.3';
  devicePanel: 'list' | 'groups' = 'list';
  showDeviceGroupDialog = false;
  showDeviceGroupBindingDialog = false;
  deviceGroupDialogMode: 'create' | 'edit' = 'create';
  editingDeviceGroup: DeviceGroupRow | null = null;
  bindingDeviceGroup: DeviceGroupRow | null = null;
  deviceGroupName = '开发部';
  deviceGroupDescription = '';
  deviceGroupDialogMessage = '';
  deviceGroupBindingIds: string[] = [];
  deviceGroupBindingMessage = '';

  workspaceName = '默认网络';
  workspaceCode = 'default';
  workspaceIntraGroupPolicy: 'allow' | 'deny' = 'allow';
  workspaceDialogMessage = '';
  editingWorkspaceName = '';
  inviteEmail = 'bob@staticlss.com';
  domainName = 'api';
  ruleDirection = 'ingress';
  selectedWorkspaceId = DEFAULT_NETWORK_ID;
  workspacePanel: WorkspacePanel = 'zones';
  selectedZoneId = 'default';
  selectedSecurityGroupId = 'default';
  workspaceRouteMode: 'list' | 'detail' = 'list';
  showWorkspaceDialog = false;
  workspaceDialogMode: 'create' | 'edit' = 'create';
	  showInviteDialog = false;
	  workspaceInviteCode = '';
	  inviteQrDataUrl = '';
	  showBootstrapDialog = false;
	  bootstrapSessionKey = '';
	  bootstrapQrDataUrl = '';
	  bootstrapInstallCommand = '';
	  bootstrapMessage = '';
	  showJoinDialog = false;
  joinInviteCode = '';
  joinInviteMessage = '';
  showDeviceExposureDialog = false;
  selectedExposureDevice: DeviceRow | null = null;
  exposureUser = 'bob@staticlss.com';
  showWorkspaceDeviceDialog = false;
  showWorkspaceDeviceAliasDialog = false;
  showDeviceAliasDialog = false;
  showWorkspaceNameTagDialog = false;
  showWorkspacePolicyTagDialog = false;
  showSecurityGroupNameTagDialog = false;
  showUserAliasDialog = false;
  editingWorkspaceDevice: DeviceRow | null = null;
  editingDevice: DeviceRow | null = null;
  editingWorkspace: WorkspaceRow | null = null;
  editingSecurityGroup: SecurityGroupRow | null = null;
  editingUserAlias: UserAliasRow | null = null;
  bindDeviceQuery = '';
  selectedWorkspaceDeviceId = '';
  workspaceDeviceAliasValue = '';
  deviceAliasValue = '';
  workspaceNameValue = '';
  workspaceIntraGroupPolicyValue: 'allow' | 'deny' = 'allow';
  securityGroupNameValue = '';
  userAliasValue = '';
  workspaceDeviceId = 'android-001';
  workspaceDeviceOwner = 'alice@staticlss.com';
  workspaceDeviceAlias = 'Android 测试机';
  workspaceDeviceIp = '10.0.0.3';
  workspaceDevicePlatform = 'Android';
  workspaceDeviceOSVersion = '15';
  workspaceDeviceStatus = 'active';
  showZoneDialog = false;
  showZoneTagDialog = false;
  zoneDialogMode: 'create' | 'edit' = 'create';
  editingZone: DNSZoneRow | null = null;
  zoneName = 'default';
  zoneNameValue = '';
  zoneRecordType = 'A';
  zoneValue = '10.0.0.1';
  showRecordDialog = false;
  recordDialogMode: 'create' | 'edit' = 'create';
  editingRecord: DNSRow | null = null;
  recordName = 'api';
  recordType = 'A';
  recordValue = '10.0.0.1';
  recordDeviceId = DEFAULT_MAC_DEVICE_ID;
  recordPort = '443';
  showPublicMappingDialog = false;
  publicMappingDialogMode: 'create' | 'edit' = 'create';
  editingPublicMapping: PublicMappingRow | null = null;
  publicAlias = 'api';
  publicSourceRecord = 'api';
  publicProtocol = 'HTTP';
  publicExternalPort = '443';
  publicAccessMode = 'public';
  publicTlsMode = 'auto';
  showIngressRuleDialog = false;
  showEgressRuleDialog = false;
  ruleDialogMode: 'create' | 'edit' = 'create';
  editingRule: SecurityRuleRow | null = null;
  rulePriority = 100;
  ruleAction = 'allow';
  ruleProtocol = 'tcp';
  rulePort = '443';
  ruleSubjectType: RuleSubjectType = 'device';
  ruleSubjectValue = DEFAULT_MAC_DEVICE_ID;
  selectedRuleTemplate = 'Web 服务';
  devices: DeviceRow[] = INITIAL_DEVICES.map((item) => ({ ...item }));
  deviceGroups: DeviceGroupRow[] = INITIAL_DEVICE_GROUPS.map((item) => ({ ...item }));
  deviceGroupIdsByDevice: Record<string, string[]> = Object.fromEntries(
    Object.entries(INITIAL_DEVICE_GROUP_IDS).map(([deviceId, groupId]) => [deviceId, [groupId]]),
  );
  workspaceDeviceIdsByWorkspace: Record<string, string[]> = Object.fromEntries(
    Object.entries(INITIAL_WORKSPACE_DEVICE_IDS).map(([workspaceId, deviceIds]) => [workspaceId, [...deviceIds]]),
  );
  workspaceDeviceJoinMethods: Record<string, string> = {
    [`${DEFAULT_NETWORK_ID}|${DEFAULT_MAC_DEVICE_ID}`]: '手动添加',
  };
  workspaces: WorkspaceRow[] = INITIAL_WORKSPACES.map((item) => ({ ...item }));
  members: MemberRow[] = INITIAL_MEMBERS.map((item) => ({ ...item }));
  userAliases: UserAliasRow[] = INITIAL_USER_ALIASES.map((item) => ({ ...item }));
  dnsZones: DNSZoneRow[] = INITIAL_DNS_ZONES.map((item) => ({ ...item }));
  dnsRecords: DNSRow[] = INITIAL_DNS_RECORDS.map((item) => ({ ...item }));
  publicMappings: PublicMappingRow[] = INITIAL_PUBLIC_MAPPINGS.map((item) => ({ ...item }));
  securityRules: SecurityRuleRow[] = INITIAL_SECURITY_RULES.map((item) => ({ ...item }));
  securityGroups: SecurityGroupRow[] = INITIAL_SECURITY_GROUPS.map((item) => ({ ...item }));
  deviceExposures: DeviceExposureRow[] = INITIAL_DEVICE_EXPOSURES.map((item) => ({ ...item }));
  workspaceDeviceInvites: WorkspaceDeviceInviteRow[] = INITIAL_WORKSPACE_DEVICE_INVITES.map((item) => ({ ...item }));
  clientDownloads: ClientDownload[] = [];

  get activeNav(): NavItem {
    const activeId = this.active === 'devices' && this.devicePanel === 'groups' ? 'deviceGroups' : this.active;
    return this.allNavItems().find((item) => item.id === activeId) ?? this.navGroups[0].items[0];
  }

  allNavItems(): NavItem[] {
    return this.navGroups.flatMap((group) => group.items.flatMap((item) => [item, ...(item.children ?? [])]));
  }

  isNavItemActive(item: NavItem): boolean {
    if (item.id === 'deviceGroups') {
      return this.active === 'devices' && this.devicePanel === 'groups';
    }
    if (item.id === 'devices') {
      return this.active === 'devices' && this.devicePanel === 'list';
    }
    return this.active === item.id;
  }

  isNavBranchActive(item: NavItem): boolean {
    return !!item.children?.some((child) => this.isNavItemActive(child));
  }

  get ingressRules(): SecurityRuleRow[] {
    return this.securityRules.filter((rule) => rule.securityGroupId === this.selectedSecurityGroupId && rule.direction === 'ingress');
  }

  get egressRules(): SecurityRuleRow[] {
    return this.securityRules.filter((rule) => rule.securityGroupId === this.selectedSecurityGroupId && rule.direction === 'egress');
  }

  get ingressRuleTemplates(): SecurityRuleTemplate[] {
    return this.securityRuleTemplates.filter((template) => template.direction === 'ingress');
  }

  get egressRuleTemplates(): SecurityRuleTemplate[] {
    return this.securityRuleTemplates.filter((template) => template.direction === 'egress');
  }

  get ingressSubjectTypes(): Array<{ value: RuleSubjectType; label: string }> {
    return [
      { value: 'device', label: '设备' },
      { value: 'user', label: '用户' },
      { value: 'workspace', label: '网络' },
      { value: 'cidr', label: 'CIDR' },
      { value: 'all', label: '全部' },
    ];
  }

  get egressSubjectTypes(): Array<{ value: RuleSubjectType; label: string }> {
    return [
      { value: 'device', label: '设备' },
      { value: 'workspace', label: '网络' },
      { value: 'cidr', label: 'CIDR' },
      { value: 'domain', label: '域名' },
      { value: 'all', label: '全部' },
    ];
  }

  get ruleSubjectOptions(): Array<{ value: string; label: string }> {
    switch (this.ruleSubjectType) {
      case 'device':
        return this.visibleDeviceOptions.map((device) => ({ value: device.deviceId, label: `${this.userLabel(device.owner)} / ${device.alias} / ${device.deviceId}` }));
      case 'user':
        return this.members.map((member) => ({ value: member.user, label: `${member.alias} / ${this.userLabel(member.user)}` }));
      case 'workspace':
        return [
          { value: 'self', label: `${this.selectedWorkspace.name} / 当前网络` },
          ...this.workspaces.filter((workspace) => workspace.workspaceId !== this.selectedWorkspace.workspaceId).map((workspace) => ({ value: workspace.workspaceId, label: `${workspace.name} / ${workspace.code}` })),
        ];
      case 'all':
        return [{ value: 'all', label: '全部' }];
      default:
        return [];
    }
  }

  get workspaceDevices(): DeviceRow[] {
    const currentIds = this.currentWorkspaceDeviceIds();
    return this.devices.filter((device) => currentIds.includes(device.deviceId));
  }

  get currentUserDevices(): DeviceRow[] {
    return this.devices;
  }

  get currentDeviceGroups(): DeviceGroupRow[] {
    return [...this.deviceGroups].sort((a, b) => a.name.localeCompare(b.name));
  }

  get availableDeviceGroupPresets(): Pick<DeviceGroupRow, 'name' | 'description'>[] {
    const existingNames = new Set(this.deviceGroups.map((group) => group.name.trim()));
    return DEVICE_GROUP_PRESETS.filter((preset) => !existingNames.has(preset.name));
  }

  deviceGroupNameForDevice(device: DeviceRow): string {
    const names = this.deviceGroupNamesForDevice(device);
    return names.length ? names.join('、') : '未分组';
  }

  deviceGroupNamesForDevice(device: DeviceRow): string[] {
    const groupIds = this.deviceGroupIdsByDevice[device.deviceId] ?? [];
    return groupIds
      .map((groupId) => this.deviceGroups.find((group) => group.groupId === groupId)?.name ?? '')
      .filter(Boolean);
  }

  deviceGroupCount(groupId: string): number {
    return this.currentUserDevices.filter((device) => (this.deviceGroupIdsByDevice[device.deviceId] ?? []).includes(groupId)).length;
  }

  ungroupedDeviceCount(): number {
    return this.currentUserDevices.filter((device) => (this.deviceGroupIdsByDevice[device.deviceId] ?? []).length === 0).length;
  }

  devicesInGroup(groupId: string): DeviceRow[] {
    return this.currentUserDevices.filter((device) => (this.deviceGroupIdsByDevice[device.deviceId] ?? []).includes(groupId));
  }

  deviceNamesInGroup(groupId: string): string {
    const names = this.devicesInGroup(groupId).map((device) => device.alias || device.deviceId);
    return names.length ? names.join('、') : '-';
  }

  deviceInGroup(device: DeviceRow, groupId: string): boolean {
    return (this.deviceGroupIdsByDevice[device.deviceId] ?? []).includes(groupId);
  }

  latestClientDownload(platform: string): ClientDownload | null {
    return this.clientDownloads.find((item) => item.platform === platform && item.status === 'active') ?? null;
  }

  formatDownloadSize(bytes: number): string {
    if (!bytes) {
      return '-';
    }
    if (bytes >= 1024 * 1024 * 1024) {
      return `${(bytes / 1024 / 1024 / 1024).toFixed(2)} GB`;
    }
    if (bytes >= 1024 * 1024) {
      return `${(bytes / 1024 / 1024).toFixed(1)} MB`;
    }
    if (bytes >= 1024) {
      return `${(bytes / 1024).toFixed(1)} KB`;
    }
    return `${bytes} B`;
  }

  get currentPlanName(): string {
    return this.deviceQuota?.planName ?? '免费版';
  }

  get currentDeviceTotalLimit(): number {
    return this.deviceQuota?.totalDeviceLimit ?? 10;
  }

  get currentDeviceRemaining(): number {
    if (this.deviceQuota) {
      return this.deviceQuota.remainingDevices;
    }
    return Math.max(this.currentDeviceTotalLimit - this.currentUserDevices.length, 0);
  }

  get canCreateDeviceInvite(): boolean {
    return this.currentDeviceRemaining > 0;
  }

  get joinDeviceOptions(): DeviceRow[] {
    return this.currentUserDevices
      .filter((device) => this.isCurrentUserDeviceOwner(device))
      .sort((a, b) => a.deviceId.localeCompare(b.deviceId));
  }

  get currentDNSZones(): DNSZoneRow[] {
    return this.dnsZones.filter((zone) => zone.workspaceId === this.selectedWorkspaceId);
  }

  get currentDNSRecords(): DNSRow[] {
    return this.dnsRecords.filter((record) => record.workspaceId === this.selectedWorkspaceId);
  }

  get currentPublicMappings(): PublicMappingRow[] {
    return this.publicMappings.filter((mapping) => mapping.workspaceId === this.selectedWorkspaceId);
  }

  get currentSecurityGroups(): SecurityGroupRow[] {
    return this.securityGroups.filter((group) => group.workspaceId === this.selectedWorkspaceId);
  }

  get recordDeviceOptions(): DeviceRow[] {
    const byId = new Map<string, DeviceRow>();
    this.visibleDeviceOptions.forEach((device) => byId.set(device.deviceId, device));
    if (this.recordDeviceId) {
      const selected = this.devices.find((device) => device.deviceId === this.recordDeviceId);
      if (selected) {
        byId.set(selected.deviceId, selected);
      }
    }
    return Array.from(byId.values()).sort((a, b) => a.deviceId.localeCompare(b.deviceId));
  }

  get visibleDeviceOptions(): DeviceRow[] {
    const byId = new Map<string, DeviceRow>();
    [...this.workspaceDevices, ...this.currentUserDevices].forEach((device) => byId.set(device.deviceId, device));
    if (this.ruleSubjectType === 'device' && this.ruleSubjectValue) {
      const selected = this.devices.find((device) => device.deviceId === this.ruleSubjectValue);
      if (selected) {
        byId.set(selected.deviceId, selected);
      }
    }
    return Array.from(byId.values()).sort((a, b) => a.deviceId.localeCompare(b.deviceId));
  }

  get currentWorkspaceDeviceInvites(): WorkspaceDeviceInviteRow[] {
    return this.workspaceDeviceInvites
      .filter((invite) => invite.workspaceId === this.selectedWorkspaceId)
      .sort((a, b) => b.createdAt - a.createdAt);
  }

  get managedUserAliases(): UserAliasRow[] {
    const emails = new Set<string>();
    const visibleDeviceIds = new Set<string>();
    for (const deviceIds of Object.values(this.workspaceDeviceIdsByWorkspace)) {
      deviceIds.forEach((deviceId) => visibleDeviceIds.add(deviceId));
    }
    this.devices
      .filter((device) => this.currentOwnerKeys().has(device.owner.trim().toLowerCase()) || visibleDeviceIds.has(device.deviceId))
      .forEach((device) => emails.add(device.owner));
    this.members.forEach((member) => emails.add(member.user));
    this.deviceExposures
      .filter((item) => this.currentUserDevices.some((device) => device.deviceId === item.deviceId))
      .forEach((item) => emails.add(item.user));
    const current = (this.currentUser || this.authEmail).trim().toLowerCase();
    return Array.from(emails)
      .filter((email) => email.trim().toLowerCase() !== current)
      .sort()
      .map((email) => this.userAliases.find((item) => item.email === email) ?? { email, alias: '' });
  }

  get selectedDeviceUsages(): Array<{ workspaceName: string; workspaceCode: string; owner: string; joinMethod: string }> {
    if (!this.selectedExposureDevice) {
      return [];
    }
    return this.workspaces
      .filter((workspace) => (this.workspaceDeviceIdsByWorkspace[workspace.workspaceId] ?? []).includes(this.selectedExposureDevice?.deviceId ?? ''))
      .map((workspace) => ({
        workspaceName: workspace.name,
        workspaceCode: workspace.code,
        owner: this.workspaceDeviceOwnerLabel(this.selectedExposureDevice as DeviceRow),
        joinMethod: this.workspaceDeviceJoinMethod(workspace.workspaceId, (this.selectedExposureDevice as DeviceRow).deviceId),
      }));
  }

  workspaceDeviceOwnerLabel(device: DeviceRow): string {
    return this.userLabel(device.owner);
  }

  isCurrentUserDeviceOwner(device: DeviceRow): boolean {
    return this.currentOwnerKeys().has(device.owner.trim().toLowerCase());
  }

  userLabel(value: string): string {
    const current = (this.currentUser || this.authEmail).trim().toLowerCase();
    if (value.trim().toLowerCase() === current) {
      return 'owner';
    }
    return this.userAliases.find((item) => item.email.toLowerCase() === value.trim().toLowerCase())?.alias || value;
  }

  displayUserText(value: string): string {
    const current = (this.currentUser || this.authEmail).trim();
    let output = current ? value.replaceAll(current, 'owner') : value;
    for (const item of this.userAliases) {
      if (item.alias.trim()) {
        output = output.replaceAll(item.email, item.alias.trim());
      }
    }
    return output;
  }

  workspaceDeviceJoinMethod(workspaceId: string, deviceId: string): string {
    return this.workspaceDeviceJoinMethods[`${workspaceId}|${deviceId}`] ?? '手动添加';
  }

  get bindableWorkspaceDevices(): DeviceRow[] {
    const currentIds = this.currentWorkspaceDeviceIds();
    return this.currentUserDevices.filter((device) => !currentIds.includes(device.deviceId));
  }

  get queriedBindableWorkspaceDevices(): DeviceRow[] {
    const query = this.bindDeviceQuery.trim().toLowerCase();
    if (!query) {
      return this.bindableWorkspaceDevices;
    }
    return this.bindableWorkspaceDevices.filter((device) => [
      device.deviceId,
      device.owner,
      device.alias,
      device.ip,
      device.platform,
      device.osVersion,
    ].some((value) => value.toLowerCase().includes(query)));
  }

  get selectedWorkspace(): WorkspaceRow {
    return this.workspaces.find((workspace) => workspace.workspaceId === this.selectedWorkspaceId) ?? this.workspaces[0];
  }

  get invitedWorkspace(): WorkspaceRow | null {
    const code = this.joinInviteCode.split('-')[0]?.trim().toLowerCase();
    if (!code) {
      return null;
    }
    return this.workspaces.find((workspace) => workspace.code.toLowerCase() === code) ?? null;
  }

  get selectedJoinInvite(): WorkspaceDeviceInviteRow | null {
    const code = this.joinInviteCode.trim().toUpperCase();
    if (!code) {
      return null;
    }
    return this.workspaceDeviceInvites.find((invite) => invite.inviteCode.toUpperCase() === code) ?? null;
  }

  get canJoinInvite(): boolean {
    if (!this.joinInviteCode.trim() || !this.deviceId) {
      return false;
    }
    const invite = this.selectedJoinInvite;
    return !invite || this.effectiveInviteStatus(invite) === 'pending';
  }

  get joinInviteValidationMessage(): string {
    if (!this.joinInviteCode || !this.deviceId) {
      return this.joinInviteMessage;
    }
    const invite = this.selectedJoinInvite;
    if (invite && this.effectiveInviteStatus(invite) !== 'pending') {
      return '该邀请码已失效，不能继续接入。';
    }
    return this.joinInviteMessage;
  }

  currentWorkspaceDeviceIds(): string[] {
    return this.workspaceDeviceIdsByWorkspace[this.selectedWorkspaceId] ?? [];
  }

  currentOwnerKeys(): Set<string> {
    return new Set([this.currentUser, this.authEmail, this.currentUserId].filter(Boolean).map((value) => value.trim().toLowerCase()));
  }

  get userSlug(): string {
    return this.currentUserShortCode || shortCodeFromEmail(this.currentUser || this.authEmail);
  }

  get userShortSubdomain(): string {
    return `${this.userSlug}.slan.com`;
  }

  get publicDomainPreview(): string {
    return `${slug(this.publicAlias)}.${this.selectedWorkspace.code}.${this.userSlug}.pub.staticlss.com`;
  }

  get selectedDeviceExposures(): DeviceExposureRow[] {
    if (!this.selectedExposureDevice) {
      return [];
    }
    return this.deviceExposures.filter((item) => item.deviceId === this.selectedExposureDevice?.deviceId);
  }

  get inviteQrCells(): boolean[] {
    const source = this.workspaceInviteCode || 'SLAN';
    return Array.from({ length: 49 }, (_, index) => {
      const charCode = source.charCodeAt(index % source.length);
      return ((charCode + index * 7) % 5) < 2 || [0, 1, 7, 8, 40, 41, 47, 48].includes(index);
    });
  }

  protected abstract applyRouteFromLocation(): void;
  protected abstract loadDashboard(userId?: string): Promise<void>;
  protected abstract loadDeviceGroups(userId: string): Promise<void>;
  protected abstract loadWorkspaceDevices(workspaceId: string): Promise<void>;
  protected abstract loadWorkspaceResources(workspaceId: string): Promise<void>;
  protected abstract loadSecurityRules(securityGroupId: string): Promise<void>;
  protected abstract upsertWorkspaceDeviceInvite(invite: WorkspaceDeviceInviteRow): void;
  protected abstract markInviteAccepted(inviteCode: string, workspaceId: string, deviceId: string): void;
  protected abstract mapDevice(device: ApiDevice): DeviceRow;
  protected abstract mapDNSZone(zone: ApiDNSZone): DNSZoneRow;
  protected abstract mapDNSRecord(record: ApiDNSRecord): DNSRow;
  protected abstract mapPublicMapping(mapping: ApiPublicMapping): PublicMappingRow;
  protected abstract mapSecurityGroup(group: ApiSecurityGroup): SecurityGroupRow;
  protected abstract mapSecurityRule(rule: ApiSecurityRule): SecurityRuleRow;
  abstract isWorkspaceCodeDuplicated(code: string, exceptWorkspaceId?: string): boolean;
  abstract effectiveInviteStatus(invite: WorkspaceDeviceInviteRow): string;
}
