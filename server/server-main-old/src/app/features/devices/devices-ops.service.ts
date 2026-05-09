import { Injectable, inject, signal } from '@angular/core';
import { OpsDevice } from '../../shared/models';
import { OpsApiService } from '../../shared/ops-api.service';

@Injectable({ providedIn: 'root' })
export class DevicesOpsService {
  private readonly api = inject(OpsApiService);
  readonly devices = signal<OpsDevice[]>([]);

  async refresh(token: string): Promise<void> {
    this.devices.set(await this.listDevices(token));
  }

  async listDevices(token: string): Promise<OpsDevice[]> {
    return (await this.api.request<{ items: OpsDevice[] }>('/devices', token)).items || [];
  }
}
