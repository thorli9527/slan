import { Injectable, inject, signal } from '@angular/core';
import { WireNodeEventFilter, WireNodeEventList, WireNodesOpsView } from '../../shared/models';
import { OpsApiService } from '../../shared/ops-api.service';

@Injectable({ providedIn: 'root' })
export class WireOpsService {
  private readonly api = inject(OpsApiService);

  readonly nodes = signal<WireNodesOpsView | null>(null);
  readonly events = signal<WireNodeEventList>({ items: [], page: 1, pageSize: 25, total: 0 });
  filter: WireNodeEventFilter = emptyWireNodeEventFilter();

  async refresh(token: string): Promise<void> {
    this.nodes.set(await this.fetchNodes(token));
    await this.refreshEvents(token);
  }

  async refreshEvents(token: string): Promise<void> {
    this.events.set(await this.fetchEvents(token, this.filter));
  }

  async applyFilter(token: string): Promise<void> {
    this.filter.page = 1;
    await this.refreshEvents(token);
  }

  async resetFilter(token: string): Promise<void> {
    this.filter = emptyWireNodeEventFilter();
    await this.refreshEvents(token);
  }

  async previousPage(token: string): Promise<void> {
    if (this.filter.page <= 1) {
      return;
    }
    this.filter.page -= 1;
    await this.refreshEvents(token);
  }

  async nextPage(token: string): Promise<void> {
    if (!this.canNextPage()) {
      return;
    }
    this.filter.page += 1;
    await this.refreshEvents(token);
  }

  canNextPage(): boolean {
    return this.filter.page * this.filter.pageSize < this.events().total;
  }

  fetchNodes(token: string): Promise<WireNodesOpsView> {
    return this.api.request<WireNodesOpsView>('/wire-nodes', token);
  }

  fetchEvents(token: string, filter: WireNodeEventFilter): Promise<WireNodeEventList> {
    return this.api.request<WireNodeEventList>(`/wire-node-events?${wireEventQueryString(filter)}`, token);
  }
}

function emptyWireNodeEventFilter(): WireNodeEventFilter {
  return {
    nodeKind: '',
    regionId: '',
    nodeId: '',
    eventType: '',
    createdFromMs: '',
    createdToMs: '',
    page: 1,
    pageSize: 25,
  };
}

function wireEventQueryString(filter: WireNodeEventFilter): string {
  const params = new URLSearchParams();
  addQueryParam(params, 'nodeKind', filter.nodeKind);
  addQueryParam(params, 'regionId', filter.regionId);
  addQueryParam(params, 'nodeId', filter.nodeId);
  addQueryParam(params, 'eventType', filter.eventType);
  addQueryParam(params, 'createdFromMs', filter.createdFromMs);
  addQueryParam(params, 'createdToMs', filter.createdToMs);
  params.set('page', String(Math.max(1, Number(filter.page) || 1)));
  params.set('pageSize', String(Math.min(200, Math.max(1, Number(filter.pageSize) || 25))));
  return params.toString();
}

function addQueryParam(params: URLSearchParams, key: string, value: string): void {
  const trimmed = value.trim();
  if (trimmed) {
    params.set(key, trimmed);
  }
}
