import { Injectable, inject, signal } from '@angular/core';
import { OpsUser, PlanConfig } from '../../shared/models';
import { OpsApiService } from '../../shared/ops-api.service';

@Injectable({ providedIn: 'root' })
export class UsersOpsService {
  private readonly api = inject(OpsApiService);
  readonly users = signal<OpsUser[]>([]);
  readonly editingUser = signal<OpsUser | null>(null);
  draft: PlanConfig = emptyPlanConfig();

  async refresh(token: string): Promise<void> {
    this.users.set(await this.listUsers(token));
  }

  async listUsers(token: string): Promise<OpsUser[]> {
    return (await this.api.request<{ items: OpsUser[] }>('/users', token)).items || [];
  }

  openPlanDialog(user: OpsUser, fallback: PlanConfig): void {
    this.editingUser.set(user);
    this.draft = normalizePlanConfig(user.planOverride || fallback);
  }

  closePlanDialog(): void {
    this.editingUser.set(null);
    this.draft = emptyPlanConfig();
  }

  async saveCurrentPlanOverride(token: string): Promise<void> {
    const user = this.editingUser();
    if (!user) {
      return;
    }
    await this.savePlanOverride(token, user.userId, normalizePlanConfig(this.draft));
    this.closePlanDialog();
  }

  async clearCurrentPlanOverride(token: string): Promise<void> {
    const user = this.editingUser();
    if (!user) {
      return;
    }
    await this.clearPlanOverride(token, user.userId);
    this.closePlanDialog();
  }

  savePlanOverride(token: string, userId: string, plan: PlanConfig): Promise<unknown> {
    return this.api.request(`/users/${encodeURIComponent(userId)}/plan`, token, {
      method: 'PUT',
      body: JSON.stringify(plan),
    });
  }

  clearPlanOverride(token: string, userId: string): Promise<unknown> {
    return this.api.request(`/users/${encodeURIComponent(userId)}/plan`, token, { method: 'DELETE' });
  }
}

function emptyPlanConfig(): PlanConfig {
  return {
    maxActiveDevices: 5,
    relayIngressKbps: 512,
    relayEgressKbps: 512,
    udpIngressKbps: 0,
    udpEgressKbps: 0,
  };
}

function normalizePlanConfig(input: PlanConfig): PlanConfig {
  return {
    maxActiveDevices: Math.max(1, Number(input.maxActiveDevices) || 5),
    relayIngressKbps: Math.max(0, Number(input.relayIngressKbps) || 0),
    relayEgressKbps: Math.max(0, Number(input.relayEgressKbps) || 0),
    udpIngressKbps: Math.max(0, Number(input.udpIngressKbps) || 0),
    udpEgressKbps: Math.max(0, Number(input.udpEgressKbps) || 0),
  };
}
