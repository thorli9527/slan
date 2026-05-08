import { CommonModule } from '@angular/common';
import { Component, OnInit } from '@angular/core';
import { FormsModule } from '@angular/forms';
import QRCode from 'qrcode';
import { AppApiClient } from './app-api.service';
import { panelFromRoute, workspacePanelPath } from './app-routing';
import {
  INITIAL_DEVICE_EXPOSURES,
  INITIAL_DEVICES,
  INITIAL_DNS_RECORDS,
  INITIAL_DNS_ZONES,
  INITIAL_MEMBERS,
  INITIAL_PUBLIC_MAPPINGS,
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
  ApiDNSRecord,
  ApiDNSZone,
  ApiPublicMapping,
  ApiSecurityGroup,
  ApiSecurityRule,
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
  SecurityRuleRow,
  SecurityGroupRow,
  SecurityRuleTemplate,
  UserAliasRow,
  WorkspaceDeviceInviteRow,
  WorkspacePanel,
  WorkspaceRow,
} from './app.models';
import { shortCodeFromEmail, slug } from './app.utils';

@Component({
  selector: 'app-root',
  standalone: true,
  imports: [CommonModule, FormsModule],
  templateUrl: './app.component.html',
  styleUrl: './app.component.css',
})
export class AppComponent implements OnInit {
  readonly rootDomain = ROOT_DOMAIN;
  readonly workspacePresets = WORKSPACE_PRESETS;
  readonly securityRuleTemplates: SecurityRuleTemplate[] = SECURITY_RULE_TEMPLATES;
  readonly navGroups = NAV_GROUPS;

  constructor(private readonly api: AppApiClient) {}

  mode: 'login' | 'register' | 'home' = 'login';
  active = 'overview';
  currentUser = '';
  currentUserId = '';
  currentUserShortCode = '';

  authEmail = 'alice@vlan.com';
  authPassword = '123456';
  authName = 'Alice';
  showPasswordDialog = false;
  oldPassword = '';
  newPassword = '';
  confirmPassword = '';
  passwordMessage = '';

  deviceId = 'mac-001';
  deviceAlias = '办公 Mac';
  devicePlatform = 'macOS';
  deviceOSVersion = '15.3';

  workspaceName = '默认网络';
  workspaceCode = 'default';
  workspaceDialogMessage = '';
  editingWorkspaceName = '';
  inviteEmail = 'bob@vlan.com';
  domainName = 'api';
  ruleDirection = 'ingress';
  selectedWorkspaceId = 'default-user-000001';
  workspacePanel: WorkspacePanel = 'zones';
  selectedZoneId = 'default';
  selectedSecurityGroupId = 'default';
  workspaceRouteMode: 'list' | 'detail' = 'list';
  showWorkspaceDialog = false;
  workspaceDialogMode: 'create' | 'edit' = 'create';
  showInviteDialog = false;
  workspaceInviteCode = '';
  inviteQrDataUrl = '';
  showJoinDialog = false;
  joinInviteCode = '';
  joinInviteMessage = '';
  showDeviceExposureDialog = false;
  selectedExposureDevice: DeviceRow | null = null;
  exposureUser = 'bob@vlan.com';
  showWorkspaceDeviceDialog = false;
  showWorkspaceDeviceAliasDialog = false;
  showDeviceAliasDialog = false;
  showWorkspaceNameTagDialog = false;
  showWorkspaceCodeTagDialog = false;
  showUserAliasDialog = false;
  editingWorkspaceDevice: DeviceRow | null = null;
  editingDevice: DeviceRow | null = null;
  editingWorkspace: WorkspaceRow | null = null;
  editingUserAlias: UserAliasRow | null = null;
  bindDeviceQuery = '';
  selectedWorkspaceDeviceId = '';
  workspaceDeviceAliasValue = '';
  deviceAliasValue = '';
  workspaceNameValue = '';
  workspaceCodeValue = '';
  userAliasValue = '';
  workspaceDeviceId = 'android-001';
  workspaceDeviceOwner = 'alice@vlan.com';
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
  recordDeviceId = 'mac-001';
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
  ruleSubjectValue = 'mac-001';
  selectedRuleTemplate = 'Web 服务';

  devices: DeviceRow[] = INITIAL_DEVICES.map((item) => ({ ...item }));
  workspaceDeviceIdsByWorkspace: Record<string, string[]> = Object.fromEntries(
    Object.entries(INITIAL_WORKSPACE_DEVICE_IDS).map(([workspaceId, deviceIds]) => [workspaceId, [...deviceIds]]),
  );
  workspaceDeviceJoinMethods: Record<string, string> = {
    'default-user-000001|mac-001': '手动添加',
    'default-user-000001|iphone-001': '手动添加',
    'workspace-000001|mac-001': '手动添加',
  };
  workspaces: WorkspaceRow[] = INITIAL_WORKSPACES.map((item) => ({ ...item }));
  members: MemberRow[] = INITIAL_MEMBERS.map((item) => ({ ...item }));
  userAliases: UserAliasRow[] = INITIAL_USER_ALIASES.map((item) => ({ ...item }));
  dnsZones: DNSZoneRow[] = INITIAL_DNS_ZONES.map((item) => ({ ...item }));
  dnsRecords: DNSRow[] = INITIAL_DNS_RECORDS.map((item) => ({ ...item }));
  publicMappings: PublicMappingRow[] = INITIAL_PUBLIC_MAPPINGS.map((item) => ({ ...item }));
  securityRules: SecurityRuleRow[] = INITIAL_SECURITY_RULES.map((item) => ({ ...item }));
  securityGroups: SecurityGroupRow[] = [];
  deviceExposures: DeviceExposureRow[] = INITIAL_DEVICE_EXPOSURES.map((item) => ({ ...item }));
  workspaceDeviceInvites: WorkspaceDeviceInviteRow[] = INITIAL_WORKSPACE_DEVICE_INVITES.map((item) => ({ ...item }));

  get activeNav(): NavItem {
    return this.navGroups.flatMap((group) => group.items).find((item) => item.id === this.active) ?? this.navGroups[0].items[0];
  }

  get ingressRules(): SecurityRuleRow[] {
    return this.securityRules.filter((rule) => rule.direction === 'ingress');
  }

  get egressRules(): SecurityRuleRow[] {
    return this.securityRules.filter((rule) => rule.direction === 'egress');
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
    return `${slug(this.publicAlias)}.${this.selectedWorkspace.code}.${this.userSlug}.pub.slan.com`;
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

  ngOnInit(): void {
    this.applyRouteFromLocation();
    window.addEventListener('popstate', () => this.applyRouteFromLocation());
  }

  async enter(mode: 'login' | 'register'): Promise<void> {
    if (!this.authEmail || !this.authPassword) {
      return;
    }
    try {
      const path = mode === 'login' ? '/api/auth/login' : '/api/auth/register';
      const payload = mode === 'login'
        ? { email: this.authEmail, password: this.authPassword }
        : { email: this.authEmail, password: this.authPassword, name: this.authName };
      const response = await this.api.post<{ auth: { user: { userId: string; email: string } } }>(path, payload);
      this.currentUser = response.auth.user.email;
      this.currentUserId = response.auth.user.userId;
      this.currentUserShortCode = shortCodeFromEmail(response.auth.user.email);
      this.mode = 'home';
      await this.loadDashboard(response.auth.user.userId);
      this.applyRouteFromLocation();
    } catch {
      this.currentUser = this.authEmail;
      this.currentUserShortCode = shortCodeFromEmail(this.authEmail);
      this.mode = 'home';
      if (mode === 'register' && !this.devices.some((device) => device.owner === this.authEmail)) {
        this.devices = [
          ...this.devices,
          { deviceId: 'new-device', platform: 'macOS', osVersion: '15.x', alias: '新设备', ip: '10.0.0.20', owner: this.authEmail, status: 'active' },
        ];
      }
    }
  }

  logout(): void {
    this.mode = 'login';
  }

  openPasswordDialog(): void {
    this.oldPassword = '';
    this.newPassword = '';
    this.confirmPassword = '';
    this.passwordMessage = '';
    this.showPasswordDialog = true;
  }

  closePasswordDialog(): void {
    this.showPasswordDialog = false;
  }

  async changePassword(): Promise<void> {
    this.passwordMessage = '';
    if (!this.oldPassword || !this.newPassword || !this.confirmPassword) {
      this.passwordMessage = '请填写完整密码信息';
      return;
    }
    if (this.newPassword !== this.confirmPassword) {
      this.passwordMessage = '两次输入的新密码不一致';
      return;
    }
    if (this.newPassword.length < 6) {
      this.passwordMessage = '新密码至少 6 位';
      return;
    }
    try {
      await this.api.patch(`/api/users/${encodeURIComponent(this.currentUserId || 'user-000001')}/password`, {
        oldPassword: this.oldPassword,
        newPassword: this.newPassword,
      });
    } catch {
      // Preview mode without API: keep the interaction complete locally.
    }
    this.authPassword = this.newPassword;
    this.closePasswordDialog();
  }

  setActive(id: string): void {
    this.active = id;
    if (id === 'overview') {
      this.workspaceRouteMode = 'list';
      history.pushState({}, '', '/overview');
      return;
    }
    if (id === 'devices') {
      this.workspaceRouteMode = 'list';
      history.pushState({}, '', '/devices');
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

  selectWorkspace(workspace: WorkspaceRow): void {
    this.selectedWorkspaceId = workspace.workspaceId;
    this.editingWorkspaceName = workspace.name;
    this.workspacePanel = 'zones';
  }

  openWorkspaceDetail(workspace: WorkspaceRow, panel: WorkspacePanel = 'zones'): void {
    this.selectedWorkspaceId = workspace.workspaceId;
    this.editingWorkspaceName = workspace.name;
    this.workspacePanel = panel;
    this.workspaceRouteMode = 'detail';
    this.active = 'workspaces';
    history.pushState({}, '', workspacePanelPath(workspace.workspaceId, panel, this.selectedZoneId, this.selectedSecurityGroupId));
    void this.loadWorkspaceDevices(workspace.workspaceId);
  }

  backToWorkspaceList(): void {
    this.workspaceRouteMode = 'list';
    this.active = 'workspaces';
    history.pushState({}, '', '/spaces');
  }

  openWorkspacePanel(workspace: WorkspaceRow, panel: WorkspacePanel): void {
    this.openWorkspaceDetail(workspace, panel);
  }

  openZoneRecords(zone?: DNSZoneRow): void {
    if (zone) {
      this.selectedZoneId = zone.zone;
    }
    this.workspacePanel = 'records';
    history.pushState({}, '', workspacePanelPath(this.selectedWorkspaceId, 'records', this.selectedZoneId, this.selectedSecurityGroupId));
  }

  openSecurityRules(): void {
    this.selectedSecurityGroupId = this.currentSecurityGroups[0]?.securityGroupId ?? this.selectedSecurityGroupId;
    this.workspacePanel = 'securityRules';
    history.pushState({}, '', workspacePanelPath(this.selectedWorkspaceId, 'securityRules', this.selectedZoneId, this.selectedSecurityGroupId));
  }

  openSecurityGroupRules(group: SecurityGroupRow): void {
    this.selectedSecurityGroupId = group.securityGroupId;
    this.workspacePanel = 'securityRules';
    history.pushState({}, '', workspacePanelPath(this.selectedWorkspaceId, 'securityRules', this.selectedZoneId, this.selectedSecurityGroupId));
    void this.loadSecurityRules(group.securityGroupId);
  }

  setWorkspacePanel(panel: WorkspacePanel): void {
    this.workspacePanel = panel;
    history.pushState({}, '', workspacePanelPath(this.selectedWorkspaceId, panel, this.selectedZoneId, this.selectedSecurityGroupId));
    void this.loadWorkspaceResources(this.selectedWorkspaceId);
  }

  selectWorkspaceForDeviceManagement(workspaceId: string): void {
    this.selectedWorkspaceId = workspaceId;
    this.workspacePanel = 'zones';
    void this.loadWorkspaceDevices(workspaceId);
  }

  onWorkspaceNameChanged(): void {
    const preset = this.workspacePresets.find((item) => item.name === this.workspaceName.trim());
    if (preset) {
      this.workspaceCode = preset.code;
      return;
    }
    this.workspaceCode = slug(this.workspaceName);
  }

  openWorkspaceDialog(): void {
    this.workspaceName = '默认网络';
    this.workspaceCode = 'default';
    this.workspaceDialogMessage = '';
    this.workspaceDialogMode = 'create';
    this.showWorkspaceDialog = true;
  }

  openEditWorkspaceDialog(workspace: WorkspaceRow): void {
    this.selectedWorkspaceId = workspace.workspaceId;
    this.workspaceName = workspace.name;
    this.workspaceCode = workspace.code;
    this.workspaceDialogMessage = '';
    this.workspaceDialogMode = 'edit';
    this.showWorkspaceDialog = true;
  }

  closeWorkspaceDialog(): void {
    this.showWorkspaceDialog = false;
  }

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

  async addWorkspace(): Promise<void> {
    this.workspaceDialogMessage = '';
    if (this.workspaceDialogMode === 'edit') {
      this.saveWorkspaceDialog();
      return;
    }
    const code = slug(this.workspaceCode || this.workspaceName);
    if (this.isWorkspaceCodeDuplicated(code)) {
      this.workspaceDialogMessage = '当前用户下网络编码不能重复';
      return;
    }
    try {
      await this.api.post('/api/workspaces', {
        ownerUserId: this.currentUserId || 'user-000001',
        name: this.workspaceName,
        code,
        templateKey: code,
      });
      await this.loadDashboard(this.currentUserId);
      this.closeWorkspaceDialog();
      return;
    } catch {
      // Keep local template behavior available for UI review.
    }
    const id = `workspace-${String(this.workspaces.length + 1).padStart(6, '0')}`;
    this.workspaces = [
      ...this.workspaces,
      { workspaceId: id, name: this.workspaceName, code, template: code || 'custom', status: 'enabled', members: 1, devices: 0, zone: `${code}.${id}.user-000001.sub.slan.com` },
    ];
    this.closeWorkspaceDialog();
  }

  saveWorkspaceDialog(): void {
    const workspace = this.selectedWorkspace;
    if (!workspace || !this.workspaceName.trim()) {
      return;
    }
    const code = slug(this.workspaceCode || this.workspaceName);
    if (this.isWorkspaceCodeDuplicated(code, workspace.workspaceId)) {
      this.workspaceDialogMessage = '当前用户下网络编码不能重复';
      return;
    }
    workspace.name = this.workspaceName.trim();
    workspace.code = code;
    workspace.template = workspace.code;
    workspace.zone = `${slug(workspace.code)}.${workspace.workspaceId}.user-000001.sub.slan.com`;
    this.closeWorkspaceDialog();
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

  saveDeviceAliasDialog(): void {
    if (!this.editingDevice || !this.deviceAliasValue.trim()) {
      return;
    }
    const alias = this.deviceAliasValue.trim();
    const deviceID = this.editingDevice.deviceId;
    this.devices = this.devices.map((item) => item.deviceId === deviceID ? { ...item, alias } : item);
    this.closeDeviceAliasDialog();
  }

  openUserAliasDialog(user: UserAliasRow): void {
    if (this.showUserAliasDialog && this.editingUserAlias?.email === user.email) {
      this.closeUserAliasDialog();
      return;
    }
    this.editingUserAlias = user;
    this.userAliasValue = user.alias || user.email.split('@')[0] || '';
    this.showUserAliasDialog = true;
  }

  closeUserAliasDialog(): void {
    this.showUserAliasDialog = false;
    this.editingUserAlias = null;
  }

  saveUserAliasDialog(): void {
    if (!this.editingUserAlias || !this.userAliasValue.trim()) {
      return;
    }
    const email = this.editingUserAlias.email;
    const alias = this.userAliasValue.trim();
    const existing = this.userAliases.find((item) => item.email === email);
    if (existing) {
      existing.alias = alias;
    } else {
      this.userAliases = [...this.userAliases, { email, alias }];
    }
    this.closeUserAliasDialog();
  }

  openWorkspaceNameTagDialog(workspace: WorkspaceRow): void {
    if (this.showWorkspaceNameTagDialog && this.editingWorkspace?.workspaceId === workspace.workspaceId) {
      this.closeWorkspaceTagDialogs();
      return;
    }
    this.editingWorkspace = workspace;
    this.workspaceNameValue = workspace.name;
    this.workspaceCodeValue = workspace.code;
    this.showWorkspaceNameTagDialog = true;
    this.showWorkspaceCodeTagDialog = false;
  }

  openWorkspaceCodeTagDialog(workspace: WorkspaceRow): void {
    if (this.showWorkspaceCodeTagDialog && this.editingWorkspace?.workspaceId === workspace.workspaceId) {
      this.closeWorkspaceTagDialogs();
      return;
    }
    this.editingWorkspace = workspace;
    this.workspaceNameValue = workspace.name;
    this.workspaceCodeValue = workspace.code;
    this.showWorkspaceCodeTagDialog = true;
    this.showWorkspaceNameTagDialog = false;
  }

  closeWorkspaceTagDialogs(): void {
    this.showWorkspaceNameTagDialog = false;
    this.showWorkspaceCodeTagDialog = false;
    this.editingWorkspace = null;
  }

  async saveWorkspaceTagDialog(): Promise<void> {
    if (!this.editingWorkspace || !this.workspaceNameValue.trim() || !this.workspaceCodeValue.trim()) {
      return;
    }
    const code = slug(this.workspaceCodeValue);
    if (this.isWorkspaceCodeDuplicated(code, this.editingWorkspace.workspaceId)) {
      return;
    }
    try {
      const updated = await this.api.patch<ApiWorkspace>(`/api/workspaces/${encodeURIComponent(this.editingWorkspace.workspaceId)}`, {
        name: this.workspaceNameValue.trim(),
        code,
        status: this.editingWorkspace.status,
      });
      this.editingWorkspace.name = updated.name;
      this.editingWorkspace.code = updated.code ?? code;
      this.editingWorkspace.template = updated.templateKey ?? this.editingWorkspace.code;
    } catch {
      this.editingWorkspace.name = this.workspaceNameValue.trim();
      this.editingWorkspace.code = code;
      this.editingWorkspace.template = this.editingWorkspace.code;
    }
    this.editingWorkspace.zone = `${slug(this.editingWorkspace.code)}.${this.editingWorkspace.workspaceId}.user-000001.sub.slan.com`;
    this.closeWorkspaceTagDialogs();
  }

  isWorkspaceCodeDuplicated(code: string, exceptWorkspaceId = ''): boolean {
    return this.workspaces.some((workspace) => workspace.workspaceId !== exceptWorkspaceId && workspace.code === code);
  }

  async toggleWorkspace(workspace: WorkspaceRow): Promise<void> {
    const status = workspace.status === 'enabled' ? 'disabled' : 'enabled';
    try {
      const updated = await this.api.patch<ApiWorkspace>(`/api/workspaces/${encodeURIComponent(workspace.workspaceId)}`, {
        name: workspace.name,
        code: workspace.code,
        status,
      });
      workspace.status = updated.status;
    } catch {
      workspace.status = status;
    }
  }

  async openInviteDialog(): Promise<void> {
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

  removeMember(member: MemberRow): void {
    if (member.role === 'owner') {
      return;
    }
    this.members = this.members.filter((item) => item !== member);
  }

  async removeWorkspaceDevice(device: DeviceRow): Promise<void> {
    try {
      await this.api.delete(`/api/workspaces/${encodeURIComponent(this.selectedWorkspaceId)}/devices/${encodeURIComponent(device.deviceId)}`);
    } catch {
      // Local preview mode removes the row below.
    }
    const currentIds = this.currentWorkspaceDeviceIds().filter((deviceId) => deviceId !== device.deviceId);
    this.workspaceDeviceIdsByWorkspace[this.selectedWorkspaceId] = currentIds;
    this.selectedWorkspace.devices = currentIds.length;
  }

  openWorkspaceDeviceDialog(): void {
    this.bindDeviceQuery = '';
    this.selectedWorkspaceDeviceId = this.queriedBindableWorkspaceDevices[0]?.deviceId ?? '';
    this.showWorkspaceDeviceDialog = true;
  }

  queryWorkspaceDevices(): void {
    const first = this.queriedBindableWorkspaceDevices[0]?.deviceId ?? '';
    if (!this.queriedBindableWorkspaceDevices.some((device) => device.deviceId === this.selectedWorkspaceDeviceId)) {
      this.selectedWorkspaceDeviceId = first;
    }
  }

  closeWorkspaceDeviceDialog(): void {
    this.showWorkspaceDeviceDialog = false;
  }

  async saveWorkspaceDeviceDialog(): Promise<void> {
    const currentIds = this.currentWorkspaceDeviceIds();
    if (!this.selectedWorkspaceDeviceId || currentIds.includes(this.selectedWorkspaceDeviceId)) {
      return;
    }
    const selected = this.devices.find((device) => device.deviceId === this.selectedWorkspaceDeviceId);
    try {
      await this.api.post(`/api/workspaces/${encodeURIComponent(this.selectedWorkspaceId)}/devices`, {
        deviceId: this.selectedWorkspaceDeviceId,
        actorUserId: this.currentUserId || selected?.owner || 'user-000001',
        alias: selected?.alias ?? '',
        enabled: true,
      });
      await this.loadWorkspaceDevices(this.selectedWorkspaceId);
      this.closeWorkspaceDeviceDialog();
      return;
    } catch {
      // Local preview mode updates the in-memory relationship below.
    }
    const nextIds = [...currentIds, this.selectedWorkspaceDeviceId];
    this.workspaceDeviceIdsByWorkspace[this.selectedWorkspaceId] = nextIds;
    this.workspaceDeviceJoinMethods[`${this.selectedWorkspaceId}|${this.selectedWorkspaceDeviceId}`] = '手动添加';
    this.selectedWorkspace.devices = nextIds.length;
    this.closeWorkspaceDeviceDialog();
  }

  openWorkspaceDeviceAliasDialog(device: DeviceRow): void {
    if (this.showWorkspaceDeviceAliasDialog && this.editingWorkspaceDevice?.deviceId === device.deviceId) {
      this.closeWorkspaceDeviceAliasDialog();
      return;
    }
    this.editingWorkspaceDevice = device;
    this.workspaceDeviceAliasValue = device.alias;
    this.showWorkspaceDeviceAliasDialog = true;
  }

  closeWorkspaceDeviceAliasDialog(): void {
    this.showWorkspaceDeviceAliasDialog = false;
    this.editingWorkspaceDevice = null;
  }

  async saveWorkspaceDeviceAliasDialog(): Promise<void> {
    if (!this.editingWorkspaceDevice || !this.workspaceDeviceAliasValue.trim()) {
      return;
    }
    const device = this.editingWorkspaceDevice;
    const alias = this.workspaceDeviceAliasValue.trim();
    try {
      await this.api.patch(`/api/workspaces/${encodeURIComponent(this.selectedWorkspaceId)}/devices/${encodeURIComponent(device.deviceId)}`, {
        alias,
      });
    } catch {
      // Local preview mode updates the current mock row.
    }
    this.devices = this.devices.map((item) => item.deviceId === device.deviceId ? { ...item, alias } : item);
    this.closeWorkspaceDeviceAliasDialog();
  }

  openZoneDialog(): void {
    const workspace = this.selectedWorkspace;
    this.zoneName = workspace.code === 'default' ? 'default.lan' : `${workspace.code}.internal`;
    this.zoneRecordType = 'A';
    this.zoneValue = this.devices[0]?.ip ?? '10.0.0.1';
    this.editingZone = null;
    this.zoneDialogMode = 'create';
    this.showZoneDialog = true;
  }

  openEditZoneDialog(zone: DNSZoneRow): void {
    this.zoneName = zone.zone;
    this.zoneRecordType = zone.recordType;
    this.zoneValue = zone.value;
    this.editingZone = zone;
    this.zoneDialogMode = 'edit';
    this.showZoneDialog = true;
  }

  openZoneTagDialog(zone: DNSZoneRow): void {
    if (this.showZoneTagDialog && this.editingZone === zone) {
      this.closeZoneTagDialog();
      return;
    }
    this.editingZone = zone;
    this.zoneNameValue = zone.zone;
    this.showZoneTagDialog = true;
  }

  closeZoneTagDialog(): void {
    this.showZoneTagDialog = false;
    this.editingZone = null;
  }

  saveZoneTagDialog(): void {
    if (!this.editingZone || !this.zoneNameValue.trim()) {
      return;
    }
    this.editingZone.zone = this.normalizePrivateZone(this.zoneNameValue);
    this.closeZoneTagDialog();
  }

  closeZoneDialog(): void {
    this.showZoneDialog = false;
  }

  async saveZoneDialog(): Promise<void> {
    const workspace = this.selectedWorkspace;
    const zone = this.normalizePrivateZone(this.zoneName);
    if (this.zoneDialogMode === 'edit' && this.editingZone) {
      try {
        const updated = await this.api.patch<ApiDNSZone>(`/api/workspaces/${encodeURIComponent(workspace.workspaceId)}/dns/zones/${encodeURIComponent(this.editingZone.zoneId ?? this.editingZone.zone)}`, {
          zoneName: zone,
          exposeGlobal: this.editingZone.expose,
        });
        Object.assign(this.editingZone, this.mapDNSZone(updated));
      } catch {
        this.editingZone.zone = zone;
        this.editingZone.recordType = this.zoneRecordType;
        this.editingZone.value = this.zoneValue;
      }
      this.closeZoneDialog();
      return;
    }
    try {
      const created = await this.api.post<ApiDNSZone>(`/api/workspaces/${encodeURIComponent(workspace.workspaceId)}/dns/zones`, {
        zoneName: zone,
        exposeGlobal: workspace.name !== '默认网络',
      });
      this.dnsZones = [...this.dnsZones, this.mapDNSZone(created)];
    } catch {
      this.dnsZones = [
        ...this.dnsZones,
        { workspaceId: workspace.workspaceId, zone, recordType: this.zoneRecordType, value: this.zoneValue, expose: workspace.name !== '默认网络', status: 'active' },
      ];
    }
    this.closeZoneDialog();
  }

  async removeZone(zone: DNSZoneRow): Promise<void> {
    try {
      await this.api.delete(`/api/workspaces/${encodeURIComponent(zone.workspaceId)}/dns/zones/${encodeURIComponent(zone.zoneId ?? zone.zone)}`);
    } catch {
      // Local preview mode removes below.
    }
    this.dnsZones = this.dnsZones.filter((item) => item !== zone);
  }

  openRecordDialog(): void {
    this.recordName = this.domainName;
    this.recordType = 'A';
    this.recordDeviceId = this.workspaceDevices[0]?.deviceId ?? this.devices[0]?.deviceId ?? '';
    this.recordPort = '443';
    this.recordValue = this.buildRecordValue();
    this.editingRecord = null;
    this.recordDialogMode = 'create';
    this.showRecordDialog = true;
  }

  openEditRecordDialog(record: DNSRow): void {
    this.recordName = record.name;
    this.recordType = record.recordType;
    this.recordDeviceId = record.deviceId || this.workspaceDevices[0]?.deviceId || '';
    this.recordPort = record.port || '443';
    this.recordValue = this.buildRecordValue();
    this.editingRecord = record;
    this.recordDialogMode = 'edit';
    this.showRecordDialog = true;
  }

  buildRecordValue(): string {
    const device = this.devices.find((item) => item.deviceId === this.recordDeviceId);
    if (!device) {
      return `- / - / ${this.recordPort}`;
    }
    return `${this.userLabel(device.owner)} / ${device.alias || device.deviceId} / ${this.recordPort}`;
  }

  dnsRecordValue(record: DNSRow): string {
    const device = this.devices.find((item) => item.deviceId === record.deviceId);
    if (!device) {
      return this.displayUserText(record.value || '-');
    }
    return `${this.userLabel(device.owner)} / ${device.alias || device.deviceId} / ${record.port || '-'}`;
  }

  publicMappingDeviceLabel(mapping: PublicMappingRow): string {
    const device = this.devices.find((item) => item.deviceId === mapping.deviceId);
    if (!device) {
      return mapping.deviceId || '-';
    }
    return `${this.userLabel(device.owner)} / ${device.alias || device.deviceId}`;
  }

  syncRecordValue(): void {
    this.recordValue = this.buildRecordValue();
  }

  closeRecordDialog(): void {
    this.showRecordDialog = false;
  }

  async saveRecordDialog(): Promise<void> {
    const workspace = this.selectedWorkspace;
    const name = slug(this.recordName);
    const zoneRow = this.currentDNSZones.find((item) => item.zone === this.selectedZoneId) ?? this.currentDNSZones[0];
    const zone = zoneRow?.zone ?? `${workspace.code}.internal`;
    const fqdn = `${name}.${zone}`;
    this.recordValue = this.buildRecordValue();
    if (this.recordDialogMode === 'edit' && this.editingRecord) {
      try {
        const updated = await this.api.patch<ApiDNSRecord>(`/api/workspaces/${encodeURIComponent(workspace.workspaceId)}/dns/records/${encodeURIComponent(this.editingRecord.recordId ?? this.editingRecord.fqdn)}`, {
          name,
          recordType: this.recordType,
          targetDeviceId: this.recordDeviceId,
          port: this.recordPort,
          ttl: 60,
        });
        Object.assign(this.editingRecord, this.mapDNSRecord(updated));
      } catch {
        this.editingRecord.name = name;
        this.editingRecord.fqdn = fqdn;
        this.editingRecord.recordType = this.recordType;
        this.editingRecord.value = this.recordValue;
        this.editingRecord.deviceId = this.recordDeviceId;
        this.editingRecord.port = this.recordPort;
      }
      this.closeRecordDialog();
      return;
    }
    try {
      const created = await this.api.post<ApiDNSRecord>(`/api/workspaces/${encodeURIComponent(workspace.workspaceId)}/dns/records`, {
        zoneId: zoneRow?.zoneId ?? zoneRow?.zone ?? '',
        name,
        recordType: this.recordType,
        targetDeviceId: this.recordDeviceId,
        port: this.recordPort,
        ttl: 60,
      });
      this.dnsRecords = [...this.dnsRecords, this.mapDNSRecord(created)];
    } catch {
      this.dnsRecords = [
        ...this.dnsRecords,
        { workspaceId: workspace.workspaceId, name, fqdn, recordType: this.recordType, value: this.recordValue, deviceId: this.recordDeviceId, port: this.recordPort, expose: false },
      ];
    }
    this.closeRecordDialog();
  }

  async removeDomainRecord(record: DNSRow): Promise<void> {
    try {
      await this.api.delete(`/api/workspaces/${encodeURIComponent(record.workspaceId)}/dns/records/${encodeURIComponent(record.recordId ?? record.fqdn)}`);
    } catch {
      // Local preview mode removes below.
    }
    this.dnsRecords = this.dnsRecords.filter((item) => item !== record);
  }

  private normalizePrivateZone(value: string): string {
    return value
      .trim()
      .toLowerCase()
      .replace(/^https?:\/\//, '')
      .replace(/\/.*$/, '')
      .replace(/[^a-z0-9.-]+/g, '-')
      .replace(/^-+|-+$/g, '') || 'internal.lan';
  }

  openPublicMappingDialog(): void {
    const device = this.currentUserDevices[0];
    this.publicAlias = 'api';
    this.publicSourceRecord = device?.deviceId ?? '';
    this.publicProtocol = 'HTTP';
    this.publicExternalPort = '443';
    this.publicAccessMode = 'public';
    this.publicTlsMode = 'auto';
    this.editingPublicMapping = null;
    this.publicMappingDialogMode = 'create';
    this.showPublicMappingDialog = true;
  }

  openEditPublicMappingDialog(mapping: PublicMappingRow): void {
    this.publicAlias = mapping.alias;
    this.publicSourceRecord = mapping.deviceId || mapping.sourceRecord;
    this.publicProtocol = mapping.protocol;
    this.publicExternalPort = mapping.externalPort;
    this.publicAccessMode = mapping.accessMode;
    this.publicTlsMode = mapping.tlsMode;
    this.editingPublicMapping = mapping;
    this.publicMappingDialogMode = 'edit';
    this.showPublicMappingDialog = true;
  }

  closePublicMappingDialog(): void {
    this.showPublicMappingDialog = false;
  }

  async savePublicMappingDialog(): Promise<void> {
    const device = this.currentUserDevices.find((item) => item.deviceId === this.publicSourceRecord) ?? this.currentUserDevices[0];
    const alias = slug(this.publicAlias);
    const publicDomain = `${alias}.${this.selectedWorkspace.code}.${this.userSlug}.pub.slan.com`;
    if (this.publicMappingDialogMode === 'edit' && this.editingPublicMapping) {
      try {
        const updated = await this.api.patch<ApiPublicMapping>(`/api/workspaces/${encodeURIComponent(this.selectedWorkspaceId)}/public-mappings/${encodeURIComponent(this.editingPublicMapping.mappingId ?? this.editingPublicMapping.publicDomain)}`, {
          alias,
          publicDomain,
          sourceRecord: device?.alias || device?.deviceId || '',
          deviceId: device?.deviceId ?? '',
          protocol: this.publicProtocol,
          port: this.publicExternalPort,
          externalPort: this.publicExternalPort,
          status: this.editingPublicMapping.status,
        });
        Object.assign(this.editingPublicMapping, this.mapPublicMapping(updated));
      } catch {
        this.editingPublicMapping.alias = alias;
        this.editingPublicMapping.publicDomain = publicDomain;
        this.editingPublicMapping.sourceRecord = device?.alias || device?.deviceId || '';
        this.editingPublicMapping.deviceId = device?.deviceId ?? '';
        this.editingPublicMapping.protocol = this.publicProtocol;
        this.editingPublicMapping.port = this.publicExternalPort;
        this.editingPublicMapping.externalPort = this.publicExternalPort;
        this.editingPublicMapping.accessMode = this.publicAccessMode;
        this.editingPublicMapping.tlsMode = this.publicTlsMode;
      }
      this.closePublicMappingDialog();
      return;
    }
    try {
      const created = await this.api.post<ApiPublicMapping>(`/api/workspaces/${encodeURIComponent(this.selectedWorkspaceId)}/public-mappings`, {
        alias,
        publicDomain,
        sourceRecord: device?.alias || device?.deviceId || '',
        deviceId: device?.deviceId ?? '',
        protocol: this.publicProtocol,
        port: this.publicExternalPort,
        externalPort: this.publicExternalPort,
        status: 'enabled',
      });
      this.publicMappings = [...this.publicMappings, this.mapPublicMapping(created)];
    } catch {
      this.publicMappings = [
        ...this.publicMappings,
        { workspaceId: this.selectedWorkspaceId, alias, publicDomain, sourceRecord: device?.alias || device?.deviceId || '', deviceId: device?.deviceId ?? '', protocol: this.publicProtocol, port: this.publicExternalPort, externalPort: this.publicExternalPort, accessMode: this.publicAccessMode, tlsMode: this.publicTlsMode, status: 'enabled' },
      ];
    }
    this.closePublicMappingDialog();
  }

  async removePublicMapping(mapping: PublicMappingRow): Promise<void> {
    try {
      await this.api.delete(`/api/workspaces/${encodeURIComponent(mapping.workspaceId)}/public-mappings/${encodeURIComponent(mapping.mappingId ?? mapping.publicDomain)}`);
    } catch {
      // Local preview mode removes below.
    }
    this.publicMappings = this.publicMappings.filter((item) => item !== mapping);
  }

  async removeSecurityGroup(group: SecurityGroupRow): Promise<void> {
    try {
      await this.api.delete(`/api/workspaces/${encodeURIComponent(group.workspaceId)}/security-groups/${encodeURIComponent(group.securityGroupId)}`);
    } catch {
      // Local preview mode removes below.
    }
    this.securityGroups = this.securityGroups.filter((item) => item.securityGroupId !== group.securityGroupId);
    if (this.selectedSecurityGroupId === group.securityGroupId) {
      this.selectedSecurityGroupId = this.currentSecurityGroups[0]?.securityGroupId ?? '';
    }
  }

  openRuleDialog(direction: string): void {
    this.ruleDirection = direction;
    this.selectedRuleTemplate = direction === 'egress' ? '全部出站' : 'Web 服务';
    this.applyRuleTemplate(direction);
    this.editingRule = null;
    this.ruleDialogMode = 'create';
    this.showIngressRuleDialog = direction === 'ingress';
    this.showEgressRuleDialog = direction === 'egress';
  }

  openEditRuleDialog(rule: SecurityRuleRow): void {
    this.ruleDirection = rule.direction;
    this.rulePriority = rule.priority;
    this.ruleAction = rule.action;
    this.ruleProtocol = rule.protocol;
    this.rulePort = rule.port;
    this.ruleSubjectType = rule.subjectType;
    this.ruleSubjectValue = rule.subjectValue;
    this.selectedRuleTemplate = '自定义';
    this.editingRule = rule;
    this.ruleDialogMode = 'edit';
    this.showIngressRuleDialog = rule.direction === 'ingress';
    this.showEgressRuleDialog = rule.direction === 'egress';
  }

  applyRuleTemplate(direction = this.ruleDirection): void {
    const template = this.securityRuleTemplates.find((item) => item.name === this.selectedRuleTemplate && item.direction === direction);
    if (!template) {
      this.ruleDirection = direction;
      return;
    }
    this.ruleDirection = direction;
    this.rulePriority = template.priority;
    this.ruleAction = template.action;
    this.ruleProtocol = template.protocol;
    this.rulePort = template.port;
    this.ruleSubjectType = template.subjectType;
    this.ruleSubjectValue = template.subjectValue;
    this.onRuleSubjectTypeChanged();
  }

  ruleSubjectLabel(rule: SecurityRuleRow): string {
    const prefix = this.subjectTypeLabel(rule.subjectType);
    if (rule.subjectType === 'device') {
      const device = this.devices.find((item) => item.deviceId === rule.subjectValue);
      return `${prefix}:${device ? `${this.userLabel(device.owner)} / ${device.alias || device.deviceId} / ${device.deviceId}` : rule.subjectValue}`;
    }
    if (rule.subjectType === 'workspace') {
      const workspace = rule.subjectValue === 'self' ? this.selectedWorkspace : this.workspaces.find((item) => item.workspaceId === rule.subjectValue);
      return `${prefix}:${workspace?.name ?? rule.subjectValue}`;
    }
    return `${prefix}:${rule.subjectValue}`;
  }

  subjectTypeLabel(type: RuleSubjectType): string {
    return ({ device: '设备', user: '用户', workspace: '网络', cidr: 'CIDR', domain: '域名', all: '全部' } as Record<RuleSubjectType, string>)[type];
  }

  onRuleSubjectTypeChanged(): void {
    const first = this.ruleSubjectOptions[0]?.value;
    if (first) {
      this.ruleSubjectValue = first;
      return;
    }
    this.ruleSubjectValue = this.ruleSubjectType === 'cidr' ? '0.0.0.0/0' : 'example.com';
  }

  closeRuleDialog(): void {
    this.showIngressRuleDialog = false;
    this.showEgressRuleDialog = false;
  }

  async saveRuleDialog(): Promise<void> {
    const portFrom = this.rulePort === 'all' ? 0 : Number.parseInt(this.rulePort.split(',')[0], 10) || 0;
    const portTo = this.rulePort === 'all' ? 0 : Number.parseInt(this.rulePort.split(',').at(-1) ?? this.rulePort, 10) || portFrom;
    if (this.ruleDialogMode === 'edit' && this.editingRule) {
      try {
        const updated = await this.api.patch<ApiSecurityRule>(`/api/security-groups/rules/${encodeURIComponent(this.editingRule.ruleId ?? '')}`, {
          direction: this.ruleDirection,
          priority: this.rulePriority,
          action: this.ruleAction,
          protocol: this.ruleProtocol,
          portFrom,
          portTo,
          peerType: this.ruleSubjectType,
          peerValue: this.ruleSubjectValue,
          enabled: true,
        });
        Object.assign(this.editingRule, this.mapSecurityRule(updated));
      } catch {
        this.editingRule.direction = this.ruleDirection;
        this.editingRule.priority = this.rulePriority;
        this.editingRule.action = this.ruleAction;
        this.editingRule.protocol = this.ruleProtocol;
        this.editingRule.port = this.rulePort;
        this.editingRule.subjectType = this.ruleSubjectType;
        this.editingRule.subjectValue = this.ruleSubjectValue;
      }
      this.closeRuleDialog();
      return;
    }
    try {
      const created = await this.api.post<ApiSecurityRule>(`/api/security-groups/${encodeURIComponent(this.selectedSecurityGroupId)}/rules`, {
        direction: this.ruleDirection,
        priority: this.rulePriority,
        action: this.ruleAction,
        protocol: this.ruleProtocol,
        portFrom,
        portTo,
        peerType: this.ruleSubjectType,
        peerValue: this.ruleSubjectValue,
        enabled: true,
      });
      this.securityRules = [...this.securityRules, this.mapSecurityRule(created)];
    } catch {
      this.securityRules = [
        ...this.securityRules,
        { direction: this.ruleDirection, priority: this.rulePriority, action: this.ruleAction, protocol: this.ruleProtocol, port: this.rulePort, subjectType: this.ruleSubjectType, subjectValue: this.ruleSubjectValue },
      ];
    }
    this.closeRuleDialog();
  }

  async removeSecurityRule(rule: SecurityRuleRow): Promise<void> {
    try {
      await this.api.delete(`/api/security-groups/rules/${encodeURIComponent(rule.ruleId ?? '')}`);
    } catch {
      // Local preview mode removes below.
    }
    this.securityRules = this.securityRules.filter((item) => item !== rule);
  }

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

  effectiveInviteStatus(invite: WorkspaceDeviceInviteRow): string {
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

  private applyRouteFromLocation(): void {
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
      this.workspacePanel = routePanel.panel;
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

  private async loadDashboard(userId = ''): Promise<void> {
    try {
      const [devices, workspaces] = await Promise.all([
        this.api.get<{ items: ApiDevice[] }>(`/api/devices/visible${userId ? `?userId=${encodeURIComponent(userId)}` : ''}`),
        this.api.get<{ items: ApiWorkspace[] }>(`/api/workspaces${userId ? `?userId=${encodeURIComponent(userId)}` : ''}`),
      ]);
      this.devices = devices.items.map((device) => ({
        deviceId: device.deviceId,
        platform: device.platform,
        osVersion: device.osVersion ?? '',
        alias: device.alias || device.name || device.deviceId,
        ip: device.globalIp,
        owner: device.ownerId,
        status: device.status,
      }));
      this.workspaces = workspaces.items.map((workspace) => ({
        workspaceId: workspace.workspaceId,
        name: workspace.name,
        code: workspace.code ?? slug(workspace.name),
        template: workspace.templateKey ?? 'custom',
        status: workspace.status,
        members: 0,
        devices: 0,
        zone: `${workspace.code ?? slug(workspace.name)}.${workspace.workspaceId}.${userId || 'user'}.sub.slan.com`,
      }));
      await Promise.all(this.workspaces.map((workspace) => this.loadWorkspaceDevices(workspace.workspaceId)));
    } catch {
      // The checked-in UI remains previewable without a running API.
    }
  }

  private async loadWorkspaceDevices(workspaceId: string): Promise<void> {
    try {
      const response = await this.api.get<{ items: ApiWorkspaceDevice[] }>(`/api/workspaces/${encodeURIComponent(workspaceId)}/devices`);
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

  private async loadWorkspaceResources(workspaceId: string): Promise<void> {
    await Promise.all([
      this.loadDNSZones(workspaceId),
      this.loadDNSRecords(workspaceId),
      this.loadPublicMappings(workspaceId),
      this.loadSecurityResources(workspaceId),
    ]);
  }

  private async loadDNSZones(workspaceId: string): Promise<void> {
    try {
      const response = await this.api.get<{ items: ApiDNSZone[] }>(`/api/workspaces/${encodeURIComponent(workspaceId)}/dns/zones`);
      this.dnsZones = [...this.dnsZones.filter((zone) => zone.workspaceId !== workspaceId), ...response.items.map((zone) => this.mapDNSZone(zone))];
    } catch {
      // Preview seed data remains available without the API.
    }
  }

  private async loadDNSRecords(workspaceId: string): Promise<void> {
    try {
      const response = await this.api.get<{ items: ApiDNSRecord[] }>(`/api/workspaces/${encodeURIComponent(workspaceId)}/dns/records`);
      this.dnsRecords = [...this.dnsRecords.filter((record) => record.workspaceId !== workspaceId), ...response.items.map((record) => this.mapDNSRecord(record))];
    } catch {
      // Preview seed data remains available without the API.
    }
  }

  private async loadPublicMappings(workspaceId: string): Promise<void> {
    try {
      const response = await this.api.get<{ items: ApiPublicMapping[] }>(`/api/workspaces/${encodeURIComponent(workspaceId)}/public-mappings`);
      this.publicMappings = [...this.publicMappings.filter((mapping) => mapping.workspaceId !== workspaceId), ...response.items.map((mapping) => this.mapPublicMapping(mapping))];
    } catch {
      // Preview seed data remains available without the API.
    }
  }

  private async loadSecurityResources(workspaceId: string): Promise<void> {
    try {
      const groups = await this.api.get<{ items: ApiSecurityGroup[] }>(`/api/workspaces/${encodeURIComponent(workspaceId)}/security-groups`);
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

  private async loadSecurityRules(securityGroupId: string): Promise<void> {
    try {
      const rules = await this.api.get<{ items: ApiSecurityRule[] }>(`/api/security-groups/${encodeURIComponent(securityGroupId)}/rules`);
      this.securityRules = rules.items.map((rule) => this.mapSecurityRule(rule));
    } catch {
      // Preview seed data remains available without the API.
    }
  }

  private mapDNSZone(zone: ApiDNSZone): DNSZoneRow {
    return { zoneId: zone.zoneId, workspaceId: zone.workspaceId, zone: zone.zoneName, recordType: 'A', value: '', expose: zone.exposeGlobal, status: zone.status };
  }

  private mapDNSRecord(record: ApiDNSRecord): DNSRow {
    const port = record.port ?? '';
    const value = record.targetDeviceId ? this.dnsRecordValue({ workspaceId: record.workspaceId, name: record.name, fqdn: record.fqdn, recordType: record.recordType, value: '', deviceId: record.targetDeviceId, port, expose: false }) : (record.targetIp || record.cname || '');
    return { recordId: record.recordId, zoneId: record.zoneId, workspaceId: record.workspaceId, name: record.name, fqdn: record.fqdn, recordType: record.recordType, value, deviceId: record.targetDeviceId ?? '', port, expose: false };
  }

  private mapPublicMapping(mapping: ApiPublicMapping): PublicMappingRow {
    return { mappingId: mapping.mappingId, workspaceId: mapping.workspaceId, alias: mapping.alias, publicDomain: mapping.publicDomain, sourceRecord: mapping.sourceRecord, deviceId: mapping.deviceId, protocol: mapping.protocol, port: mapping.port, externalPort: mapping.externalPort, accessMode: 'public', tlsMode: 'off', status: mapping.status };
  }

  private mapSecurityGroup(group: ApiSecurityGroup): SecurityGroupRow {
    return { securityGroupId: group.securityGroupId, workspaceId: group.workspaceId, name: group.name, defaultPolicy: group.defaultPolicy, status: group.status };
  }

  private mapSecurityRule(rule: ApiSecurityRule): SecurityRuleRow {
    const port = rule.portFrom === 0 && rule.portTo === 0 ? 'all' : rule.portFrom === rule.portTo ? String(rule.portFrom) : `${rule.portFrom},${rule.portTo}`;
    return { ruleId: rule.ruleId, direction: rule.direction, priority: rule.priority, action: rule.action, protocol: rule.protocol, port, subjectType: rule.peerType, subjectValue: rule.peerValue };
  }

  private upsertWorkspaceDeviceInvite(invite: WorkspaceDeviceInviteRow): void {
    this.workspaceDeviceInvites = [
      invite,
      ...this.workspaceDeviceInvites.filter((item) => item.inviteCode !== invite.inviteCode),
    ];
  }

  private markInviteAccepted(inviteCode: string, workspaceId: string, deviceId: string): void {
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
