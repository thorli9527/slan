import { Injectable } from '@angular/core';

import { NetworkDetail } from './api-contracts';

export type NetworkUpdateDraft = {
  name: string;
  description: string;
  cidr: string;
};

@Injectable({ providedIn: 'root' })
export class ConsoleNetworkFormService {
  cidrFromAddressAndMask(address: string, subnetMask: string): string {
    const addressValue = this.parseIPv4(address, 'ip address is invalid');
    const maskValue = this.parseIPv4(subnetMask, 'subnet mask is invalid');
    const prefix = this.subnetMaskToPrefix(maskValue);
    const network = addressValue & maskValue;
    return `${this.formatIPv4(network)}/${prefix}`;
  }

  addressAndMaskFromCidr(cidr: string): { address: string; subnetMask: string } {
    const [address, prefixText] = cidr.trim().split('/');
    if (!this.isLikelyIPv4(address || '')) {
      return { address: '100.64.0.0', subnetMask: '255.255.252.0' };
    }
    const prefix = Number(prefixText);
    if (!Number.isInteger(prefix) || prefix < 1 || prefix > 30) {
      return { address, subnetMask: '255.255.255.0' };
    }
    const mask = (0xffffffff << (32 - prefix)) >>> 0;
    return { address, subnetMask: this.formatIPv4(mask) };
  }

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

  private parseIPv4(value: string, errorMessage: string): number {
    const trimmed = value.trim();
    if (!this.isLikelyIPv4(trimmed)) {
      throw new Error(errorMessage);
    }
    return trimmed.split('.').reduce((acc, item) => ((acc << 8) | Number(item)) >>> 0, 0);
  }

  private subnetMaskToPrefix(mask: number): number {
    let seenZero = false;
    let prefix = 0;
    for (let bit = 31; bit >= 0; bit--) {
      const isOne = ((mask >>> bit) & 1) === 1;
      if (isOne && seenZero) {
        throw new Error('subnet mask is invalid');
      }
      if (isOne) {
        prefix++;
      } else {
        seenZero = true;
      }
    }
    if (prefix < 1 || prefix > 30) {
      throw new Error('subnet mask must allow usable host addresses');
    }
    return prefix;
  }

  private formatIPv4(value: number): string {
    return [
      (value >>> 24) & 255,
      (value >>> 16) & 255,
      (value >>> 8) & 255,
      value & 255,
    ].join('.');
  }
}
