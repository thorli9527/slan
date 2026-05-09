import { CommonModule } from '@angular/common';
import { Component, ViewEncapsulation, computed, inject, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { DevicesViewComponent } from './features/devices/devices-view.component';
import { IceViewComponent } from './features/ice/ice-view.component';
import { OverviewViewComponent } from './features/overview/overview-view.component';
import { QualityViewComponent } from './features/quality/quality-view.component';
import { AdminsViewComponent } from './features/rbac/admins-view.component';
import { MenusViewComponent } from './features/rbac/menus-view.component';
import { RolesViewComponent } from './features/rbac/roles-view.component';
import { RelayViewComponent } from './features/relay/relay-view.component';
import { SettingsViewComponent } from './features/settings/settings-view.component';
import { UsersViewComponent } from './features/users/users-view.component';
import { WireViewComponent } from './features/wire/wire-view.component';
import {
  OpsIceServer,
  OpsUser,
  PlanConfig,
  ViewKey,
} from './shared/models';
import { OpsStoreService } from './shared/ops-store.service';
import { filterRows, TableStateService } from './shared/table-state.service';

@Component({
  selector: 'slan-root',
  standalone: true,
  imports: [
    CommonModule,
    FormsModule,
    AdminsViewComponent,
    DevicesViewComponent,
    IceViewComponent,
    MenusViewComponent,
    OverviewViewComponent,
    QualityViewComponent,
    RelayViewComponent,
    RolesViewComponent,
    SettingsViewComponent,
    UsersViewComponent,
    WireViewComponent,
  ],
  templateUrl: './app.component.html',
  styleUrl: './app.component.css',
  encapsulation: ViewEncapsulation.None,
})
export class AppComponent {
  private readonly store = inject(OpsStoreService);
  readonly auth = this.store.auth;
  readonly devices = this.store.devices;
  readonly ice = this.store.ice;
  readonly overview = this.store.overview;
  readonly quality = this.store.quality;
  readonly rbac = this.store.rbac;
  readonly relay = this.store.relay;
  readonly settings = this.store.settings;
  readonly table = inject(TableStateService);
  readonly userPlans = this.store.users;
  readonly wire = this.store.wire;

  readonly views: Array<{ key: ViewKey; label: string; caption: string }> = [
    { key: 'overview', label: '运营概览', caption: '核心指标与告警' },
    { key: 'users', label: '用户管理', caption: '用户、角色与设备数' },
    { key: 'devices', label: '设备管理', caption: '设备、节点和在线状态' },
    { key: 'admins', label: '管理员', caption: '运营账号资料' },
    { key: 'roles', label: '角色权限', caption: 'RBAC 角色' },
    { key: 'menus', label: '菜单权限', caption: '运营菜单' },
    { key: 'relays', label: 'Relay 节点', caption: '中继节点管理' },
    { key: 'wire', label: 'Wire 节点', caption: 'DERP/Relay 调度' },
    { key: 'ice', label: 'ICE 服务', caption: 'STUN 与打洞质量' },
    { key: 'quality', label: '网络质量', caption: '用户实时网络质量' },
    { key: 'settings', label: '配置管理', caption: '设备与流量全局限制' },
  ];

  readonly activeView = signal<ViewKey>('overview');
  readonly loading = signal(false);
  readonly error = signal('');
  readonly message = signal('');
  readonly accountMenuOpen = signal(false);
  loginName = 'admin';
  password = '';
  emergencyToken = '';
  opsNewPassword = '';
  opsConfirmPassword = '';
  showPasswordPanel = false;

  readonly filteredUsers = computed(() => filterRows(this.userPlans.users(), this.table.keyword()));
  readonly filteredDevices = computed(() => filterRows(this.devices.devices(), this.table.keyword()));
  readonly filteredAdmins = computed(() => filterRows(this.rbac.admins(), this.table.keyword()));
  readonly pagedUsers = computed(() => this.table.rows('users', this.filteredUsers()));
  readonly pagedDevices = computed(() => this.table.rows('devices', this.filteredDevices()));
  readonly pagedAdmins = computed(() => this.table.rows('admins', this.filteredAdmins()));
  readonly pagedRoles = computed(() => this.table.rows('roles', this.rbac.roles()));
  readonly pagedMenus = computed(() => this.table.rows('menus', this.rbac.menus()));
  readonly filteredIceServers = computed(() => filterRows(this.ice.servers(), this.table.keyword()));
  readonly pagedIceServers = computed(() => this.table.rows('ice', this.filteredIceServers()));
  readonly filteredNetworkQuality = computed(() => filterRows(this.quality.items(), this.table.keyword()));
  readonly pagedNetworkQuality = computed(() => this.table.rows('quality', this.filteredNetworkQuality()));
  readonly activeViewLabel = computed(() => this.views.find((item) => item.key === this.activeView())?.label || '运营后台');

  constructor() {
    if (this.auth.token()) {
      void this.refreshAll();
    }
  }

  async login(): Promise<void> {
    this.clearNotices();
    if (this.emergencyToken.trim()) {
      this.auth.setSession(this.emergencyToken.trim());
      await this.refreshAll();
      return;
    }
    this.loading.set(true);
    try {
      const resp = await this.store.login(this.loginName.trim(), this.password);
      this.auth.setSession(resp.accessToken, resp.adminId, resp.loginName);
      await this.refreshAll();
    } catch (error) {
      this.setError(error);
    } finally {
      this.loading.set(false);
    }
  }

  async logout(): Promise<void> {
    this.accountMenuOpen.set(false);
    if (this.auth.token()) {
      try {
        await this.store.logout(this.auth.token());
      } catch {
        // Local logout still clears the browser session when the server token is already invalid.
      }
    }
    this.auth.clearSession();
    this.password = '';
    this.error.set('');
    this.message.set('');
  }

  async changeOwnPassword(): Promise<void> {
    this.clearNotices();
    if (!this.auth.adminId()) {
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
      await this.store.changeOwnPassword(this.auth.token(), this.opsNewPassword);
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
    this.accountMenuOpen.set(false);
    this.activeView.set(view);
    this.table.keyword.set('');
    this.table.resetView(view);
    await this.refreshView(view);
  }

  setKeyword(value: string): void {
    this.table.setKeyword(value, this.activeView());
  }

  toggleAccountMenu(): void {
    this.accountMenuOpen.update((open) => !open);
  }

  openPasswordPanel(): void {
    this.accountMenuOpen.set(false);
    this.showPasswordPanel = true;
  }

  async refreshAll(): Promise<void> {
    await this.refreshView(this.activeView());
  }

  async refreshView(view = this.activeView()): Promise<void> {
    if (!this.auth.token()) {
      return;
    }
    this.loading.set(true);
    this.clearNotices();
    try {
      switch (view) {
        case 'overview':
          await this.overview.refresh(this.auth.token());
          break;
        case 'users':
          await this.userPlans.refresh(this.auth.token());
          break;
        case 'devices':
          await this.devices.refresh(this.auth.token());
          break;
        case 'admins':
          await this.rbac.refreshAdmins(this.auth.token());
          break;
        case 'roles':
          await this.rbac.refreshRoles(this.auth.token());
          break;
        case 'menus':
          await this.rbac.refreshMenus(this.auth.token());
          break;
        case 'relays':
          await this.relay.refresh(this.auth.token());
          break;
        case 'wire':
          await this.refreshWire();
          break;
        case 'ice':
          await this.ice.refresh(this.auth.token());
          break;
        case 'quality':
          await this.quality.refresh(this.auth.token());
          break;
        case 'settings':
          await this.settings.refresh(this.auth.token());
          break;
      }
    } catch (error) {
      this.setError(error);
    } finally {
      this.loading.set(false);
    }
  }
  async refreshWire(): Promise<void> {
    await this.wire.refresh(this.auth.token());
  }

  async refreshWireEvents(): Promise<void> {
    await this.wire.refreshEvents(this.auth.token());
  }

  async applyWireEventFilter(): Promise<void> {
    this.loading.set(true);
    this.clearNotices();
    try {
      await this.wire.applyFilter(this.auth.token());
    } catch (error) {
      this.setError(error);
    } finally {
      this.loading.set(false);
    }
  }

  async resetWireEventFilter(): Promise<void> {
    this.loading.set(true);
    this.clearNotices();
    try {
      await this.wire.resetFilter(this.auth.token());
    } catch (error) {
      this.setError(error);
    } finally {
      this.loading.set(false);
    }
  }

  async previousWireEventPage(): Promise<void> {
    this.loading.set(true);
    this.clearNotices();
    try {
      await this.wire.previousPage(this.auth.token());
    } catch (error) {
      this.setError(error);
    } finally {
      this.loading.set(false);
    }
  }

  async nextWireEventPage(): Promise<void> {
    this.loading.set(true);
    this.clearNotices();
    try {
      await this.wire.nextPage(this.auth.token());
    } catch (error) {
      this.setError(error);
    } finally {
      this.loading.set(false);
    }
  }

  editIceServer(server: OpsIceServer): void {
    this.clearNotices();
    this.ice.edit(server);
  }

  resetIceDraft(): void {
    this.ice.resetDraft();
  }

  async saveIceServer(): Promise<void> {
    this.clearNotices();
    this.loading.set(true);
    try {
      await this.ice.saveCurrent(this.auth.token());
      this.message.set('ICE 服务配置已保存。');
    } catch (error) {
      this.setError(error);
    } finally {
      this.loading.set(false);
    }
  }

  async updateIceStatus(server: OpsIceServer, status: string): Promise<void> {
    this.clearNotices();
    this.loading.set(true);
    try {
      await this.ice.updateNodeStatus(this.auth.token(), server, status);
      this.message.set(`ICE 服务已切换为 ${status}。`);
    } catch (error) {
      this.setError(error);
    } finally {
      this.loading.set(false);
    }
  }

  async saveGlobalPlanConfig(): Promise<void> {
    this.clearNotices();
    this.loading.set(true);
    try {
      await this.settings.saveCurrent(this.auth.token());
      this.message.set('全局设备与流量配置已保存。');
    } catch (error) {
      this.setError(error);
    } finally {
      this.loading.set(false);
    }
  }

  updateGlobalPlanField(key: keyof PlanConfig, value: number): void {
    this.settings.updateField(key, value);
  }

  openUserPlanDialog(user: OpsUser): void {
    this.clearNotices();
    this.userPlans.openPlanDialog(user, this.settings.config());
  }

  closeUserPlanDialog(): void {
    this.userPlans.closePlanDialog();
  }

  async saveUserPlanOverride(): Promise<void> {
    if (!this.userPlans.editingUser()) {
      return;
    }
    this.loading.set(true);
    try {
      await this.userPlans.saveCurrentPlanOverride(this.auth.token());
      await this.refreshView('users');
      this.message.set('用户专属设备与流量配置已保存。');
    } catch (error) {
      this.setError(error);
    } finally {
      this.loading.set(false);
    }
  }

  async clearUserPlanOverride(): Promise<void> {
    if (!this.userPlans.editingUser()) {
      return;
    }
    this.loading.set(true);
    try {
      await this.userPlans.clearCurrentPlanOverride(this.auth.token());
      await this.refreshView('users');
      this.message.set('用户已恢复使用全局配置。');
    } catch (error) {
      this.setError(error);
    } finally {
      this.loading.set(false);
    }
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
