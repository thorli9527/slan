import { CommonModule } from '@angular/common';
import { Component, computed, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';

type ViewKey = 'overview' | 'users' | 'devices' | 'admins' | 'roles' | 'menus' | 'relays' | 'settings';
type PagedViewKey = Exclude<ViewKey, 'overview' | 'relays' | 'settings'>;

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

type OpsUser = {
  userId: string;
  email: string;
  deviceCount: number;
  nodeCount: number;
  roleCodes?: string[];
  planOverride?: PlanConfig;
};

type PlanConfig = {
  maxActiveDevices: number;
  relayIngressKbps: number;
  relayEgressKbps: number;
  udpIngressKbps: number;
  udpEgressKbps: number;
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
    { key: 'admins', label: '管理员', caption: '运营账号资料' },
    { key: 'roles', label: '角色权限', caption: 'RBAC 角色' },
    { key: 'menus', label: '菜单权限', caption: '运营菜单' },
    { key: 'relays', label: 'Relay 拓扑', caption: '中继节点健康' },
    { key: 'settings', label: '配置管理', caption: '设备与流量全局限制' },
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
  readonly admins = signal<OpsAdmin[]>([]);
  readonly roles = signal<OpsRole[]>([]);
  readonly menus = signal<OpsMenu[]>([]);
  readonly relays = signal<OpsRelayTopology | null>(null);
  readonly planConfig = signal<PlanConfig>({
    maxActiveDevices: 5,
    relayIngressKbps: 512,
    relayEgressKbps: 512,
    udpIngressKbps: 0,
    udpEgressKbps: 0,
  });
  readonly keyword = signal('');
  readonly pageSize = 10;
  readonly listPages = signal<Record<PagedViewKey, number>>({
    users: 1,
    devices: 1,
    admins: 1,
    roles: 1,
    menus: 1,
  });

  loginName = 'admin';
  password = '';
  emergencyToken = '';
  opsNewPassword = '';
  opsConfirmPassword = '';
  showPasswordPanel = false;
  editingUserPlan: OpsUser | null = null;
  userPlanDraft: PlanConfig = this.emptyPlanConfig();

  readonly filteredUsers = computed(() => filterRows(this.users(), this.keyword()));
  readonly filteredDevices = computed(() => filterRows(this.devices(), this.keyword()));
  readonly filteredAdmins = computed(() => filterRows(this.admins(), this.keyword()));
  readonly pagedUsers = computed(() => this.pageRows('users', this.filteredUsers()));
  readonly pagedDevices = computed(() => this.pageRows('devices', this.filteredDevices()));
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

  async logout(): Promise<void> {
    if (this.token()) {
      try {
        await this.request('/logout', { method: 'POST' });
      } catch {
        // Local logout still clears the browser session when the server token is already invalid.
      }
    }
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
      await this.request('/me/password', {
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
          break;
        case 'devices':
          this.devices.set((await this.request<{ items: OpsDevice[] }>('/devices')).items || []);
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
        case 'settings':
          this.planConfig.set(await this.request<PlanConfig>('/plan-config'));
          break;
      }
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

  async saveGlobalPlanConfig(): Promise<void> {
    this.clearNotices();
    this.loading.set(true);
    try {
      const saved = await this.request<PlanConfig>('/plan-config', {
        method: 'PUT',
        body: JSON.stringify(this.normalizePlanConfig(this.planConfig())),
      });
      this.planConfig.set(saved);
      this.message.set('全局设备与流量配置已保存。');
    } catch (error) {
      this.setError(error);
    } finally {
      this.loading.set(false);
    }
  }

  updateGlobalPlanField(key: keyof PlanConfig, value: number): void {
    this.planConfig.update((config) => ({
      ...config,
      [key]: Number(value) || 0,
    }));
  }

  openUserPlanDialog(user: OpsUser): void {
    this.clearNotices();
    this.editingUserPlan = user;
    this.userPlanDraft = this.normalizePlanConfig(user.planOverride || this.planConfig());
  }

  closeUserPlanDialog(): void {
    this.editingUserPlan = null;
    this.userPlanDraft = this.emptyPlanConfig();
  }

  async saveUserPlanOverride(): Promise<void> {
    const user = this.editingUserPlan;
    if (!user) {
      return;
    }
    this.loading.set(true);
    try {
      await this.request(`/users/${encodeURIComponent(user.userId)}/plan`, {
        method: 'PUT',
        body: JSON.stringify(this.normalizePlanConfig(this.userPlanDraft)),
      });
      this.closeUserPlanDialog();
      await this.refreshView('users');
      this.message.set('用户专属设备与流量配置已保存。');
    } catch (error) {
      this.setError(error);
    } finally {
      this.loading.set(false);
    }
  }

  async clearUserPlanOverride(): Promise<void> {
    const user = this.editingUserPlan;
    if (!user) {
      return;
    }
    this.loading.set(true);
    try {
      await this.request(`/users/${encodeURIComponent(user.userId)}/plan`, { method: 'DELETE' });
      this.closeUserPlanDialog();
      await this.refreshView('users');
      this.message.set('用户已恢复使用全局配置。');
    } catch (error) {
      this.setError(error);
    } finally {
      this.loading.set(false);
    }
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
    return view !== 'overview' && view !== 'relays' && view !== 'settings';
  }

  private emptyPlanConfig(): PlanConfig {
    return {
      maxActiveDevices: 5,
      relayIngressKbps: 512,
      relayEgressKbps: 512,
      udpIngressKbps: 0,
      udpEgressKbps: 0,
    };
  }

  private normalizePlanConfig(input: PlanConfig): PlanConfig {
    return {
      maxActiveDevices: Math.max(1, Number(input.maxActiveDevices) || 5),
      relayIngressKbps: Math.max(0, Number(input.relayIngressKbps) || 0),
      relayEgressKbps: Math.max(0, Number(input.relayEgressKbps) || 0),
      udpIngressKbps: Math.max(0, Number(input.udpIngressKbps) || 0),
      udpEgressKbps: Math.max(0, Number(input.udpEgressKbps) || 0),
    };
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


function filterRows<T>(rows: T[], keyword: string): T[] {
  const value = keyword.trim().toLowerCase();
  if (!value) {
    return rows;
  }
  return rows.filter((row) => JSON.stringify(row).toLowerCase().includes(value));
}


