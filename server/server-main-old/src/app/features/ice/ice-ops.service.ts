import { Injectable, inject, signal } from '@angular/core';
import { IceServerDraft, OpsIceServer, OpsIceStats, OpsIceServerStats } from '../../shared/models';
import { OpsApiService } from '../../shared/ops-api.service';

@Injectable({ providedIn: 'root' })
export class IceOpsService {
  private readonly api = inject(OpsApiService);

  readonly servers = signal<OpsIceServer[]>([]);
  readonly stats = signal<OpsIceServerStats[]>([]);
  editingServerId = '';
  draft: IceServerDraft = emptyIceServerDraft();

  async refresh(token: string): Promise<void> {
    this.servers.set(await this.listServers(token));
    this.stats.set(await this.listStats(token));
  }

  edit(server: OpsIceServer): void {
    this.editingServerId = server.serverId;
    this.draft = {
      serverId: server.serverId,
      name: server.name || '',
      provider: server.provider || '',
      region: server.region || '',
      country: server.country || 'CN',
      publicIp: server.publicIp || '',
      udpAddr: server.udpAddr || '',
      stunPort: server.stunPort || 3478,
      priority: server.priority || 100,
      weight: server.weight || 100,
      status: server.status || 'enabled',
      featuresText: (server.features || []).join(', '),
      remark: server.remark || '',
    };
  }

  resetDraft(): void {
    this.editingServerId = '';
    this.draft = emptyIceServerDraft();
  }

  async saveCurrent(token: string): Promise<void> {
    await this.saveServer(token, this.draft, this.editingServerId);
    this.resetDraft();
    await this.refresh(token);
  }

  async updateNodeStatus(token: string, server: OpsIceServer, status: string): Promise<void> {
    await this.updateStatus(token, server.serverId, status);
    await this.refresh(token);
  }

  async listServers(token: string): Promise<OpsIceServer[]> {
    return (await this.api.request<{ items: OpsIceServer[] }>('/ice-servers', token)).items || [];
  }

  async listStats(token: string): Promise<OpsIceServerStats[]> {
    return (await this.api.request<OpsIceStats>('/ice-stats', token)).servers || [];
  }

  saveServer(token: string, draft: IceServerDraft, editingServerId: string): Promise<OpsIceServer> {
    const path = editingServerId ? `/ice-servers/${encodeURIComponent(editingServerId)}` : '/ice-servers';
    return this.api.request<OpsIceServer>(path, token, {
      method: editingServerId ? 'PUT' : 'POST',
      body: JSON.stringify(normalizeIceServerDraft(draft)),
    });
  }

  updateStatus(token: string, serverId: string, status: string): Promise<OpsIceServer> {
    return this.api.request<OpsIceServer>(`/ice-servers/${encodeURIComponent(serverId)}/status`, token, {
      method: 'PATCH',
      body: JSON.stringify({ status }),
    });
  }
}

function emptyIceServerDraft(): IceServerDraft {
  return {
    serverId: '',
    name: '',
    provider: 'self',
    region: '',
    country: 'CN',
    publicIp: '',
    udpAddr: '',
    stunPort: 3478,
    priority: 100,
    weight: 100,
    status: 'enabled',
    featuresText: 'stun',
    remark: '',
  };
}

function normalizeIceServerDraft(input: IceServerDraft): Record<string, unknown> {
  return {
    serverId: input.serverId.trim(),
    name: input.name.trim(),
    provider: input.provider.trim(),
    region: input.region.trim(),
    country: input.country.trim(),
    publicIp: input.publicIp.trim(),
    udpAddr: input.udpAddr.trim(),
    stunPort: Math.max(1, Number(input.stunPort) || 3478),
    priority: Math.max(1, Number(input.priority) || 100),
    weight: Math.max(1, Number(input.weight) || 100),
    status: input.status || 'enabled',
    features: input.featuresText.split(',').map((item) => item.trim()).filter(Boolean),
    remark: input.remark.trim(),
  };
}
