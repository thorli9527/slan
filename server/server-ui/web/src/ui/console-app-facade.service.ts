import { Injectable } from '@angular/core';

import { ConsoleApiService } from './console-api.service';
import { ConsoleNetworkFormService } from './console-network-form.service';
import { ConsoleSessionService, ManagedDeviceState } from './console-session.service';
import { ConsoleWorkspaceService, WorkspaceSnapshot } from './console-workspace.service';
import { AuthResponse, Network } from './api-contracts';
import { AuthMode } from './ui-models';

export type RefreshWorkspaceResult = {
  managedDevice: ManagedDeviceState;
  workspace: WorkspaceSnapshot;
};

export type AuthenticateResult = {
  auth: AuthResponse;
  managedDevice: ManagedDeviceState;
  workspace?: WorkspaceSnapshot;
};

@Injectable({ providedIn: 'root' })
export class ConsoleAppFacadeService {
  constructor(
    private readonly api: ConsoleApiService,
    private readonly networkFormService: ConsoleNetworkFormService,
    private readonly sessionService: ConsoleSessionService,
    private readonly workspaceService: ConsoleWorkspaceService,
  ) {}

  async authenticate(input: {
    mode: AuthMode;
    email: string;
    password: string;
    loginDeviceId?: string;
    tokenDeviceState: Omit<EnsureDeviceInput, 'token'>;
  }): Promise<AuthenticateResult> {
    if (!input.email.trim()) {
      throw new Error('email is required');
    }
    if (!input.password.trim()) {
      throw new Error('password is required');
    }
    if (input.mode === 'register' && input.password.trim().length < 8) {
      throw new Error('password must be at least 8 characters');
    }

    const auth = await this.api.authenticate(
      input.mode,
      input.email,
      input.password,
      input.loginDeviceId,
    );
    this.sessionService.persistAuth(auth);
    const managedDevice = await this.ensureManagementDevice({
      token: auth.accessToken,
      ...input.tokenDeviceState,
    });
    const workspace = await this.workspaceService.loadWorkspace(auth.accessToken);
    return { auth, managedDevice, workspace };
  }

  async refreshWorkspace(input: EnsureDeviceInput): Promise<RefreshWorkspaceResult> {
    const managedDevice = await this.ensureManagementDevice(input);
    const workspace = await this.workspaceService.loadWorkspace(input.token);
    return { managedDevice, workspace };
  }

  completeCallback(callbackId: string, payload: {
    accessToken: string;
    userId: string;
    refreshToken?: string;
    expiresIn: number;
    deviceId?: string;
    userLabel?: string;
    action?: string;
  }): Promise<void> {
    return this.api.completeCallback(callbackId, payload);
  }

  async createOwnNetwork(input: {
    token: string;
    createName: string;
    createDescription: string;
    createNetworkIp: string;
    createSubnetMask: string;
    allocationStartIp?: string;
    allocationEndIp?: string;
    deviceState: Omit<EnsureDeviceInput, 'token'>;
  }): Promise<{ network: Network; refreshed: RefreshWorkspaceResult }> {
    const cidr = this.networkFormService.cidrFromAddressAndMask(input.createNetworkIp, input.createSubnetMask);
    this.networkFormService.validateNetworkCidr(cidr);
    const managedDevice = await this.ensureManagementDevice({
      token: input.token,
      ...input.deviceState,
    });
    const network = await this.api.createNetwork(input.token, {
      name: input.createName.trim(),
      description: input.createDescription.trim(),
      cidr,
      allocationStartIp: this.networkFormService.optionalValue(input.allocationStartIp || ''),
      allocationEndIp: this.networkFormService.optionalValue(input.allocationEndIp || ''),
      bindDeviceId: managedDevice.deviceId,
    });
    const refreshed = await this.refreshWorkspace({
      token: input.token,
      ...managedDeviceToEnsureInput(managedDevice, input.deviceState),
      callbackDeviceId: managedDevice.callbackDeviceId || input.deviceState.callbackDeviceId,
    });
    await this.workspaceService.switchNetwork(input.token, network.networkId, refreshed.managedDevice.deviceId);
    await this.workspaceService.activateNetwork(input.token, network.networkId, refreshed.managedDevice.deviceId);
    const switched = await this.refreshWorkspace({
      token: input.token,
      ...managedDeviceToEnsureInput(refreshed.managedDevice, input.deviceState),
      callbackDeviceId: refreshed.managedDevice.callbackDeviceId || input.deviceState.callbackDeviceId,
    });
    return { network, refreshed: switched };
  }

  async joinByKey(input: {
    token: string;
    joinKey: string;
    deviceState: Omit<EnsureDeviceInput, 'token'>;
  }): Promise<RefreshWorkspaceResult> {
    const joinKey = input.joinKey.trim();
    if (!joinKey) {
      throw new Error('invite code is required');
    }
    const managedDevice = await this.ensureManagementDevice({
      token: input.token,
      ...input.deviceState,
    });
    const result = await this.api.joinByKey(input.token, joinKey, managedDevice.deviceId);
    const networkId = result.attachment?.networkId || result.member?.networkId;
    if (networkId && result.member.status === 'active') {
      await this.workspaceService.activateNetwork(input.token, networkId, managedDevice.deviceId);
    }
    return this.refreshWorkspace({
      token: input.token,
      ...managedDeviceToEnsureInput(managedDevice, input.deviceState),
      callbackDeviceId: managedDevice.callbackDeviceId || input.deviceState.callbackDeviceId,
    });
  }

  async updateOwnedNetwork(input: {
    token: string;
    networkId: string;
    name: string;
    description: string;
    cidr: string;
    allocationStartIp?: string;
    allocationEndIp?: string;
    deviceState: Omit<EnsureDeviceInput, 'token'>;
  }): Promise<RefreshWorkspaceResult> {
    this.networkFormService.validateNetworkCidr(input.cidr);
    await this.api.updateNetwork(input.token, input.networkId, {
      name: input.name.trim(),
      description: input.description.trim(),
      cidr: input.cidr.trim(),
      allocationStartIp: this.networkFormService.optionalValue(input.allocationStartIp || ''),
      allocationEndIp: this.networkFormService.optionalValue(input.allocationEndIp || ''),
    });
    return this.refreshWorkspace({
      token: input.token,
      ...input.deviceState,
    });
  }

  async updateAttachmentIp(input: {
    token: string;
    networkId: string;
    attachmentId: string;
    virtualIp: string;
    deviceState: Omit<EnsureDeviceInput, 'token'>;
  }): Promise<RefreshWorkspaceResult> {
    const virtualIp = input.virtualIp.trim();
    this.networkFormService.validateAttachmentIp(virtualIp);
    await this.api.updateAttachmentIp(input.token, input.networkId, input.attachmentId, virtualIp);
    return this.refreshWorkspace({
      token: input.token,
      ...input.deviceState,
    });
  }

  async updateNetworkJoinKey(input: {
    token: string;
    networkId: string;
    joinKey: string;
    deviceState: Omit<EnsureDeviceInput, 'token'>;
  }): Promise<RefreshWorkspaceResult> {
    await this.api.updateNetworkJoinKey(input.token, input.networkId, input.joinKey.trim());
    return this.refreshWorkspace({
      token: input.token,
      ...input.deviceState,
    });
  }

  async updateNetworkDns(input: {
    token: string;
    networkId: string;
    servers: string[];
    searchDomains: string[];
    wildcards: string[];
    deviceState: Omit<EnsureDeviceInput, 'token'>;
  }): Promise<RefreshWorkspaceResult> {
    await this.api.updateNetworkDns(input.token, input.networkId, {
      servers: input.servers,
      searchDomains: input.searchDomains,
      wildcards: input.wildcards,
    });
    return this.refreshWorkspace({
      token: input.token,
      ...input.deviceState,
    });
  }

  async updateNetworkMemberStatus(input: {
    token: string;
    networkId: string;
    memberId: string;
    status: 'active' | 'rejected';
    deviceState: Omit<EnsureDeviceInput, 'token'>;
  }): Promise<RefreshWorkspaceResult> {
    await this.api.updateNetworkMemberStatus(input.token, input.networkId, input.memberId, input.status);
    return this.refreshWorkspace({
      token: input.token,
      ...input.deviceState,
    });
  }

  async updateAttachmentRemark(input: {
    token: string;
    networkId: string;
    attachmentId: string;
    remark: string;
    deviceState: Omit<EnsureDeviceInput, 'token'>;
  }): Promise<RefreshWorkspaceResult> {
    await this.api.updateAttachmentRemark(input.token, input.networkId, input.attachmentId, input.remark.trim());
    return this.refreshWorkspace({
      token: input.token,
      ...input.deviceState,
    });
  }

  async deleteAttachment(input: {
    token: string;
    networkId: string;
    attachmentId: string;
    deviceState: Omit<EnsureDeviceInput, 'token'>;
  }): Promise<RefreshWorkspaceResult> {
    await this.api.deleteAttachment(input.token, input.networkId, input.attachmentId);
    return this.refreshWorkspace({
      token: input.token,
      ...input.deviceState,
    });
  }

  private ensureManagementDevice(input: EnsureDeviceInput): Promise<ManagedDeviceState> {
    return this.sessionService.ensureManagementDevice(input);
  }

}

type EnsureDeviceInput = {
  token: string;
  currentDeviceId: string;
  callbackDeviceId: string;
  preferredMachineId: string;
  clientPlatform: string;
  clientName: string;
};

function managedDeviceToEnsureInput(
  managedDevice: ManagedDeviceState,
  fallback: Omit<EnsureDeviceInput, 'token'>,
): Omit<EnsureDeviceInput, 'token'> {
  return {
    currentDeviceId: managedDevice.currentDeviceId,
    callbackDeviceId: managedDevice.callbackDeviceId || fallback.callbackDeviceId,
    preferredMachineId: fallback.preferredMachineId,
    clientPlatform: fallback.clientPlatform,
    clientName: fallback.clientName,
  };
}
