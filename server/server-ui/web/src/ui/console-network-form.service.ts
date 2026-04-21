import { Injectable } from '@angular/core';

import { NetworkDetail } from './api-contracts';

export type NetworkUpdateDraft = {
  name: string;
  description: string;
  cidr: string;
};

export type SubnetCreateDraft = {
  name: string;
  cidr: string;
  gatewayIp: string;
  allocationStartIp: string;
  allocationEndIp: string;
};

@Injectable({ providedIn: 'root' })
export class ConsoleNetworkFormService {
  validateNetworkCidr(cidr: string): void {
    if (!this.isLikelyCIDR(cidr)) {
      throw new Error('default cidr is invalid');
    }
  }

  validateAttachmentIp(virtualIp: string): void {
    if (!this.isLikelyIPv4(virtualIp)) {
      throw new Error('virtual ip is invalid');
    }
  }

  validateSubnetDraft(draft: SubnetCreateDraft): void {
    if (!draft.name.trim()) {
      throw new Error('subnet name is required');
    }
    if (!this.isLikelyCIDR(draft.cidr)) {
      throw new Error('subnet cidr is invalid');
    }
    const hasStart = draft.allocationStartIp.trim().length > 0;
    const hasEnd = draft.allocationEndIp.trim().length > 0;
    if (hasStart !== hasEnd) {
      throw new Error('allocationStartIp and allocationEndIp must be provided together');
    }
    for (const [label, value] of [
      ['gatewayIp', draft.gatewayIp],
      ['allocationStartIp', draft.allocationStartIp],
      ['allocationEndIp', draft.allocationEndIp],
    ] as const) {
      const trimmed = value.trim();
      if (trimmed && !this.isLikelyIPv4(trimmed)) {
        throw new Error(`${label} is invalid`);
      }
    }
  }

  buildNetworkUpdateDraft(detail: NetworkDetail): NetworkUpdateDraft {
    return {
      name: detail.name,
      description: detail.description || '',
      cidr: detail.defaultSubnetCidr || '',
    };
  }

  emptySubnetDraft(): SubnetCreateDraft {
    return {
      name: '',
      cidr: '',
      gatewayIp: '',
      allocationStartIp: '',
      allocationEndIp: '',
    };
  }

  optionalValue(value: string): string | undefined {
    const trimmed = value.trim();
    return trimmed ? trimmed : undefined;
  }

  private isLikelyCIDR(value: string): boolean {
    return /^\d{1,3}(?:\.\d{1,3}){3}\/\d{1,2}$/.test(value.trim());
  }

  private isLikelyIPv4(value: string): boolean {
    const trimmed = value.trim();
    if (!/^\d{1,3}(?:\.\d{1,3}){3}$/.test(trimmed)) {
      return false;
    }
    return trimmed.split('.').every((item) => Number(item) >= 0 && Number(item) <= 255);
  }
}
