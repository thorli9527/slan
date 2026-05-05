import { Injectable, inject, signal } from '@angular/core';
import { PlanConfig } from '../../shared/models';
import { OpsApiService } from '../../shared/ops-api.service';

@Injectable({ providedIn: 'root' })
export class SettingsOpsService {
  private readonly api = inject(OpsApiService);
  readonly config = signal<PlanConfig>(emptyPlanConfig());

  async refresh(token: string): Promise<void> {
    this.config.set(await this.planConfig(token));
  }

  updateField(key: keyof PlanConfig, value: number): void {
    this.config.update((config) => ({
      ...config,
      [key]: Number(value) || 0,
    }));
  }

  async saveCurrent(token: string): Promise<PlanConfig> {
    const saved = await this.savePlanConfig(token, normalizePlanConfig(this.config()));
    this.config.set(saved);
    return saved;
  }

  planConfig(token: string): Promise<PlanConfig> {
    return this.api.request<PlanConfig>('/plan-config', token);
  }

  savePlanConfig(token: string, plan: PlanConfig): Promise<PlanConfig> {
    return this.api.request<PlanConfig>('/plan-config', token, {
      method: 'PUT',
      body: JSON.stringify(plan),
    });
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
