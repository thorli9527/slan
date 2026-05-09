import { Injectable, inject, signal } from '@angular/core';
import { OpsNetworkQuality, OpsNetworkQualityItem } from '../../shared/models';
import { OpsApiService } from '../../shared/ops-api.service';

@Injectable({ providedIn: 'root' })
export class QualityOpsService {
  private readonly api = inject(OpsApiService);
  readonly items = signal<OpsNetworkQualityItem[]>([]);

  async refresh(token: string): Promise<void> {
    this.items.set(await this.listNetworkQuality(token));
  }

  async listNetworkQuality(token: string): Promise<OpsNetworkQualityItem[]> {
    return (await this.api.request<OpsNetworkQuality>('/network-quality', token)).items || [];
  }
}
