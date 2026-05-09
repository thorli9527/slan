import { Injectable, inject, signal } from '@angular/core';
import { OpsRelayTopology } from '../../shared/models';
import { OpsApiService } from '../../shared/ops-api.service';

@Injectable({ providedIn: 'root' })
export class RelayOpsService {
  private readonly api = inject(OpsApiService);
  readonly topology = signal<OpsRelayTopology | null>(null);

  async refresh(token: string): Promise<void> {
    this.topology.set(await this.fetchTopology(token));
  }

  fetchTopology(token: string): Promise<OpsRelayTopology> {
    return this.api.request<OpsRelayTopology>('/relays', token);
  }
}
