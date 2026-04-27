import { CommonModule } from '@angular/common';
import { Component, OnDestroy, computed, inject, signal } from '@angular/core';

import { AuthResponse, Device, NetworkAssignment, NetworkDetail, NetworkHome, NetworkMember, PurchaseOrder, PurchaseProduct, Subnet } from './api-contracts';
import { AuthPanelComponent } from './auth-panel.component';
import { ConsoleApiError, ConsoleApiService } from './console-api.service';
import { AuthenticateResult, ConsoleAppFacadeService, RefreshWorkspaceResult } from './console-app-facade.service';
import { CallbackTrackingHandle, ConsoleCallbackService } from './console-callback.service';
import { ConsoleNetworkFormService } from './console-network-form.service';
import { ConsoleSessionService } from './console-session.service';
import { ConsoleWorkspaceService } from './console-workspace.service';
import { NetworkEmptyStateComponent } from './network-empty-state.component';
import { NetworkWorkspaceComponent } from './network-workspace.component';
import { AuthMode, ConsoleView } from './ui-models';

@Component({
  selector: 'app-root',
  standalone: true,
  imports: [
    CommonModule,
    AuthPanelComponent,
    NetworkEmptyStateComponent,
    NetworkWorkspaceComponent
  ],
  templateUrl: './app.component.html',
  styleUrl: './app.component.css'
})
export class AppComponent implements OnDestroy {
  private static readonly SESSION_CHECK_INTERVAL_MS = 30_000;
  readonly navItems: Array<{ id: ConsoleView; label: string; caption: string }> = [
    { id: 'account', label: '账户概览', caption: '账号、设备、接入状态' },
    { id: 'network', label: '网络管理', caption: 'IP 绑定、设备接入和邀请码管理' }
    ,
    { id: 'orders', label: '订单管理', caption: '设备扩容和 DNS 服务订单' }
  ];
  private readonly facade = inject(ConsoleAppFacadeService);
  private readonly callbackService = inject(ConsoleCallbackService);
  private readonly networkFormService = inject(ConsoleNetworkFormService);
  private readonly api = inject(ConsoleApiService);
  private readonly sessionService = inject(ConsoleSessionService);
  private readonly workspaceService = inject(ConsoleWorkspaceService);
  email = '';
  password = '';
  currentPassword = '';
  newPassword = '';
  confirmPassword = '';
  createName = 'My Network';
  createDescription = '';
  createNetworkIp = '10.0.0.0';
  createSubnetMask = '255.255.252.0';
  manageNetworkIp = '10.0.0.0';
  manageSubnetMask = '255.255.255.0';
  manageAllocationStartIp = '';
  manageAllocationEndIp = '';
  createAllocationStartIp = '';
  createAllocationEndIp = '';
  joinKey = '';
  generatedInviteCode = '';
  assignmentSearch = '';
  assignmentRoleFilter = 'all';
  readonly pageSize = 10;
  updateName = '';
  updateDescription = '';
  updateCidr = '';
  networkJoinKey = '';
  dnsDocumentText = '';
  purchaseQuantityValue = 1;
  purchaseMonthsValue = 1;

  readonly token = signal(localStorage.getItem('slan.accessToken') || '');
  readonly userId = signal(localStorage.getItem('slan.userId') || '');
  readonly userEmail = signal(localStorage.getItem('slan.userEmail') || '');
  readonly authMode = signal<AuthMode>('login');
  readonly activeView = signal<ConsoleView>('account');
  readonly networkDialog = signal<'create' | 'join' | 'current' | 'invite' | 'manage' | 'dns' | 'purchase' | ''>('');
  readonly passwordDialog = signal(false);
  readonly loading = signal(false);
  readonly actionBusy = signal('');
  readonly error = signal('');
  readonly message = signal('');
  readonly home = signal<NetworkHome>({ hasNetwork: false });
  readonly networks = signal<NonNullable<NetworkHome['activeNetwork']>[]>([]);
  readonly detail = signal<NetworkDetail | null>(null);
  readonly assignments = signal<NetworkAssignment[]>([]);
  readonly devices = signal<Device[]>([]);
  readonly subnets = signal<Subnet[]>([]);
  readonly purchaseProducts = signal<PurchaseProduct[]>([]);
  readonly purchaseOrders = signal<PurchaseOrder[]>([]);
  readonly purchaseOrder = signal<PurchaseOrder | null>(null);
  readonly assignmentPage = signal(1);
  readonly purchaseOrderPage = signal(1);
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
    const items = [
      ...this.networks(),
      this.home().activeNetwork,
      this.home().ownedNetwork,
    ].filter(Boolean) as NonNullable<NetworkHome['activeNetwork']>[];
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
  readonly pagedAssignments = computed(() => this.pageRows(this.filteredAssignments(), this.assignmentPage()));
  readonly pagedPurchaseOrders = computed(() => this.pageRows(this.purchaseOrders(), this.purchaseOrderPage()));
  readonly pendingMembers = computed((): NetworkMember[] => {
    return (this.detail()?.members || []).filter((item) => item.status === 'pending');
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

  onlineDeviceCount(): number {
    return this.devices().filter((device) => this.deviceStatusTone(device) === 'success').length;
  }

  currentVirtualIp(): string {
    return this.currentAssignment()?.virtualIp || '未分配';
  }

  currentSubnetLabel(): string {
    const subnet = this.currentSubnet();
    if (!subnet) {
      return '未接入网络';
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

  switchView(view: ConsoleView): void {
    this.activeView.set(view);
    if (view === 'orders') {
      this.purchaseOrderPage.set(1);
      void this.loadPurchaseOrders();
    }
  }

  setAssignmentSearch(value: string): void {
    this.assignmentSearch = value;
    this.assignmentPage.set(1);
  }

  setAssignmentRoleFilter(value: string): void {
    this.assignmentRoleFilter = value;
    this.assignmentPage.set(1);
  }

  pageSummary(page: number, total: number): string {
    if (!total) {
      return '0 / 0';
    }
    const current = Math.min(Math.max(1, page), this.totalPages(total));
    const start = (current - 1) * this.pageSize + 1;
    const end = Math.min(current * this.pageSize, total);
    return `${start}-${end} / ${total}`;
  }

  totalPages(total: number): number {
    return Math.max(1, Math.ceil(total / this.pageSize));
  }

  previousAssignmentPage(): void {
    this.assignmentPage.set(Math.max(1, this.assignmentPage() - 1));
  }

  nextAssignmentPage(): void {
    this.assignmentPage.set(Math.min(this.totalPages(this.filteredAssignments().length), this.assignmentPage() + 1));
  }

  previousPurchaseOrderPage(): void {
    this.purchaseOrderPage.set(Math.max(1, this.purchaseOrderPage() - 1));
  }

  nextPurchaseOrderPage(): void {
    this.purchaseOrderPage.set(Math.min(this.totalPages(this.purchaseOrders().length), this.purchaseOrderPage() + 1));
  }

  openCurrentNetworkDialog(): void {
    this.clearNotices();
    this.networkDialog.set('current');
  }

  async openDnsNetworkDialog(): Promise<void> {
    this.clearNotices();
    if (!this.canManageNetwork()) {
      this.error.set('只有当前网络 owner 可以管理 DNS。');
      return;
    }
    const allowed = await this.ensureDnsEntitlement();
    if (!allowed) {
      return;
    }
    this.networkDialog.set('dns');
  }

  openInviteNetworkDialog(): void {
    this.clearNotices();
    if (!this.canManageNetwork()) {
      this.error.set('只有当前网络 owner 可以邀请用户入网。');
      return;
    }
    this.generatedInviteCode = this.networkJoinKey.trim();
    this.networkDialog.set('invite');
  }

  networkDialogTitle(): string {
    switch (this.networkDialog()) {
      case 'create':
        return '网络规划';
      case 'current':
        return '网络切换';
      case 'invite':
        return '邀请入网';
      case 'manage':
        return '网络管理';
      case 'dns':
        return 'DNS 管理';
      case 'purchase':
        return '购买下单';
      default:
        return '加入网络';
    }
  }

  async openPurchaseDialog(productCode = 'extra-device'): Promise<void> {
    this.purchaseOrder.set(null);
    this.purchaseQuantityValue = 1;
    this.purchaseMonthsValue = 1;
    this.networkDialog.set('purchase');
    await this.loadPurchaseProducts(productCode);
  }

  async loadPurchaseOrders(): Promise<void> {
    if (!this.token()) {
      return;
    }
    this.actionBusy.set('loadOrders');
    try {
      const result = await this.api.listPurchaseOrders(this.token());
      this.purchaseOrders.set(result.items || []);
      if (this.purchaseOrderPage() > this.totalPages(this.purchaseOrders().length)) {
        this.purchaseOrderPage.set(this.totalPages(this.purchaseOrders().length));
      }
    } catch (error) {
      this.setError(error);
    } finally {
      this.actionBusy.set('');
    }
  }

  openCreateNetworkDialog(): void {
    this.clearNotices();
    if (this.home().ownedNetwork) {
      this.error.set('当前账户已经拥有自己的网络，不能重复创建。');
      return;
    }
    this.networkDialog.set('create');
  }

  openManageNetworkDialog(): void {
    this.clearNotices();
    if (!this.canManageNetwork()) {
      this.error.set('只有当前网络 owner 可以管理网络。');
      return;
    }
    const cidr = this.activeNetwork()?.defaultSubnetCidr || this.updateCidr || '';
    const plan = this.networkFormService.addressAndMaskFromCidr(cidr);
    const subnet = this.currentSubnet() || this.subnets().find((item) => item.isDefault) || null;
    this.manageNetworkIp = plan.address;
    this.manageSubnetMask = plan.subnetMask;
    this.manageAllocationStartIp = subnet?.allocationStartIp || '';
    this.manageAllocationEndIp = subnet?.allocationEndIp || '';
    this.networkDialog.set('manage');
  }

  openJoinNetworkDialog(): void {
    this.clearNotices();
    if (this.home().hasNetwork && !this.canManageNetwork()) {
      this.error.set('只有当前网络 owner 可以进行入网确认。');
      return;
    }
    this.networkDialog.set('join');
  }

  closeNetworkDialog(): void {
    if (this.actionBusy()) {
      return;
    }
    this.networkDialog.set('');
  }

  openPasswordDialog(): void {
    this.clearNotices();
    this.currentPassword = '';
    this.newPassword = '';
    this.confirmPassword = '';
    this.passwordDialog.set(true);
  }

  closePasswordDialog(): void {
    if (this.actionBusy()) {
      return;
    }
    this.passwordDialog.set(false);
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

  async createOwnNetwork(): Promise<void> {
    this.clearNotices();
    if (this.home().ownedNetwork) {
      this.error.set('当前账户已经拥有自己的网络，不能重复创建。');
      this.networkDialog.set('');
      return;
    }
    this.actionBusy.set('createNetwork');
    try {
      const result = await this.facade.createOwnNetwork({
        token: this.token(),
        createName: this.createName,
        createDescription: this.createDescription,
        createNetworkIp: this.createNetworkIp,
        createSubnetMask: this.createSubnetMask,
        allocationStartIp: this.createAllocationStartIp,
        allocationEndIp: this.createAllocationEndIp,
        deviceState: this.currentDeviceState(),
      });
      this.applyRefreshWorkspaceResult(result.refreshed);
      this.networkDialog.set('');
      this.message.set(`created network ${result.network.name}, created default DHCP plan and bound the current device`);
    } catch (error) {
      this.handleActionError(error);
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
      this.networkDialog.set('');
      this.message.set('已通过邀请码提交加入申请，请等待网络 owner 审批');
    } catch (error) {
      this.setError(error);
    } finally {
      this.actionBusy.set('');
    }
  }

  async generateInviteCode(): Promise<void> {
    this.clearNotices();
    const active = this.activeNetwork();
    if (!active) {
      return;
    }
    this.actionBusy.set('generateInviteCode');
    try {
      const result = await this.facade.updateNetworkJoinKey({
        token: this.token(),
        networkId: active.networkId,
        joinKey: '',
        deviceState: this.currentDeviceState(),
      });
      this.applyRefreshWorkspaceResult(result);
      this.generatedInviteCode = this.detail()?.joinKey || this.networkJoinKey;
    } catch (error) {
      this.handleActionError(error);
    } finally {
      this.actionBusy.set('');
    }
  }

  inviteQrUrl(): string {
    const code = this.generatedInviteCode.trim();
    if (!code) {
      return '';
    }
    return `https://api.qrserver.com/v1/create-qr-code/?size=196x196&margin=12&data=${encodeURIComponent(code)}`;
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
        allocationStartIp: this.manageAllocationStartIp,
        allocationEndIp: this.manageAllocationEndIp,
        deviceState: this.currentDeviceState(),
      });
      this.applyRefreshWorkspaceResult(result);
      this.message.set('network updated, clients should restart tunnel to pick up the new network plan');
      if (this.networkDialog() === 'manage') {
        this.networkDialog.set('');
      }
    } catch (error) {
      this.setError(error);
    }
  }

  async updateManagedNetworkPlan(): Promise<void> {
    this.updateCidr = this.networkFormService.cidrFromAddressAndMask(this.manageNetworkIp, this.manageSubnetMask);
    await this.updateOwnedNetwork();
  }

  async updateNetworkDns(): Promise<void> {
    this.clearNotices();
    const active = this.activeNetwork();
    if (!active) {
      return;
    }
    const allowed = await this.ensureDnsEntitlement();
    if (!allowed) {
      return;
    }
    this.actionBusy.set('saveDns');
    try {
      const result = await this.facade.updateNetworkDns({
        token: this.token(),
        networkId: active.networkId,
        ...this.parseDnsDocument(this.dnsDocumentText),
        deviceState: this.currentDeviceState(),
      });
      this.applyRefreshWorkspaceResult(result);
      this.message.set('DNS 配置已保存，客户端下次启用网络时会按配置启动本地 DNS');
      if (this.networkDialog() === 'dns') {
        this.networkDialog.set('');
      }
    } catch (error) {
      if (error instanceof ConsoleApiError && error.isPaymentRequired) {
        void this.openPurchaseDialog('dns');
        this.error.set('DNS 管理需要购买 DNS 服务，或当前 DNS 服务已过期。');
      } else {
        this.setError(error);
      }
    } finally {
      this.actionBusy.set('');
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

  async updateAttachmentEdit(input: { attachmentId: string; virtualIp: string; remark: string }): Promise<void> {
    const active = this.activeNetwork();
    if (!active) {
      return;
    }
    const virtualIp = input.virtualIp.trim();
    const remark = input.remark.trim();
    this.setDraftIp(input.attachmentId, virtualIp);
    this.setDraftRemark(input.attachmentId, remark);
    try {
      let result = await this.facade.updateAttachmentIp({
        token: this.token(),
        networkId: active.networkId,
        attachmentId: input.attachmentId,
        virtualIp,
        deviceState: this.currentDeviceState(),
      });
      this.applyRefreshWorkspaceResult(result);
      result = await this.facade.updateAttachmentRemark({
        token: this.token(),
        networkId: active.networkId,
        attachmentId: input.attachmentId,
        remark,
        deviceState: this.currentDeviceState(),
      });
      this.applyRefreshWorkspaceResult(result);
      this.message.set('设备绑定已保存');
    } catch (error) {
      this.setError(error);
    }
  }

  async updateMemberStatus(memberId: string, status: 'active' | 'rejected'): Promise<void> {
    const active = this.activeNetwork();
    if (!active) {
      return;
    }
    if (this.actionBusy().startsWith(`member:${memberId}:`)) {
      return;
    }
    this.actionBusy.set(`member:${memberId}:${status}`);
    try {
      const result = await this.facade.updateNetworkMemberStatus({
        token: this.token(),
        networkId: active.networkId,
        memberId,
        status,
        deviceState: this.currentDeviceState(),
      });
      this.applyRefreshWorkspaceResult(result);
      this.message.set(status === 'active' ? '加入申请已通过' : '加入申请已拒绝');
    } catch (error) {
      this.handleActionError(error);
    } finally {
      this.actionBusy.set('');
    }
  }

  async createExtraDeviceOrder(): Promise<void> {
    await this.createOrder('extra-device', this.purchaseQuantityValue, 1);
  }

  async createDnsOrder(): Promise<void> {
    await this.createOrder('dns', 1, this.purchaseMonthsValue);
  }

  async createPurchaseProductOrder(product: PurchaseProduct): Promise<void> {
    if (this.isDnsProduct(product)) {
      await this.createOrder(product.productCode, 1, this.purchaseMonthsValue);
      return;
    }
    await this.createOrder(product.productCode, this.purchaseQuantityValue, 1);
  }

  isDeviceAddonProduct(product: PurchaseProduct): boolean {
    return !this.isDnsProduct(product);
  }

  isDnsProduct(product: PurchaseProduct): boolean {
    return product.productType === 'addon_dns' || product.productCode === 'dns';
  }

  async changePassword(): Promise<void> {
    this.clearNotices();
    if (this.newPassword !== this.confirmPassword) {
      this.error.set('两次输入的新密码不一致。');
      return;
    }
    this.actionBusy.set('changePassword');
    try {
      await this.api.changePassword(this.token(), {
        currentPassword: this.currentPassword,
        newPassword: this.newPassword,
      });
      this.passwordDialog.set(false);
      this.currentPassword = '';
      this.newPassword = '';
      this.confirmPassword = '';
      this.message.set('密码已修改，请妥善保存新密码。');
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

  formatMoney(cents: number, currency = 'CNY'): string {
    const amount = (Number(cents || 0) / 100).toFixed(2);
    return currency === 'CNY' ? `¥${amount}` : `${currency} ${amount}`;
  }

  deviceProtocol(device: Device): string {
    return device.connectivityProtocol || '-';
  }

  orderQuantityLabel(order: PurchaseOrder): string {
    const duration = this.orderDurationLabel(order.billingCycle, order.months);
    if (order.productCode === 'dns' || order.productType === 'addon_dns') {
      return duration;
    }
    return `${Math.max(1, Number(order.quantity || 1))} 台 / ${duration}`;
  }

  orderDurationLabel(billingCycle = 'month', months = 1): string {
    const count = Math.max(1, Number(months || 1));
    switch ((billingCycle || '').toLowerCase()) {
      case 'quarter':
      case 'quarterly':
        return `${count} 季度`;
      case 'year':
      case 'yearly':
      case 'annual':
        return `${count} 年`;
      case 'day':
      case 'daily':
        return `${count} 天`;
      default:
        return `${count} 个月`;
    }
  }

  setPurchaseMonths(value: string | number): void {
    const months = Number(value);
    this.purchaseMonthsValue = Number.isFinite(months) && months > 0 ? Math.floor(months) : 1;
  }

  setPurchaseQuantity(value: string | number): void {
    const quantity = Number(value);
    this.purchaseQuantityValue = Number.isFinite(quantity) && quantity > 0 ? Math.floor(quantity) : 1;
  }

  deviceStatusLabel(device: Device): string {
    if (device.networkState?.networkOnline) {
      return '网络在线';
    }
    if (device.networkState?.controlReachable) {
      return '控制可达';
    }
    const value = (device.linkStatus || device.status || '').toLowerCase();
    switch (value) {
      case 'connected':
      case 'reachable':
        return '控制可达';
      case 'online':
        return '控制可达';
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

  deviceControlLabel(device: Device): string {
    if (device.networkState) {
      return device.networkState.controlReachable ? '可达' : '不可达';
    }
    const value = (device.status || '').toLowerCase();
    return value === 'online' || value === 'connected' || value === 'reachable' ? '可达' : '未知';
  }

  deviceControlTone(device: Device): string {
    if (device.networkState?.controlReachable) {
      return 'success';
    }
    const value = (device.status || '').toLowerCase();
    return value === 'online' || value === 'connected' || value === 'reachable' ? 'success' : 'muted';
  }

  deviceNetworkLabel(device: Device): string {
    if (!device.networkState) {
      return this.deviceStatusLabel(device);
    }
    return device.networkState.networkOnline ? '已启用' : '已停用';
  }

  deviceNetworkTone(device: Device): string {
    if (device.networkState?.networkOnline) {
      return 'success';
    }
    if (device.networkState?.controlReachable) {
      return 'warn';
    }
    return 'muted';
  }

  deviceTunnelLabel(device: Device): string {
    const state = device.networkState;
    if (!state) {
      return this.deviceProtocol(device);
    }
    if (state.tunnelUp && state.lastProbeOk) {
      return '健康';
    }
    if (state.tunnelUp) {
      return '隧道已启动';
    }
    return '未启用';
  }

  deviceTunnelTone(device: Device): string {
    const state = device.networkState;
    if (!state) {
      return 'muted';
    }
    if (state.tunnelUp && state.lastProbeOk) {
      return 'success';
    }
    if (state.tunnelUp || state.controlReachable) {
      return 'warn';
    }
    return 'muted';
  }

  deviceLastSeenLabel(device: Device): string {
    return device.networkState?.lastSeenAt ? this.formatTimestamp(device.networkState.lastSeenAt) : '-';
  }

  membershipStatusTone(device: Device): string {
    const value = (device.membershipStatus || '').toLowerCase();
    switch (value) {
      case 'active':
        return 'success';
      case 'pending':
      case 'joining':
        return 'warn';
      case 'rejected':
        return 'danger';
      default:
        return 'muted';
    }
  }

  deviceStatusTone(device: Device): string {
    if (device.networkState?.networkOnline) {
      return 'success';
    }
    if (device.networkState?.controlReachable) {
      return 'warn';
    }
    const value = (device.linkStatus || device.status || '').toLowerCase();
    switch (value) {
      case 'connected':
      case 'online':
      case 'reachable':
        return 'warn';
      case 'disconnected':
        return 'warn';
      case 'offline':
      default:
        return 'muted';
    }
  }

  logout(): void {
    this.clearCachedAuth();
    this.activeView.set('account');
    this.home.set({ hasNetwork: false });
    this.networks.set([]);
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
    this.clearNotices();
    try {
      const result = await this.facade.refreshWorkspace({
        token: this.token(),
        ...this.currentDeviceState(),
      });
      this.applyManagedDeviceState(result.managedDevice);
      await this.workspaceService.switchNetwork(this.token(), networkId, result.managedDevice.deviceId);
      await this.workspaceService.activateNetwork(this.token(), networkId, result.managedDevice.deviceId);
      const switched = await this.facade.refreshWorkspace({
        token: this.token(),
        ...this.currentDeviceState(),
      });
      this.applyRefreshWorkspaceResult(switched);
      this.message.set(`已切换到网络 ${switched.workspace.home.activeNetwork?.name || networkId}`);
    } catch (error) {
      this.handleActionError(error);
    }
  }

  private hydrateNetworkDrafts(detail: NetworkDetail): void {
    const draft = this.networkFormService.buildNetworkUpdateDraft(detail);
    this.updateName = draft.name;
    this.updateDescription = draft.description;
    this.updateCidr = draft.cidr;
    this.networkJoinKey = detail.joinKey || '';
    this.dnsDocumentText = this.buildDnsDocument(detail);
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

  private handleActionError(error: unknown): void {
    if (error instanceof ConsoleApiError && error.isPaymentRequired) {
      void this.openPurchaseDialog('extra-device');
      this.error.set('免费套餐最多支持 2 台设备同时接入，请先购买附加设备后再继续。');
      return;
    }
    this.setError(error);
  }

  private async ensureDnsEntitlement(): Promise<boolean> {
    if (!this.token()) {
      return false;
    }
    this.actionBusy.set('checkDnsEntitlement');
    try {
      const entitlement = await this.api.getProductEntitlement(this.token(), 'dns');
      if (entitlement.active) {
        return true;
      }
      await this.openPurchaseDialog('dns');
      this.error.set('DNS 管理需要购买 DNS 服务，或当前 DNS 服务已过期。');
      return false;
    } catch (error) {
      this.setError(error);
      return false;
    } finally {
      if (this.actionBusy() === 'checkDnsEntitlement') {
        this.actionBusy.set('');
      }
    }
  }

  private async loadPurchaseProducts(preferredProductCode = 'extra-device'): Promise<void> {
    if (!this.token()) {
      return;
    }
    this.actionBusy.set('loadProducts');
    try {
      const result = await this.api.listPurchaseProducts(this.token());
      const items = (result.items || []).filter((item) => {
        if (preferredProductCode === 'extra-device') {
          return item.productType === 'addon_device';
        }
        return item.productCode === preferredProductCode;
      });
      this.purchaseProducts.set(items);
    } catch (error) {
      this.setError(error);
    } finally {
      this.actionBusy.set('');
    }
  }

  private async createOrder(productCode: string, quantity = 1, months = 1): Promise<void> {
    if (!this.token()) {
      return;
    }
    this.actionBusy.set(`order:${productCode}`);
    try {
      const order = await this.api.createPurchaseOrder(this.token(), productCode, quantity, months);
      this.purchaseOrder.set(order);
      await this.loadPurchaseOrders();
      this.message.set(`订单已生成：${order.orderId}`);
    } catch (error) {
      this.setError(error);
    } finally {
      this.actionBusy.set('');
    }
  }

  private handleUnauthorizedSession(message: string): void {
    this.clearCachedAuth();
    this.activeView.set('account');
    this.home.set({ hasNetwork: false });
    this.networks.set([]);
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
    this.userEmail.set(result.auth.email || this.email.trim());
    if (this.userEmail()) {
      localStorage.setItem('slan.userEmail', this.userEmail());
    }
    this.startSessionMonitor();
    this.applyManagedDeviceState(result.managedDevice);
  }

  private applyRefreshWorkspaceResult(result: RefreshWorkspaceResult): void {
    this.applyManagedDeviceState(result.managedDevice);
    this.home.set(result.workspace.home);
    this.networks.set(result.workspace.networks);
    this.detail.set(result.workspace.detail);
    this.subnets.set(result.workspace.subnets);
    this.assignments.set(result.workspace.assignments);
    if (this.assignmentPage() > this.totalPages(result.workspace.assignments.length)) {
      this.assignmentPage.set(this.totalPages(result.workspace.assignments.length));
    }
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

  private pageRows<T>(rows: T[], page: number): T[] {
    const current = Math.min(Math.max(1, page), this.totalPages(rows.length));
    const start = (current - 1) * this.pageSize;
    return rows.slice(start, start + this.pageSize);
  }

  private buildDnsDocument(detail: NetworkDetail): string {
    const dns = detail.dns || { servers: [], searchDomains: [], wildcards: [] };
    return [
      ...(dns.servers || []).map((item) => `server ${item}`),
      ...(dns.searchDomains || []).map((item) => `domain ${item}`),
      ...(dns.wildcards || []).map((item) => `wildcard ${item}`),
    ].join('\n');
  }

  private parseDnsDocument(value: string): { servers: string[]; searchDomains: string[]; wildcards: string[] } {
    const servers: string[] = [];
    const searchDomains: string[] = [];
    const wildcards: string[] = [];
    for (const raw of value.split(/\r?\n/)) {
      const line = raw.trim();
      if (!line || line.startsWith('#')) {
        continue;
      }
      const match = line.match(/^(server|dns|domain|search|wildcard)\s+(.+)$/i);
      if (!match) {
        throw new Error('DNS 配置格式错误，请使用 server/domain/wildcard 开头');
      }
      const key = match[1].toLowerCase();
      const payload = match[2].trim();
      if (!payload) {
        continue;
      }
      if (key === 'server' || key === 'dns') {
        servers.push(payload);
      } else if (key === 'domain' || key === 'search') {
        searchDomains.push(payload);
      } else {
        wildcards.push(payload);
      }
    }
    return { servers, searchDomains, wildcards };
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
