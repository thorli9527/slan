import { Injectable } from '@angular/core';

import { ConsoleApiService } from './console-api.service';
import { Network, NetworkAssignment, NetworkDetail, NetworkHome, NetworkJoinResult, Subnet } from './api-contracts';

export type WorkspaceSnapshot = {
  home: NetworkHome;
  networks: Network[];
  detail: NetworkDetail | null;
  subnets: Subnet[];
  assignments: NetworkAssignment[];
  draftIps: Record<string, string>;
};

@Injectable({ providedIn: 'root' })
export class ConsoleWorkspaceService {
  constructor(private readonly api: ConsoleApiService) {}

  async loadWorkspace(token: string): Promise<WorkspaceSnapshot> {
    const [home, networksResponse] = await Promise.all([
      this.api.getHome(token),
      this.api.listNetworks(token),
    ]);
    const networks = networksResponse.items;
    if (!home.activeNetwork) {
      return {
        home,
        networks,
        detail: null,
        subnets: [],
        assignments: [],
        draftIps: {},
      };
    }

    const networkId = home.activeNetwork.networkId;
    const [detail, subnets] = await Promise.all([
      this.api.getNetworkDetail(token, networkId),
      this.api.getSubnets(token, networkId),
    ]);

    if (!detail.ownedByCurrentUser) {
      return {
        home,
        networks,
        detail,
        subnets: subnets.items,
        assignments: [],
        draftIps: {},
      };
    }

    const assignments = (await this.api.getAssignments(token, networkId)).items;
    return {
      home,
      networks,
      detail,
      subnets: subnets.items,
      assignments,
      draftIps: Object.fromEntries(assignments.map((item) => [item.attachmentId, item.virtualIp || ''])),
    };
  }

  switchNetwork(token: string, networkId: string, deviceId: string): Promise<NetworkJoinResult> {
    return this.api.switchNetwork(token, networkId, deviceId);
  }

  activateNetwork(token: string, networkId: string, deviceId: string): Promise<NetworkJoinResult> {
    return this.api.activateNetwork(token, networkId, deviceId);
  }
}
