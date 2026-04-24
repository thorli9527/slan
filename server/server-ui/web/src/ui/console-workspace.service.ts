import { Injectable } from '@angular/core';

import { ConsoleApiService } from './console-api.service';
import { NetworkAssignment, NetworkDetail, NetworkHome, Subnet } from './api-contracts';

export type WorkspaceSnapshot = {
  home: NetworkHome;
  detail: NetworkDetail | null;
  subnets: Subnet[];
  assignments: NetworkAssignment[];
  draftIps: Record<string, string>;
};

@Injectable({ providedIn: 'root' })
export class ConsoleWorkspaceService {
  constructor(private readonly api: ConsoleApiService) {}

  async loadWorkspace(token: string): Promise<WorkspaceSnapshot> {
    const home = await this.api.getHome(token);
    if (!home.activeNetwork) {
      return {
        home,
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
        detail,
        subnets: subnets.items,
        assignments: [],
        draftIps: {},
      };
    }

    const assignments = (await this.api.getAssignments(token, networkId)).items;
    return {
      home,
      detail,
      subnets: subnets.items,
      assignments,
      draftIps: Object.fromEntries(assignments.map((item) => [item.attachmentId, item.virtualIp || ''])),
    };
  }

  switchNetwork(token: string, networkId: string, deviceId: string): Promise<void> {
    return this.api.switchNetwork(token, networkId, deviceId);
  }

  activateNetwork(token: string, networkId: string, deviceId: string): Promise<void> {
    return this.api.activateNetwork(token, networkId, deviceId);
  }
}
