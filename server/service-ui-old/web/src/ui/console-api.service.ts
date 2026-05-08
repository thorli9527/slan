import { Injectable } from '@angular/core';

import {
  AuthCallbackStatusResponse,
  AuthResponse,
  ChangePasswordRequest,
  CompleteAuthCallbackRequest,
  ConsoleLoginKeyResponse,
  Device,
  PlanStatus,
  Network,
  NetworkAssignment,
  NetworkQuality,
  NetworkMember,
  NetworkDetail,
  NetworkHome,
  NetworkJoinResult,
  PublicSystemConfig,
  RelayPolicyRequest,
  RelayPolicyResponse,
  RelayPolicyTemplate,
  RelayPolicyTemplateRequest,
  Subnet,
  SubnetAttachment,
  UpdateNetworkDNSRequest,
} from './api-contracts';
import { AuthMode as AuthModeLocal } from './ui-models';

export class ConsoleApiError extends Error {
  constructor(
    readonly status: number,
    readonly code: string,
    message: string,
  ) {
    super(message);
  }

  get isUnauthorized(): boolean {
    return this.status === 401 || this.code === 'UNAUTHORIZED';
  }

  get isDeviceLimitExceeded(): boolean {
    return this.status === 403 && this.code === 'DEVICE_LIMIT_EXCEEDED';
  }
}

type RequestOptions = {
  token?: string;
  init?: RequestInit;
};

type RegisterDeviceInput = {
  deviceId?: string;
  name: string;
  platform: string;
  publicKey: string;
};

type CreateNetworkInput = {
  name: string;
  description: string;
  cidr?: string;
  allocationStartIp?: string;
  allocationEndIp?: string;
  bindDeviceId: string;
};

type UpdateNetworkInput = {
  name: string;
  description: string;
  cidr: string;
  allocationStartIp?: string;
  allocationEndIp?: string;
};

@Injectable({ providedIn: 'root' })
export class ConsoleApiService {
  private readonly serverBase = '/api';

  getPublicConfig(): Promise<PublicSystemConfig> {
    return this.request<PublicSystemConfig>('/system/public-config');
  }

  authenticate(
    mode: AuthModeLocal,
    email: string,
    password: string,
    deviceId?: string,
  ): Promise<AuthResponse> {
    const path = mode === 'login' ? '/auth/login' : '/auth/register';
    return this.request<AuthResponse>(path, {
      init: {
        method: 'POST',
        body: JSON.stringify({
          email,
          password,
          deviceId: deviceId?.trim() || undefined,
        })
      }
    });
  }

  consumeConsoleLoginKey(loginKey: string): Promise<AuthResponse> {
    return this.request<AuthResponse>('/auth/console-login', {
      init: {
        method: 'POST',
        body: JSON.stringify({ loginKey })
      }
    });
  }

  createConsoleLoginKey(token: string, deviceId?: string): Promise<ConsoleLoginKeyResponse> {
    return this.request<ConsoleLoginKeyResponse>('/auth/console-login-key', {
      token,
      init: {
        method: 'POST',
        body: JSON.stringify({ deviceId: deviceId?.trim() || undefined })
      }
    });
  }

  getHome(token: string): Promise<NetworkHome> {
    return this.request<NetworkHome>('/networks/home', { token });
  }

  listNetworks(token: string): Promise<{ items: Network[] }> {
    return this.request<{ items: Network[] }>('/networks', { token });
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

  getNetworkQuality(token: string, networkId: string, hours = 1): Promise<NetworkQuality> {
    return this.request<NetworkQuality>(`/networks/${networkId}/quality?hours=${encodeURIComponent(String(hours))}`, { token });
  }

  publishRelayPolicy(token: string, networkId: string, input: RelayPolicyRequest): Promise<RelayPolicyResponse> {
    return this.request<RelayPolicyResponse>(`/networks/${networkId}/quality/relay-policy`, {
      token,
      init: {
        method: 'POST',
        body: JSON.stringify(input)
      }
    });
  }

  listRelayPolicyTemplates(token: string, networkId: string): Promise<{ items: RelayPolicyTemplate[] }> {
    return this.request<{ items: RelayPolicyTemplate[] }>(`/networks/${networkId}/quality/templates`, { token });
  }

  createRelayPolicyTemplate(token: string, networkId: string, input: RelayPolicyTemplateRequest): Promise<RelayPolicyTemplate> {
    return this.request<RelayPolicyTemplate>(`/networks/${networkId}/quality/templates`, {
      token,
      init: {
        method: 'POST',
        body: JSON.stringify(input)
      }
    });
  }

  updateRelayPolicyTemplate(token: string, networkId: string, templateId: string, input: RelayPolicyTemplateRequest): Promise<RelayPolicyTemplate> {
    return this.request<RelayPolicyTemplate>(`/networks/${networkId}/quality/templates/${encodeURIComponent(templateId)}`, {
      token,
      init: {
        method: 'PUT',
        body: JSON.stringify(input)
      }
    });
  }

  deleteRelayPolicyTemplate(token: string, networkId: string, templateId: string): Promise<void> {
    return this.request<void>(`/networks/${networkId}/quality/templates/${encodeURIComponent(templateId)}`, {
      token,
      init: { method: 'DELETE' }
    });
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

  joinByKey(token: string, joinKey: string, deviceId: string): Promise<NetworkJoinResult> {
    return this.request<NetworkJoinResult>('/networks/join-by-key', {
      token,
      init: {
        method: 'POST',
        body: JSON.stringify({ joinKey, deviceId })
      }
    });
  }

  updateNetwork(token: string, networkId: string, input: UpdateNetworkInput): Promise<Network> {
    return this.request<Network>(`/networks/${networkId}`, {
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

  updateNetworkDns(token: string, networkId: string, input: UpdateNetworkDNSRequest): Promise<NetworkDetail> {
    return this.request<NetworkDetail>(`/networks/${networkId}/dns`, {
      token,
      init: {
        method: 'PUT',
        body: JSON.stringify(input)
      }
    });
  }

  updateNetworkMemberStatus(token: string, networkId: string, memberId: string, status: 'active' | 'rejected'): Promise<NetworkMember> {
    return this.request<NetworkMember>(`/networks/${networkId}/members/${memberId}/status`, {
      token,
      init: {
        method: 'PUT',
        body: JSON.stringify({ status })
      }
    });
  }

  updateAttachmentIp(token: string, networkId: string, attachmentId: string, virtualIp: string): Promise<SubnetAttachment> {
    return this.request<SubnetAttachment>(`/networks/${networkId}/attachments/${attachmentId}/ip`, {
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

  updateAttachmentStatus(token: string, networkId: string, attachmentId: string, status: 'active' | 'disabled'): Promise<NetworkAssignment> {
    return this.request<NetworkAssignment>(`/networks/${networkId}/attachments/${attachmentId}/status`, {
      token,
      init: {
        method: 'PUT',
        body: JSON.stringify({ status })
      }
    });
  }

  switchNetwork(token: string, networkId: string, deviceId: string): Promise<NetworkJoinResult> {
    return this.request<NetworkJoinResult>(`/networks/${networkId}/switch`, {
      token,
      init: {
        method: 'POST',
        body: JSON.stringify({ deviceId })
      }
    });
  }

  activateNetwork(token: string, networkId: string, deviceId: string): Promise<NetworkJoinResult> {
    return this.request<NetworkJoinResult>(`/networks/${networkId}/activate`, {
      token,
      init: {
        method: 'POST',
        body: JSON.stringify({ deviceId })
      }
    });
  }

  changePassword(token: string, input: ChangePasswordRequest): Promise<void> {
    return this.request<void>('/auth/password', {
      token,
      init: {
        method: 'PUT',
        body: JSON.stringify(input)
      }
    });
  }

  getPlan(token: string): Promise<PlanStatus> {
    return this.request<PlanStatus>('/plan', { token });
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
    token?: string,
  ): Promise<void> {
    return this.request(`/auth/callback-status/${encodeURIComponent(callbackId)}/complete`, {
      token,
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
      throw new ConsoleApiError(
        response.status,
        payload?.code || '',
        this.normalizeErrorMessage(raw),
      );
    }
    return payload as T;
  }

  private normalizeErrorMessage(message: string): string {
    return message
      .replace(/^invalid argument:\s*/i, '')
      .replace(/^conflict:\s*/i, '')
      .replace(/^unauthorized:\s*/i, '')
      .replace(/^forbidden:\s*/i, '')
      .replace(/^device limit exceeded:\s*/i, '')
      .replace(/^not found:\s*/i, '');
  }
}
