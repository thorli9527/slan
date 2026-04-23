import { Injectable } from '@angular/core';

import {
  AuthCallbackStatusResponse,
  AuthResponse,
  CompleteAuthCallbackRequest,
  Device,
  Network,
  NetworkAssignment,
  NetworkDetail,
  NetworkHome,
  Subnet,
} from './api-contracts';
import { AuthMode as AuthModeLocal } from './ui-models';

type RequestOptions = {
  token?: string;
  init?: RequestInit;
};

type RegisterDeviceInput = {
  name: string;
  platform: string;
  machineId: string;
  publicKey: string;
};

type CreateNetworkInput = {
  name: string;
  description: string;
  cidr: string;
  bindDeviceId: string;
};

type UpdateNetworkInput = {
  name: string;
  description: string;
  cidr: string;
};

type CreateSubnetInput = {
  name: string;
  cidr: string;
  gatewayIp?: string;
  allocationStartIp?: string;
  allocationEndIp?: string;
};

@Injectable({ providedIn: 'root' })
export class ConsoleApiService {
  private readonly serverBase = '/api';

  authenticate(mode: AuthModeLocal, email: string, password: string): Promise<AuthResponse> {
    const path = mode === 'login' ? '/auth/login' : '/auth/register';
    return this.request<AuthResponse>(path, {
      init: {
        method: 'POST',
        body: JSON.stringify({ email, password })
      }
    });
  }

  getHome(token: string): Promise<NetworkHome> {
    return this.request<NetworkHome>('/networks/home', { token });
  }

  getNetworkDetail(token: string, networkId: string): Promise<NetworkDetail> {
    return this.request<NetworkDetail>(`/networks/${networkId}`, { token });
  }

  getSubnets(token: string, networkId: string): Promise<{ items: Subnet[] }> {
    return this.request<{ items: Subnet[] }>(`/networks/${networkId}/subnets`, { token });
  }

  getAssignments(token: string, networkId: string): Promise<{ items: NetworkAssignment[] }> {
    return this.request<{ items: NetworkAssignment[] }>(`/networks/${networkId}/assignments`, { token });
  }

  createNetwork(token: string, input: CreateNetworkInput): Promise<Network> {
    return this.request<Network>('/networks', {
      token,
      init: {
        method: 'POST',
        body: JSON.stringify(input)
      }
    });
  }

  joinByOwnerEmail(token: string, ownerEmail: string, deviceId: string): Promise<void> {
    return this.request('/networks/join-by-owner-email', {
      token,
      init: {
        method: 'POST',
        body: JSON.stringify({ ownerEmail, deviceId })
      }
    });
  }

  joinByKey(token: string, joinKey: string, deviceId: string): Promise<void> {
    return this.request('/networks/join-by-key', {
      token,
      init: {
        method: 'POST',
        body: JSON.stringify({ joinKey, deviceId })
      }
    });
  }

  updateNetwork(token: string, networkId: string, input: UpdateNetworkInput): Promise<void> {
    return this.request(`/networks/${networkId}`, {
      token,
      init: {
        method: 'PUT',
        body: JSON.stringify(input)
      }
    });
  }

  updateNetworkJoinKey(token: string, networkId: string, joinKey: string): Promise<NetworkDetail> {
    return this.request<NetworkDetail>(`/networks/${networkId}/join-key`, {
      token,
      init: {
        method: 'PUT',
        body: JSON.stringify({ joinKey })
      }
    });
  }

  createSubnet(token: string, networkId: string, input: CreateSubnetInput): Promise<void> {
    return this.request(`/networks/${networkId}/subnets`, {
      token,
      init: {
        method: 'POST',
        body: JSON.stringify(input)
      }
    });
  }

  updateAttachmentIp(token: string, networkId: string, attachmentId: string, virtualIp: string): Promise<void> {
    return this.request(`/networks/${networkId}/attachments/${attachmentId}/ip`, {
      token,
      init: {
        method: 'PUT',
        body: JSON.stringify({ virtualIp })
      }
    });
  }

  updateAttachmentRemark(token: string, networkId: string, attachmentId: string, remark: string): Promise<NetworkAssignment> {
    return this.request<NetworkAssignment>(`/networks/${networkId}/attachments/${attachmentId}/remark`, {
      token,
      init: {
        method: 'PUT',
        body: JSON.stringify({ remark })
      }
    });
  }

  switchNetwork(token: string, networkId: string, deviceId: string): Promise<void> {
    return this.request(`/networks/${networkId}/switch`, {
      token,
      init: {
        method: 'POST',
        body: JSON.stringify({ deviceId })
      }
    });
  }

  registerDevice(token: string, input: RegisterDeviceInput): Promise<Device> {
    return this.request<Device>('/devices/register', {
      token,
      init: {
        method: 'POST',
        body: JSON.stringify(input)
      }
    });
  }

  listDevices(token: string): Promise<{ items: Device[] }> {
    return this.request<{ items: Device[] }>('/devices', { token });
  }

  getCallbackStatus(callbackId: string): Promise<AuthCallbackStatusResponse> {
    return this.request<AuthCallbackStatusResponse>(
      `/auth/callback-status/${encodeURIComponent(callbackId)}`,
      {}
    );
  }

  completeCallback(
    callbackId: string,
    payload: CompleteAuthCallbackRequest,
  ): Promise<void> {
    return this.request(`/auth/callback-status/${encodeURIComponent(callbackId)}/complete`, {
      init: {
        method: 'POST',
        body: JSON.stringify(payload)
      },
    });
  }

  private async request<T = unknown>(path: string, options: RequestOptions = {}): Promise<T> {
    const headers = new Headers(options.init?.headers || {});
    headers.set('Content-Type', 'application/json');
    if (options.token?.trim()) {
      headers.set('Authorization', `Bearer ${options.token}`);
    }
    const response = await fetch(`${this.serverBase}${path}`, { ...options.init, headers });
    const text = await response.text();
    const payload = text ? JSON.parse(text) : null;
    if (!response.ok) {
      const raw = payload?.message || `request failed: ${response.status}`;
      throw new Error(this.normalizeErrorMessage(raw));
    }
    return payload as T;
  }

  private normalizeErrorMessage(message: string): string {
    return message
      .replace(/^invalid argument:\s*/i, '')
      .replace(/^conflict:\s*/i, '')
      .replace(/^forbidden:\s*/i, '')
      .replace(/^not found:\s*/i, '');
  }
}
