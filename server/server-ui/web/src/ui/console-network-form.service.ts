import { Injectable } from '@angular/core';

import { NetworkDetail } from './api-contracts';

export type NetworkUpdateDraft = {
  name: string;
  description: string;
  cidr: string;
};

@Injectable({ providedIn: 'root' })
export class ConsoleNetworkFormService {
  validateNetworkCidr(cidr: string): void {
    if (!cidr.trim()) {
      return;
    }
    if (!this.isLikelyCIDR(cidr)) {
      throw new Error('default cidr is invalid');
    }
  }

  validateAttachmentIp(virtualIp: string): void {
    if (!this.isLikelyIPv4(virtualIp)) {
      throw new Error('virtual ip is invalid');
    }
  }

  buildNetworkUpdateDraft(detail: NetworkDetail): NetworkUpdateDraft {
    return {
      name: detail.name,
      description: detail.description || '',
      cidr: detail.defaultSubnetCidr || '',
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
