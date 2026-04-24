import { CommonModule } from '@angular/common';
import { Component, OnDestroy, computed, inject, signal } from '@angular/core';

import { AuthResponse, Device, NetworkAssignment, NetworkDetail, NetworkHome, Subnet } from './api-contracts';
import { AuthPanelComponent } from './auth-panel.component';
import { ConsoleApiError, ConsoleApiService } from './console-api.service';
import { AuthenticateResult, ConsoleAppFacadeService, RefreshWorkspaceResult } from './console-app-facade.service';
import { CallbackTrackingHandle, ConsoleCallbackService } from './console-callback.service';
import { ConsoleNetworkFormService } from './console-network-form.service';
import { ConsoleSessionService } from './console-session.service';
import { ConsoleWorkspaceService } from './console-workspace.service';
import { DeviceContextComponent } from './device-context.component';
import { NetworkEmptyStateComponent } from './network-empty-state.component';
import { NetworkWorkspaceComponent } from './network-workspace.component';
import { AuthMode, ConsoleView } from './ui-models';

@Component({
  selector: 'app-root',
  standalone: true,
  imports: [
    CommonModule,
    AuthPanelComponent,
    DeviceContextComponent,
    NetworkEmptyStateComponent,
    NetworkWorkspaceComponent
  ],
  templateUrl: './app.component.html',
  styleUrl: './app.component.css'
})
export class AppComponent implements OnDestroy {
  private static readonly SESSION_CHECK_INTERVAL_MS = 30_000;
  readonly navItems: Array<{ id: ConsoleView; label: string; caption: string }> = [
    { id: 'account', label: '账户概览', caption: '账号、设备、接入网络、切换与接入入口' },
    { id: 'network', label: '网络管理', caption: '子网、IP 绑定、设备接入和加入 key 管理' },
    { id: 'devices', label: '设备管理', caption: '邮箱、当前 IP、链接状态、协议、加入时间、创建时间' }
  ];
  private readonly facade = inject(ConsoleAppFacadeService);
  private readonly callbackService = inject(ConsoleCallbackService);
  private readonly networkFormService = inject(ConsoleNetworkFormService);
  private readonly api = inject(ConsoleApiService);
  private readonly sessionService = inject(ConsoleSessionService);
  private readonly workspaceService = inject(ConsoleWorkspaceService);
  email = '';
  password = '';
  createName = 'My Network';
  createDescription = 'Personal host network';
  createCidr = '10.0.0.0/16';
  joinOwnerEmail = '';
  joinKey = '';
  deviceSearch = '';
  deviceSort = 'created_desc';
  deviceStatusFilter = 'all';
  assignmentSearch = '';
  assignmentRoleFilter = 'all';
  updateName = '';
  updateDescription = '';
  updateCidr = '';
  networkJoinKey = '';
  subnetName = '';
  subnetCidr = '';
  subnetGatewayIp = '';
  subnetAllocationStartIp = '';
  subnetAllocationEndIp = '';

  readonly token = signal(localStorage.getItem('slan.accessToken') || '');
  readonly userId = signal(localStorage.getItem('slan.userId') || '');
  readonly authMode = signal<AuthMode>('login');
  readonly activeView = signal<ConsoleView>('account');
  readonly loading = signal(false);
  readonly actionBusy = signal('');
  readonly error = signal('');
  readonly message = signal('');
  readonly home = signal<NetworkHome>({ hasNetwork: false });
  readonly detail = signal<NetworkDetail | null>(null);
  readonly assignments = signal<NetworkAssignment[]>([]);
  readonly devices = signal<Device[]>([]);
  readonly subnets = signal<Subnet[]>([]);
  readonly currentDeviceId = signal(localStorage.getItem('slan.deviceId') || '');
  readonly draftIps = signal<Record<string, string>>({});
  readonly draftRemarks = signal<Record<string, string>>({});
  readonly activeNetwork = computed(() => this.home().activeNetwork);
  readonly selectedDevice = computed(() => this.devices().find((item) => item.deviceId === this.currentDeviceId()) || null);
  readonly currentAssignment = computed(() => {
    const deviceId = this.currentDeviceId();
    return this.assignments().find((item) => item.deviceId === deviceId) || null;
  });
  readonly currentSubnet = computed(() => {
    const assignment = this.currentAssignment();
    return assignment ? this.subnets().find((item) => item.subnetId === assignment.subnetId) || null : null;
  });
  readonly joinedNetworks = computed(() => {
    const items = [this.home().activeNetwork, this.home().ownedNetwork].filter(Boolean) as NonNullable<NetworkHome['activeNetwork']>[];
    return items.filter((item, index, list) => list.findIndex((candidate) => candidate.networkId === item.networkId) === index);
  });
  readonly canManageNetwork = computed(() => !!this.detail()?.ownedByCurrentUser);
  readonly filteredAssignments = computed(() => {
    const keyword = this.assignmentSearch.trim().toLowerCase();
    const roleFilter = this.assignmentRoleFilter;
    return this.assignments().filter((item) => {
      if (roleFilter !== 'all' && item.role !== roleFilter) {
        return false;
      }
      if (!keyword) {
        return true;
      }
      return [item.userEmail, item.deviceName, item.deviceId, item.remark || '', item.virtualIp || '', item.role]
        .join(' ')
        .toLowerCase()
        .includes(keyword);
    });
  });
  readonly filteredDevices = computed(() => {
    const keyword = this.deviceSearch.trim().toLowerCase();
    const statusFilter = this.deviceStatusFilter;
    const filtered = this.devices().filter((item) => {
      if (statusFilter !== 'all') {
        const tone = this.deviceStatusTone(item);
        if (statusFilter === 'online' && tone !== 'success') {
          return false;
        }
        if (statusFilter === 'offline' && tone === 'success') {
          return false;
        }
      }
      if (!keyword) {
        return true;
      }
      return [item.name, item.deviceId, item.ownerEmail || '', item.platform, item.machineId || '']
        .join(' ')
        .toLowerCase()
        .includes(keyword);
    });
    const items = [...filtered];
    switch (this.deviceSort) {
      case 'created_asc':
        return items.sort((left, right) => (left.createdAt || 0) - (right.createdAt || 0));
      case 'joined_desc':
        return items.sort((left, right) => (right.joinedAt || 0) - (left.joinedAt || 0));
      case 'name_asc':
        return items.sort((left, right) => left.name.localeCompare(right.name));
      case 'status_asc':
        return items.sort((left, right) => this.deviceLinkStatus(left).localeCompare(this.deviceLinkStatus(right)));
      case 'created_desc':
      default:
        return items.sort((left, right) => (right.createdAt || 0) - (left.createdAt || 0));
    }
  });
  readonly loginClientDeviceId = signal<string>('');
  readonly loginCallbackId = signal<string>('');
  readonly loginClientPlatform = signal<string>('desktop');
  readonly loginClientName = signal<string>('SLAN Client');
  readonly callbackDeviceId = signal<string>('');
  readonly pendingCallbackId = signal<string>('');
  readonly callbackAcknowledged = signal(false);
  private callbackTracking: CallbackTrackingHandle | null = null;
  private autoCallbackAttempted = false;
  private sessionCheckTimer: number | null = null;
  private sessionCheckInFlight = false;

  constructor() {
    const loginClientContext = this.sessionService.readLoginClientContext(window.location.href);
    if (loginClientContext.authMode) {
      this.authMode.set(loginClientContext.authMode);
    }
    if (loginClientContext.callbackId) {
      this.loginCallbackId.set(loginClientContext.callbackId);
      this.pendingCallbackId.set(loginClientContext.callbackId);
    }
    if (loginClientContext.deviceId) {
      this.loginClientDeviceId.set(loginClientContext.deviceId);
    }
    this.loginClientPlatform.set(loginClientContext.clientPlatform);
    this.loginClientName.set(loginClientContext.clientName);
    if (this.token()) {
      this.startSessionMonitor();
      void this.refreshWorkspace();
    }
  }

  ngOnDestroy(): void {
    this.disposeCallbackTracking();
    this.stopSessionMonitor();
  }

  roleLabel(): string {
    return this.detail()?.ownedByCurrentUser ? 'owner' : (this.home().hasNetwork ? 'member' : 'none');
  }

  currentVirtualIp(): string {
    return this.currentAssignment()?.virtualIp || '未分配';
  }

  currentSubnetLabel(): string {
    const subnet = this.currentSubnet();
    if (!subnet) {
      return '未接入子网';
    }
    return `${subnet.name} · ${subnet.cidr}`;
  }

  networkSwitchTargets(): Array<{ networkId: string; name: string; description?: string }> {
    return this.joinedNetworks().map((network) => ({
      networkId: network.networkId,
      name: network.name,
      description: network.description
    }));
  }

  pendingCapabilityNotes(): Array<{ title: string; detail: string }> {
    return [
      {
        title: '当前已接通',
        detail: '网络加入 key、设备备注和设备 IP 绑定都已经可以直接在这个管理视图里维护。'
      },
      {
        title: '后续可继续扩展',
        detail: '如果还要做加入审批、key 过期或更细的接入策略，这一页可以继续往下长，但当前这版已经能直接管起来。'
      }
    ];
  }

  switchView(view: ConsoleView): void {
    this.activeView.set(view);
  }

  openView(view: ConsoleView): void {
    this.activeView.set(view);
  }

  isActionBusy(key: string): boolean {
    return this.actionBusy() === key;
  }

  async submitAuth(): Promise<void> {
    this.clearNotices();
    this.loading.set(true);
    try {
      const result = await this.facade.authenticate({
        mode: this.authMode(),
        email: this.email,
        password: this.password,
        loginDeviceId: this.loginClientDeviceId().trim() || undefined,
        tokenDeviceState: this.currentDeviceState(),
      });
      this.applyAuthentication(result);
      if (result.workspace) {
        this.applyRefreshWorkspaceResult({
          managedDevice: result.managedDevice,
          workspace: result.workspace,
        });
      }
      await this.forwardCallbackToServer(
        result.auth,
        result.managedDevice.deviceId,
        this.authMode() === 'register' ? 'activate_active_network' : undefined,
      );
    } catch (error) {
      this.setError(error);
    } finally {
      this.loading.set(false);
    }
  }

  async refreshWorkspace(): Promise<void> {
    this.clearNotices();
    if (!this.token()) {
      return;
    }
    this.loading.set(true);
    try {
      const result = await this.facade.refreshWorkspace({
        token: this.token(),
        ...this.currentDeviceState(),
      });
      this.applyRefreshWorkspaceResult(result);
      await this.resumeCachedLoginCallback(result.managedDevice.deviceId);
    } catch (error) {
      this.setError(error);
    } finally {
      this.loading.set(false);
    }
  }

  async reloadDevices(): Promise<void> {
    this.clearNotices();
    this.actionBusy.set('reloadDevices');
    try {
      const loaded = await this.facade.reloadDevices(this.token(), this.currentDeviceId());
      this.devices.set(loaded.devices);
      this.currentDeviceId.set(loaded.currentDeviceId);
      this.message.set('设备列表已刷新');
    } catch (error) {
      this.setError(error);
    } finally {
      this.actionBusy.set('');
    }
  }

  async createOwnNetwork(): Promise<void> {
    this.clearNotices();
    this.actionBusy.set('createNetwork');
    try {
      const result = await this.facade.createOwnNetwork({
        token: this.token(),
        createName: this.createName,
        createDescription: this.createDescription,
        createCidr: this.createCidr,
        deviceState: this.currentDeviceState(),
      });
      this.applyRefreshWorkspaceResult(result.refreshed);
      this.message.set(`created network ${result.network.name}, created default DHCP plan and bound the current device`);
    } catch (error) {
      this.setError(error);
    } finally {
      this.actionBusy.set(''); 
    }
  }

  async joinByOwnerEmail(): Promise<void> {
    this.clearNotices();
    this.actionBusy.set('joinOwnerEmail');
    try {
      const result = await this.facade.joinByOwnerEmail({
        token: this.token(),
        ownerEmail: this.joinOwnerEmail,
        deviceState: this.currentDeviceState(),
      });
      this.applyRefreshWorkspaceResult(result);
      this.message.set(`已加入 ${this.joinOwnerEmail.trim()} 的网络`);
    } catch (error) {
      this.setError(error);
    } finally {
      this.actionBusy.set('');
    }
  }

  async joinByKey(): Promise<void> {
    this.clearNotices();
    this.actionBusy.set('joinByKey');
    try {
      const result = await this.facade.joinByKey({
        token: this.token(),
        joinKey: this.joinKey,
        deviceState: this.currentDeviceState(),
      });
      this.applyRefreshWorkspaceResult(result);
      this.message.set('已通过 Join Key 加入网络');
    } catch (error) {
      this.setError(error);
    } finally {
      this.actionBusy.set('');
    }
  }

  async updateOwnedNetwork(): Promise<void> {
    this.clearNotices();
    const active = this.activeNetwork();
    if (!active) {
      return;
    }
    try {
      const result = await this.facade.updateOwnedNetwork({
        token: this.token(),
        networkId: active.networkId,
        name: this.updateName,
        description: this.updateDescription,
        cidr: this.updateCidr,
        deviceState: this.currentDeviceState(),
      });
      this.applyRefreshWorkspaceResult(result);
      this.message.set('network updated, clients should restart tunnel to pick up the new network plan');
    } catch (error) {
      this.setError(error);
    }
  }

  async createSubnet(): Promise<void> {
    this.clearNotices();
    const active = this.activeNetwork();
    if (!active) {
      return;
    }
    try {
      const result = await this.facade.createSubnet({
        token: this.token(),
        networkId: active.networkId,
        draft: {
          name: this.subnetName,
          cidr: this.subnetCidr,
          gatewayIp: this.subnetGatewayIp,
          allocationStartIp: this.subnetAllocationStartIp,
          allocationEndIp: this.subnetAllocationEndIp,
        },
        deviceState: this.currentDeviceState(),
      });
      this.resetSubnetDraft();
      this.applyRefreshWorkspaceResult(result);
      this.message.set('subnet and dhcp plan created');
    } catch (error) {
      this.setError(error);
    }
  }

  setCurrentDeviceId(deviceId: string): void {
    this.currentDeviceId.set(deviceId);
    this.sessionService.persistCurrentDeviceId(deviceId);
  }

  showSwitchToOwned(): boolean {
    const active = this.home().activeNetwork;
    const owned = this.home().ownedNetwork;
    return !!active && !!owned && active.networkId !== owned.networkId;
  }

  async switchToOwnedNetwork(): Promise<void> {
    const owned = this.home().ownedNetwork;
    if (!owned) {
      return;
    }
    await this.switchToNetwork(owned.networkId);
  }

  async updateAttachmentIp(attachmentId: string): Promise<void> {
    const active = this.activeNetwork();
    if (!active) {
      return;
    }
    try {
      const result = await this.facade.updateAttachmentIp({
        token: this.token(),
        networkId: active.networkId,
        attachmentId,
        virtualIp: this.draftIps()[attachmentId] || '',
        deviceState: this.currentDeviceState(),
      });
      this.applyRefreshWorkspaceResult(result);
      this.message.set('virtual ip updated');
    } catch (error) {
      this.setError(error);
    }
  }

  async updateAttachmentRemark(attachmentId: string): Promise<void> {
    const active = this.activeNetwork();
    if (!active) {
      return;
    }
    try {
      const result = await this.facade.updateAttachmentRemark({
        token: this.token(),
        networkId: active.networkId,
        attachmentId,
        remark: this.draftRemarks()[attachmentId] || '',
        deviceState: this.currentDeviceState(),
      });
      this.applyRefreshWorkspaceResult(result);
      this.message.set('device remark updated');
    } catch (error) {
      this.setError(error);
    }
  }

  async updateNetworkJoinKey(): Promise<void> {
    const active = this.activeNetwork();
    if (!active) {
      return;
    }
    this.actionBusy.set('updateJoinKey');
    try {
      const result = await this.facade.updateNetworkJoinKey({
        token: this.token(),
        networkId: active.networkId,
        joinKey: '',
        deviceState: this.currentDeviceState(),
      });
      this.applyRefreshWorkspaceResult(result);
      if (this.networkJoinKey.trim()) {
        await this.callbackService.copyTarget(this.networkJoinKey.trim());
        this.message.set('已生成新的 32 位 Join Key，并自动复制到剪贴板');
      } else {
        this.message.set('已生成新的 32 位 Join Key');
      }
    } catch (error) {
      this.setError(error);
    } finally {
      this.actionBusy.set('');
    }
  }

  setDraftIp(attachmentId: string, value: string): void {
    this.draftIps.update((draft) => ({ ...draft, [attachmentId]: value }));
  }

  setDraftRemark(attachmentId: string, value: string): void {
    this.draftRemarks.update((draft) => ({ ...draft, [attachmentId]: value }));
  }

  selectedDeviceLabel(): string {
    const device = this.selectedDevice();
    return device ? `${device.name} · ${device.deviceId}` : '';
  }

  formatTimestamp(value?: number): string {
    if (!value) {
      return '-';
    }
    return new Date(value * 1000).toLocaleString();
  }

  deviceLinkStatus(device: Device): string {
    return device.linkStatus || device.status || '-';
  }

  deviceProtocol(device: Device): string {
    return device.connectivityProtocol || '-';
  }

  deviceStatusLabel(device: Device): string {
    const value = (device.linkStatus || device.status || '').toLowerCase();
    switch (value) {
      case 'connected':
        return '已连接';
      case 'online':
        return '在线';
      case 'disconnected':
        return '已断开';
      case 'offline':
        return '离线';
      default:
        return value || '-';
    }
  }

  membershipStatusLabel(device: Device): string {
    const value = (device.membershipStatus || '').toLowerCase();
    switch (value) {
      case 'active':
        return '已加入';
      case 'pending':
      case 'joining':
        return '申请加入中';
      case 'rejected':
        return '已拒绝';
      case 'disabled':
        return '已停用';
      default:
        return value || '-';
    }
  }

  membershipStatusTone(device: Device): string {
    const value = (device.membershipStatus || '').toLowerCase();
    switch (value) {
      case 'active':
        return 'success';
      case 'pending':
      case 'joining':
        return 'warn';
      default:
        return 'muted';
    }
  }

  deviceStatusTone(device: Device): string {
    const value = (device.linkStatus || device.status || '').toLowerCase();
    switch (value) {
      case 'connected':
      case 'online':
        return 'success';
      case 'disconnected':
        return 'warn';
      case 'offline':
      default:
        return 'muted';
    }
  }

  async copyNetworkJoinKey(): Promise<void> {
    if (!this.networkJoinKey.trim()) {
      return;
    }
    try {
      await this.callbackService.copyTarget(this.networkJoinKey.trim());
      this.message.set('Join Key 已复制到剪贴板');
      this.error.set('');
    } catch (_) {
      this.error.set('复制 Join Key 失败，请手动复制');
      this.message.set('');
    }
  }

  logout(): void {
    this.clearCachedAuth();
    this.activeView.set('account');
    this.home.set({ hasNetwork: false });
    this.detail.set(null);
    this.subnets.set([]);
    this.assignments.set([]);
    this.currentDeviceId.set('');
    this.pendingCallbackId.set('');
    this.callbackAcknowledged.set(false);
    this.message.set('logged out');
    this.disposeCallbackTracking();
  }

  private async resumeCachedLoginCallback(deviceId: string): Promise<void> {
    if (this.authMode() !== 'login' || this.autoCallbackAttempted) {
      return;
    }
    if (!this.token() || !this.userId()) {
      return;
    }
    this.autoCallbackAttempted = true;
    this.message.set('检测到网页登录会话，正在转交服务端并等待客户端确认...');
    await this.forwardCallbackToServer(
      {
        accessToken: this.token(),
        userId: this.userId(),
        refreshToken: '',
        expiresIn: 3600,
      },
      deviceId,
    );
  }

  async switchToNetwork(networkId: string): Promise<void> {
    const result = await this.facade.refreshWorkspace({
      token: this.token(),
      ...this.currentDeviceState(),
    });
    this.applyManagedDeviceState(result.managedDevice);
    await this.workspaceService.switchNetwork(this.token(), networkId, result.managedDevice.deviceId);
    const switched = await this.facade.refreshWorkspace({
      token: this.token(),
      ...this.currentDeviceState(),
    });
    this.applyRefreshWorkspaceResult(switched);
    this.message.set(`已切换到网络 ${switched.workspace.home.activeNetwork?.name || networkId}`);
  }

  private hydrateNetworkDrafts(detail: NetworkDetail): void {
    const draft = this.networkFormService.buildNetworkUpdateDraft(detail);
    this.updateName = draft.name;
    this.updateDescription = draft.description;
    this.updateCidr = draft.cidr;
    this.networkJoinKey = detail.joinKey || '';
  }

  private resetSubnetDraft(): void {
    const draft = this.networkFormService.emptySubnetDraft();
    this.subnetName = draft.name;
    this.subnetCidr = draft.cidr;
    this.subnetGatewayIp = draft.gatewayIp;
    this.subnetAllocationStartIp = draft.allocationStartIp;
    this.subnetAllocationEndIp = draft.allocationEndIp;
  }

  private async forwardCallbackToServer(auth: AuthResponse, deviceId?: string, action?: string): Promise<void> {
    const callbackId = this.loginCallbackId().trim();
    if (!callbackId) {
      return;
    }
    const callback = this.sessionService.buildCallbackPayload({
      callbackId,
      auth,
      deviceId,
      userLabel: this.email.trim(),
      action,
    });
    await this.facade.completeCallback(callback.callbackId, {
      accessToken: callback.accessToken,
      userId: callback.userId,
      refreshToken: callback.refreshToken,
      expiresIn: callback.expiresIn,
      deviceId: callback.deviceId,
      userLabel: callback.userLabel,
      action: callback.action,
    });
    this.pendingCallbackId.set(callback.callbackId);
    this.callbackAcknowledged.set(false);
    this.message.set('登录成功，认证结果已转交服务端，等待桌面客户端确认...');
    this.disposeCallbackTracking();
    this.callbackTracking = this.callbackService.startTracking({
      callbackId: callback.callbackId,
      onAcknowledged: () => {
        this.callbackAcknowledged.set(true);
        this.disposeCallbackTracking();
        this.error.set('');
        this.message.set('客户端已确认登录结果，请切回桌面端继续。');
      },
      onFallback: () => {
        this.error.set('客户端尚未确认登录结果，请确认桌面 app 已启动并保持在线。');
      }
    });
  }

  private clearNotices(): void {
    this.error.set('');
    this.message.set('');
  }

  private clearCachedAuth(): void {
    this.sessionService.clearCachedAuth();
    this.token.set('');
    this.userId.set('');
    this.autoCallbackAttempted = false;
    this.stopSessionMonitor();
  }

  private disposeCallbackTracking(): void {
    this.callbackTracking?.dispose();
    this.callbackTracking = null;
  }

  private setError(error: unknown): void {
    if (error instanceof ConsoleApiError && error.isUnauthorized) {
      this.handleUnauthorizedSession(error.message || 'session invalid');
      return;
    }
    this.error.set(error instanceof Error ? error.message : String(error));
    this.message.set('');
  }

  private handleUnauthorizedSession(message: string): void {
    this.clearCachedAuth();
    this.activeView.set('account');
    this.home.set({ hasNetwork: false });
    this.detail.set(null);
    this.subnets.set([]);
    this.assignments.set([]);
    this.currentDeviceId.set('');
    this.pendingCallbackId.set('');
    this.callbackAcknowledged.set(false);
    this.disposeCallbackTracking();
    this.error.set(message || 'session invalid');
    this.message.set('');
  }

  private currentDeviceState() {
    return {
      currentDeviceId: this.currentDeviceId(),
      callbackDeviceId: this.callbackDeviceId(),
      preferredMachineId: this.loginClientDeviceId(),
      clientPlatform: this.loginClientPlatform(),
      clientName: this.loginClientName(),
    };
  }

  private applyAuthentication(result: AuthenticateResult): void {
    this.token.set(result.auth.accessToken);
    this.userId.set(result.auth.userId);
    this.startSessionMonitor();
    this.applyManagedDeviceState(result.managedDevice);
  }

  private applyRefreshWorkspaceResult(result: RefreshWorkspaceResult): void {
    this.applyManagedDeviceState(result.managedDevice);
    this.home.set(result.workspace.home);
    this.detail.set(result.workspace.detail);
    this.subnets.set(result.workspace.subnets);
    this.assignments.set(result.workspace.assignments);
    this.draftIps.set(result.workspace.draftIps);
    this.draftRemarks.set(
      Object.fromEntries(result.workspace.assignments.map((item) => [item.attachmentId, item.remark || '']))
    );
    if (result.workspace.detail) {
      this.hydrateNetworkDrafts(result.workspace.detail);
    }
  }

  private parseList(value: string): string[] {
    return value
      .split(/\r?\n|,/)
      .map((item) => item.trim())
      .filter(Boolean);
  }

  private applyManagedDeviceState(managedDevice: { devices: Device[]; currentDeviceId: string; callbackDeviceId: string }): void {
    this.devices.set(managedDevice.devices);
    this.currentDeviceId.set(managedDevice.currentDeviceId);
    this.callbackDeviceId.set(managedDevice.callbackDeviceId);
  }

  private startSessionMonitor(): void {
    if (this.sessionCheckTimer !== null || !this.token()) {
      return;
    }
    this.sessionCheckTimer = window.setInterval(() => {
      void this.checkSessionStillValid();
    }, AppComponent.SESSION_CHECK_INTERVAL_MS);
  }

  private stopSessionMonitor(): void {
    if (this.sessionCheckTimer !== null) {
      window.clearInterval(this.sessionCheckTimer);
      this.sessionCheckTimer = null;
    }
    this.sessionCheckInFlight = false;
  }

  private async checkSessionStillValid(): Promise<void> {
    const token = this.token();
    if (!token || this.sessionCheckInFlight) {
      return;
    }
    this.sessionCheckInFlight = true;
    try {
      await this.api.getHome(token);
    } catch (error) {
      if (error instanceof ConsoleApiError && error.isUnauthorized) {
        this.handleUnauthorizedSession(error.message || 'session invalid');
      }
    } finally {
      this.sessionCheckInFlight = false;
    }
  }
}
