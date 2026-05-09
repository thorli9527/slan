import { Injectable, inject, signal } from '@angular/core';
import { OpsAdmin, OpsMenu, OpsRole } from '../../shared/models';
import { OpsApiService } from '../../shared/ops-api.service';

@Injectable({ providedIn: 'root' })
export class RbacOpsService {
  private readonly api = inject(OpsApiService);
  readonly admins = signal<OpsAdmin[]>([]);
  readonly roles = signal<OpsRole[]>([]);
  readonly menus = signal<OpsMenu[]>([]);

  async refreshAdmins(token: string): Promise<void> {
    this.admins.set(await this.listAdmins(token));
  }

  async refreshRoles(token: string): Promise<void> {
    this.roles.set(await this.listRoles(token));
  }

  async refreshMenus(token: string): Promise<void> {
    this.menus.set(await this.listMenus(token));
  }

  async listAdmins(token: string): Promise<OpsAdmin[]> {
    return (await this.api.request<{ items: OpsAdmin[] }>('/admins', token)).items || [];
  }

  async listRoles(token: string): Promise<OpsRole[]> {
    return (await this.api.request<{ items: OpsRole[] }>('/roles', token)).items || [];
  }

  async listMenus(token: string): Promise<OpsMenu[]> {
    return (await this.api.request<{ items: OpsMenu[] }>('/menus', token)).items || [];
  }
}
