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
    if (input.mode === 'login') {
      return { auth, managedDevice };
    }
    const workspace = await this.workspaceService.loadWorkspace(auth.accessToken);
    return { auth, managedDevice, workspace };
  }

  async refreshWorkspace(input: EnsureDeviceInput): Promise<RefreshWorkspaceResult> {
    const managedDevice = await this.ensureManagementDevice(input);
    const workspace = await this.workspaceService.loadWorkspace(input.token);
    return { managedDevice, workspace };
  }

  reloadDevices(token: string, currentDeviceId: string) {
    return this.sessionService.loadDevices(token, currentDeviceId);
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
    createCidr: string;
    deviceState: Omit<EnsureDeviceInput, 'token'>;
  }): Promise<{ network: Network; refreshed: RefreshWorkspaceResult }> {
    this.networkFormService.validateNetworkCidr(input.createCidr);
    const managedDevice = await this.ensureManagementDevice({
      token: input.token,
      ...input.deviceState,
    });
    const network = await this.api.createNetwork(input.token, {
      name: input.createName.trim(),
      description: input.createDescription.trim(),
      cidr: input.createCidr.trim(),
      bindDeviceId: managedDevice.deviceId,
    });
    const refreshed = await this.refreshWorkspace({
      token: input.token,
      ...managedDeviceToEnsureInput(managedDevice, input.deviceState),
      callbackDeviceId: managedDevice.callbackDeviceId || input.deviceState.callbackDeviceId,
    });
    await this.workspaceService.switchNetwork(input.token, network.networkId, refreshed.managedDevice.deviceId);
    const switched = await this.refreshWorkspace({
      token: input.token,
      ...managedDeviceToEnsureInput(refreshed.managedDevice, input.deviceState),
      callbackDeviceId: refreshed.managedDevice.callbackDeviceId || input.deviceState.callbackDeviceId,
    });
    return { network, refreshed: switched };
  }

  async joinByOwnerEmail(input: {
    token: string;
    ownerEmail: string;
    deviceState: Omit<EnsureDeviceInput, 'token'>;
  }): Promise<RefreshWorkspaceResult> {
    const ownerEmail = input.ownerEmail.trim();
    if (!ownerEmail) {
      throw new Error('owner email is required');
    }
    const managedDevice = await this.ensureManagementDevice({
      token: input.token,
      ...input.deviceState,
    });
    await this.api.joinByOwnerEmail(input.token, ownerEmail, managedDevice.deviceId);
    return this.refreshWorkspace({
      token: input.token,
      ...managedDeviceToEnsureInput(managedDevice, input.deviceState),
      callbackDeviceId: managedDevice.callbackDeviceId || input.deviceState.callbackDeviceId,
    });
  }

  async joinByKey(input: {
    token: string;
    joinKey: string;
    deviceState: Omit<EnsureDeviceInput, 'token'>;
  }): Promise<RefreshWorkspaceResult> {
    const joinKey = input.joinKey.trim();
    if (!joinKey) {
      throw new Error('join key is required');
    }
    const managedDevice = await this.ensureManagementDevice({
      token: input.token,
      ...input.deviceState,
    });
    await this.api.joinByKey(input.token, joinKey, managedDevice.deviceId);
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
    deviceState: Omit<EnsureDeviceInput, 'token'>;
  }): Promise<RefreshWorkspaceResult> {
    this.networkFormService.validateNetworkCidr(input.cidr);
    await this.api.updateNetwork(input.token, input.networkId, {
      name: input.name.trim(),
      description: input.description.trim(),
      cidr: input.cidr.trim(),
    });
    return this.refreshWorkspace({
      token: input.token,
      ...input.deviceState,
    });
  }

  async createSubnet(input: {
    token: string;
    networkId: string;
    draft: {
      name: string;
      cidr: string;
      gatewayIp: string;
      allocationStartIp: string;
      allocationEndIp: string;
    };
    deviceState: Omit<EnsureDeviceInput, 'token'>;
  }): Promise<RefreshWorkspaceResult> {
    this.networkFormService.validateSubnetDraft(input.draft);
    await this.api.createSubnet(input.token, input.networkId, {
      name: input.draft.name.trim(),
      cidr: input.draft.cidr.trim(),
      gatewayIp: this.networkFormService.optionalValue(input.draft.gatewayIp),
      allocationStartIp: this.networkFormService.optionalValue(input.draft.allocationStartIp),
      allocationEndIp: this.networkFormService.optionalValue(input.draft.allocationEndIp),
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
