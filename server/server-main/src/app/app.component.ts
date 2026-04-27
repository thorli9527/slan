import { CommonModule } from '@angular/common';
import { Component, computed, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';

type ViewKey = 'overview' | 'users' | 'devices' | 'products' | 'orders' | 'admins' | 'roles' | 'menus' | 'relays';
type PagedViewKey = Exclude<ViewKey, 'overview' | 'relays'>;

type OpsOverview = {
  userCount: number;
  deviceCount: number;
  onlineDeviceCount: number;
  nodeCount: number;
  relayClusterCount: number;
  relayNodeCount: number;
  defaultAdminSeeded: boolean;
  defaultAdminLoginName?: string;
  defaultAdminRoleBound: boolean;
  securityWarnings?: string[];
};

type OpsLoginResponse = {
  adminId: string;
  userId: string;
  loginName: string;
  displayName: string;
  accessToken: string;
  expiresIn: number;
};

type OpsProduct = {
  productId?: string;
  merchantId?: string;
  merchantName?: string;
  productCode: string;
  productName: string;
  description?: string;
  productType: string;
  priceCents: number;
  currency: string;
  billingCycle: string;
  unitQuantity: number;
  maxActiveDevices: number;
  bandwidthLimitMbps: number;
  isDefault: boolean;
  status: string;
};

type OpsPurchaseOrder = {
  orderId: string;
  userId: string;
  userEmail?: string;
  merchantId?: string;
  merchantName?: string;
  productId?: string;
  productCode: string;
  productName: string;
  productType?: string;
  quantity: number;
  months: number;
  unitCents?: number;
  amountCents: number;
  currency: string;
  billingCycle: string;
  status: string;
  paidAt?: number;
  cancelledAt?: number;
  refundedAt?: number;
  expiresAt?: number;
  createdAt?: number;
  updatedAt?: number;
};

type PaidOrderDraft = {
  userId: string;
  productCode: string;
  quantity: number;
  months: number;
};

type OpsUser = {
  userId: string;
  email: string;
  deviceCount: number;
  nodeCount: number;
  roleCodes?: string[];
};

type OpsDevice = {
  deviceId: string;
  name: string;
  platform: string;
  status: string;
  userId: string;
  nodeCount: number;
};

type OpsAdmin = {
  adminId?: string;
  userId: string;
  loginName: string;
  email?: string;
  displayName: string;
  phone?: string;
  title?: string;
  department?: string;
  status: string;
  roleCodes?: string[];
  usingSeedPassword?: boolean;
};

type OpsRole = {
  roleId: string;
  roleCode: string;
  roleName: string;
  description?: string;
  builtin: boolean;
  menuCodes?: string[];
};

type OpsMenu = {
  menuId: string;
  menuCode: string;
  menuName: string;
  path?: string;
  parentId?: string;
  sort: number;
  status: string;
};

type OpsRelayTopology = {
  defaultClusterId: string;
  regions?: Array<{
    regionId: string;
    regionName: string;
    countryName?: string;
    cityName?: string;
    clusterId?: string;
    clusterName?: string;
    endpoints?: Array<{ endpointId: string; transport: string; address: string }>;
  }>;
  nodes?: Array<{
    nodeId: string;
    clusterId: string;
    clusterName: string;
    countryName?: string;
    cityName?: string;
    transport: string;
    address: string;
    priority: number;
    observedRttMs?: number;
    packetLossPpm?: number;
    pathScore?: number;
  }>;
};

@Component({
  selector: 'slan-root',
  standalone: true,
  imports: [CommonModule, FormsModule],
  templateUrl: './app.component.html',
  styleUrl: './app.component.css',
})
export class AppComponent {
  readonly views: Array<{ key: ViewKey; label: string; caption: string }> = [
    { key: 'overview', label: '运营概览', caption: '核心指标与告警' },
    { key: 'users', label: '用户管理', caption: '用户、角色与设备数' },
    { key: 'devices', label: '设备管理', caption: '设备、节点和在线状态' },
    { key: 'products', label: '商品管理', caption: '套餐、附加设备和 DNS 价格' },
    { key: 'orders', label: '订单管理', caption: '支付确认与权益开通' },
    { key: 'admins', label: '管理员', caption: '运营账号资料' },
    { key: 'roles', label: '角色权限', caption: 'RBAC 角色' },
    { key: 'menus', label: '菜单权限', caption: '运营菜单' },
    { key: 'relays', label: 'Relay 拓扑', caption: '中继节点健康' },
  ];

  readonly token = signal(localStorage.getItem('slan.opsToken') || '');
  readonly adminId = signal(localStorage.getItem('slan.opsAdminId') || '');
  readonly adminName = signal(localStorage.getItem('slan.opsAdminName') || '');
  readonly activeView = signal<ViewKey>('overview');
  readonly loading = signal(false);
  readonly error = signal('');
  readonly message = signal('');
  readonly overview = signal<OpsOverview | null>(null);
  readonly users = signal<OpsUser[]>([]);
  readonly devices = signal<OpsDevice[]>([]);
  readonly products = signal<OpsProduct[]>([]);
  readonly orders = signal<OpsPurchaseOrder[]>([]);
  readonly admins = signal<OpsAdmin[]>([]);
  readonly roles = signal<OpsRole[]>([]);
  readonly menus = signal<OpsMenu[]>([]);
  readonly relays = signal<OpsRelayTopology | null>(null);
  readonly keyword = signal('');
  readonly pageSize = 10;
  readonly listPages = signal<Record<PagedViewKey, number>>({
    users: 1,
    devices: 1,
    products: 1,
    orders: 1,
    admins: 1,
    roles: 1,
    menus: 1,
  });
  readonly selectedProduct = signal<OpsProduct | null>(null);
  readonly productDraft = signal<OpsProduct>(emptyProduct());
  readonly paidOrderDraft = signal<PaidOrderDraft>({ userId: '', productCode: 'extra-device', quantity: 1, months: 1 });
  readonly paidOrderUser = signal<OpsUser | null>(null);

  loginName = 'admin';
  password = '';
  emergencyToken = '';
  opsNewPassword = '';
  opsConfirmPassword = '';
  showPasswordPanel = false;

  readonly filteredUsers = computed(() => filterRows(this.users(), this.keyword()));
  readonly filteredDevices = computed(() => filterRows(this.devices(), this.keyword()));
  readonly filteredProducts = computed(() => filterRows(this.products(), this.keyword()));
  readonly filteredOrders = computed(() => filterRows(this.orders(), this.keyword()));
  readonly filteredAdmins = computed(() => filterRows(this.admins(), this.keyword()));
  readonly pagedUsers = computed(() => this.pageRows('users', this.filteredUsers()));
  readonly pagedDevices = computed(() => this.pageRows('devices', this.filteredDevices()));
  readonly pagedProducts = computed(() => this.pageRows('products', this.filteredProducts()));
  readonly pagedOrders = computed(() => this.pageRows('orders', this.filteredOrders()));
  readonly pagedAdmins = computed(() => this.pageRows('admins', this.filteredAdmins()));
  readonly pagedRoles = computed(() => this.pageRows('roles', this.roles()));
  readonly pagedMenus = computed(() => this.pageRows('menus', this.menus()));
  readonly activeViewLabel = computed(() => this.views.find((item) => item.key === this.activeView())?.label || '运营后台');

  constructor() {
    if (this.token()) {
      void this.refreshAll();
    }
  }

  async login(): Promise<void> {
    this.clearNotices();
    if (this.emergencyToken.trim()) {
      this.setToken(this.emergencyToken.trim());
      await this.refreshAll();
      return;
    }
    this.loading.set(true);
    try {
      const resp = await this.request<OpsLoginResponse>('/login', {
        method: 'POST',
        body: JSON.stringify({ loginName: this.loginName.trim(), password: this.password }),
      }, false);
      this.setToken(resp.accessToken, resp.adminId, resp.loginName);
      await this.refreshAll();
    } catch (error) {
      this.setError(error);
    } finally {
      this.loading.set(false);
    }
  }

  logout(): void {
    localStorage.removeItem('slan.opsToken');
    localStorage.removeItem('slan.opsAdminId');
    localStorage.removeItem('slan.opsAdminName');
    this.token.set('');
    this.adminId.set('');
    this.adminName.set('');
    this.password = '';
    this.error.set('');
    this.message.set('');
  }

  async changeOwnPassword(): Promise<void> {
    this.clearNotices();
    if (!this.adminId()) {
      this.error.set('当前使用应急 Token 登录，不能修改个人管理员密码。');
      return;
    }
    if (this.opsNewPassword.length < 8) {
      this.error.set('新密码至少 8 位。');
      return;
    }
    if (this.opsNewPassword !== this.opsConfirmPassword) {
      this.error.set('两次输入的新密码不一致。');
      return;
    }
    this.loading.set(true);
    try {
      await this.request(`/admins/${encodeURIComponent(this.adminId())}/password`, {
        method: 'PUT',
        body: JSON.stringify({ password: this.opsNewPassword }),
      });
      this.opsNewPassword = '';
      this.opsConfirmPassword = '';
      this.showPasswordPanel = false;
      this.message.set('运营管理员密码已修改。');
    } catch (error) {
      this.setError(error);
    } finally {
      this.loading.set(false);
    }
  }

  async switchView(view: ViewKey): Promise<void> {
    this.activeView.set(view);
    this.keyword.set('');
    if (this.isPagedView(view)) {
      this.setListPage(view, 1);
    }
    await this.refreshView(view);
  }

  setKeyword(value: string): void {
    this.keyword.set(value);
    const view = this.activeView();
    if (this.isPagedView(view)) {
      this.setListPage(view, 1);
    }
  }

  pageSummary(view: PagedViewKey, total: number): string {
    if (!total) {
      return '0 / 0';
    }
    const page = this.currentPage(view, total);
    const start = (page - 1) * this.pageSize + 1;
    const end = Math.min(page * this.pageSize, total);
    return `${start}-${end} / ${total}`;
  }

  canPreviousPage(view: PagedViewKey): boolean {
    return (this.listPages()[view] || 1) > 1;
  }

  canNextPage(view: PagedViewKey, total: number): boolean {
    return (this.listPages()[view] || 1) < this.totalPages(total);
  }

  previousPage(view: PagedViewKey): void {
    this.setListPage(view, Math.max(1, (this.listPages()[view] || 1) - 1));
  }

  nextPage(view: PagedViewKey, total: number): void {
    this.setListPage(view, Math.min(this.totalPages(total), (this.listPages()[view] || 1) + 1));
  }

  async refreshAll(): Promise<void> {
    await this.refreshView(this.activeView());
  }

  async refreshView(view = this.activeView()): Promise<void> {
    if (!this.token()) {
      return;
    }
    this.loading.set(true);
    this.clearNotices();
    try {
      switch (view) {
        case 'overview':
          this.overview.set(await this.request<OpsOverview>('/overview'));
          break;
        case 'users':
          this.users.set((await this.request<{ items: OpsUser[] }>('/users')).items || []);
          if (!this.products().length) {
            this.products.set((await this.request<{ items: OpsProduct[] }>('/products')).items || []);
          }
          break;
        case 'devices':
          this.devices.set((await this.request<{ items: OpsDevice[] }>('/devices')).items || []);
          break;
        case 'products':
          this.products.set((await this.request<{ items: OpsProduct[] }>('/products')).items || []);
          if (!this.selectedProduct()) {
            this.startCreateProduct();
          }
          break;
        case 'orders':
          this.orders.set((await this.request<{ items: OpsPurchaseOrder[] }>('/orders')).items || []);
          if (!this.users().length) {
            this.users.set((await this.request<{ items: OpsUser[] }>('/users')).items || []);
          }
          if (!this.products().length) {
            this.products.set((await this.request<{ items: OpsProduct[] }>('/products')).items || []);
          }
          break;
        case 'admins':
          this.admins.set((await this.request<{ items: OpsAdmin[] }>('/admins')).items || []);
          break;
        case 'roles':
          this.roles.set((await this.request<{ items: OpsRole[] }>('/roles')).items || []);
          break;
        case 'menus':
          this.menus.set((await this.request<{ items: OpsMenu[] }>('/menus')).items || []);
          break;
        case 'relays':
          this.relays.set(await this.request<OpsRelayTopology>('/relays'));
          break;
      }
    } catch (error) {
      this.setError(error);
    } finally {
      this.loading.set(false);
    }
  }

  startCreateProduct(): void {
    this.selectedProduct.set(null);
    this.productDraft.set(emptyProduct());
  }

  editProduct(product: OpsProduct): void {
    this.selectedProduct.set(product);
    this.productDraft.set({ ...product });
  }

  async saveProduct(): Promise<void> {
    this.clearNotices();
    const draft = this.productDraft();
    this.loading.set(true);
    try {
      const saved = await this.request<OpsProduct>('/products', {
        method: 'POST',
        body: JSON.stringify({
          ...draft,
          merchantId: draft.merchantId?.trim() || 'merchant-platform',
          merchantName: draft.merchantName?.trim() || 'SLAN Platform',
          priceCents: Number(draft.priceCents || 0),
          unitQuantity: Number(draft.unitQuantity || 1),
          maxActiveDevices: Number(draft.maxActiveDevices || 0),
          bandwidthLimitMbps: Number(draft.bandwidthLimitMbps || 0),
        }),
      });
      this.message.set(`商品已保存：${saved.productCode}`);
      this.selectedProduct.set(saved);
      this.productDraft.set(saved);
      await this.refreshView('products');
    } catch (error) {
      this.setError(error);
    } finally {
      this.loading.set(false);
    }
  }

  setProductDraft<K extends keyof OpsProduct>(key: K, value: any): void {
    this.productDraft.update((draft) => ({ ...draft, [key]: value }));
  }

  formatMoney(cents: number, currency = 'CNY'): string {
    const amount = (Number(cents || 0) / 100).toFixed(2);
    return currency === 'CNY' ? `¥${amount}` : `${currency} ${amount}`;
  }

  async updateOrderStatus(order: OpsPurchaseOrder, status: string): Promise<void> {
    this.clearNotices();
    this.loading.set(true);
    try {
      const saved = await this.request<OpsPurchaseOrder>(`/orders/${encodeURIComponent(order.orderId)}/status`, {
        method: 'PUT',
        body: JSON.stringify({ status }),
      });
      this.message.set(`订单 ${saved.orderId} 已更新为 ${saved.status}`);
      await this.refreshView('orders');
    } catch (error) {
      this.setError(error);
    } finally {
      this.loading.set(false);
    }
  }

  setPaidOrderDraft<K extends keyof PaidOrderDraft>(key: K, value: any): void {
    this.paidOrderDraft.update((draft) => ({ ...draft, [key]: value }));
  }

  openPaidOrderDialog(user: OpsUser, productCode: string): void {
    const product = this.products().find((item) => item.productCode === productCode);
    this.paidOrderUser.set(user);
    this.paidOrderDraft.set({
      userId: user.userId,
      productCode,
      quantity: product?.productType === 'addon_dns' ? 1 : 1,
      months: 1,
    });
  }

  closePaidOrderDialog(): void {
    this.paidOrderUser.set(null);
  }

  paidOrderProductName(): string {
    const code = this.paidOrderDraft().productCode;
    return this.products().find((item) => item.productCode === code)?.productName || code;
  }

  paidOrderProducts(): OpsProduct[] {
    const dns = this.isPaidOrderDNS();
    return this.products().filter((item) => dns ? item.productType === 'addon_dns' : item.productType === 'addon_device');
  }

  isPaidOrderDNS(): boolean {
    const code = this.paidOrderDraft().productCode;
    return this.products().find((item) => item.productCode === code)?.productType === 'addon_dns' || code === 'dns';
  }

  orderLineLabel(order: OpsPurchaseOrder): string {
    const duration = this.durationLabel(order.billingCycle, order.months);
    if (order.productType === 'addon_dns') {
      return `${order.productCode} · ${duration}`;
    }
    return `${order.productCode} · ${order.quantity} 台 / ${duration}`;
  }

  durationLabel(billingCycle = 'month', months = 1): string {
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

  async createPaidOrder(): Promise<void> {
    this.clearNotices();
    const draft = this.paidOrderDraft();
    if (!draft.userId || !draft.productCode) {
      this.error.set('请选择用户和商品');
      return;
    }
    this.loading.set(true);
    try {
      const saved = await this.request<OpsPurchaseOrder>('/orders/paid', {
        method: 'POST',
        body: JSON.stringify({
          userId: draft.userId,
          productCode: draft.productCode,
          quantity: Number(draft.quantity || 1),
          months: Number(draft.months || 1),
        }),
      });
      this.message.set(`已为 ${saved.userEmail || saved.userId} 开通 ${saved.productName}`);
      this.closePaidOrderDialog();
      this.orders.set((await this.request<{ items: OpsPurchaseOrder[] }>('/orders')).items || []);
      this.users.set((await this.request<{ items: OpsUser[] }>('/users')).items || []);
    } catch (error) {
      this.setError(error);
    } finally {
      this.loading.set(false);
    }
  }

  formatTime(seconds?: number): string {
    if (!seconds) {
      return '-';
    }
    return new Date(seconds * 1000).toLocaleString();
  }

  private setToken(token: string, adminId = '', adminName = ''): void {
    localStorage.setItem('slan.opsToken', token);
    this.token.set(token);
    if (adminId) {
      localStorage.setItem('slan.opsAdminId', adminId);
      this.adminId.set(adminId);
    }
    if (adminName) {
      localStorage.setItem('slan.opsAdminName', adminName);
      this.adminName.set(adminName);
    }
  }

  private pageRows<T>(view: PagedViewKey, rows: T[]): T[] {
    const page = this.currentPage(view, rows.length);
    const start = (page - 1) * this.pageSize;
    return rows.slice(start, start + this.pageSize);
  }

  private currentPage(view: PagedViewKey, total: number): number {
    return Math.min(Math.max(1, this.listPages()[view] || 1), this.totalPages(total));
  }

  private totalPages(total: number): number {
    return Math.max(1, Math.ceil(total / this.pageSize));
  }

  private setListPage(view: PagedViewKey, page: number): void {
    this.listPages.update((pages) => ({ ...pages, [view]: page }));
  }

  private isPagedView(view: ViewKey): view is PagedViewKey {
    return view !== 'overview' && view !== 'relays';
  }

  private async request<T>(path: string, init: RequestInit = {}, auth = true): Promise<T> {
    const headers = new Headers(init.headers || {});
    headers.set('Content-Type', 'application/json');
    if (auth && this.token()) {
      headers.set('Authorization', `Bearer ${this.token()}`);
    }
    const response = await fetch(`/ops-api${path}`, { ...init, headers });
    const text = await response.text();
    const payload = text ? JSON.parse(text) : null;
    if (!response.ok) {
      throw new Error(payload?.message || `request failed: ${response.status}`);
    }
    return payload as T;
  }

  private clearNotices(): void {
    this.error.set('');
    this.message.set('');
  }

  private setError(error: unknown): void {
    this.error.set(error instanceof Error ? error.message : String(error));
    this.message.set('');
  }
}

function emptyProduct(): OpsProduct {
  return {
    productCode: '',
    merchantId: 'merchant-platform',
    merchantName: 'SLAN Platform',
    productName: '',
    description: '',
    productType: 'addon_device',
    priceCents: 1000,
    currency: 'CNY',
    billingCycle: 'month',
    unitQuantity: 1,
    maxActiveDevices: 0,
    bandwidthLimitMbps: 0,
    isDefault: false,
    status: 'active',
  };
}

function filterRows<T>(rows: T[], keyword: string): T[] {
  const value = keyword.trim().toLowerCase();
  if (!value) {
    return rows;
  }
  return rows.filter((row) => JSON.stringify(row).toLowerCase().includes(value));
}
