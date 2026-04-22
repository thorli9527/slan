import { Injectable } from '@angular/core';

import { ConsoleApiService } from './console-api.service';
import { AuthResponse, Device } from './api-contracts';
import { AuthMode } from './ui-models';

export type LoginClientContext = {
  authMode?: AuthMode;
  callbackId?: string;
  deviceId: string;
  clientPlatform: string;
  clientName: string;
};

export type ManagedDeviceState = {
  deviceId: string;
  devices: Device[];
  currentDeviceId: string;
  callbackDeviceId: string;
};

@Injectable({ providedIn: 'root' })
export class ConsoleSessionService {
  constructor(private readonly api: ConsoleApiService) {}

  readLoginClientContext(url: string): LoginClientContext {
    const params = new URL(url).searchParams;
    const authMode = params.get('auth');
    const deviceId = params.get('deviceId');
    const clientDeviceId = params.get('clientDeviceId');
    const callbackId = params.get('callbackId');
    const clientPlatform = params.get('clientPlatform');
    const clientName = params.get('clientName');
    const resolvedDeviceId = (deviceId || clientDeviceId || '').trim();
    const resolvedCallbackId = callbackId?.trim() || resolvedDeviceId || undefined;
    return {
      authMode: authMode === 'login' || authMode === 'register' ? authMode : undefined,
      callbackId: resolvedCallbackId,
      deviceId: resolvedDeviceId,
      clientPlatform: clientPlatform?.trim() || 'desktop',
      clientName: clientName?.trim() || 'SLAN Client',
    };
  }

  persistAuth(auth: AuthResponse): void {
    localStorage.setItem('slan.accessToken', auth.accessToken);
    localStorage.setItem('slan.userId', auth.userId);
  }

  clearCachedAuth(): void {
    localStorage.removeItem('slan.accessToken');
    localStorage.removeItem('slan.userId');
    localStorage.removeItem('slan.deviceId');
  }

  persistCurrentDeviceId(deviceId: string): void {
    if (deviceId.trim()) {
      localStorage.setItem('slan.deviceId', deviceId);
      return;
    }
    localStorage.removeItem('slan.deviceId');
  }

  ensureMachineId(): string {
    const existing = localStorage.getItem('slan.machineId');
    if (existing) {
      return existing;
    }
    const created = `web-${crypto.randomUUID()}`;
    localStorage.setItem('slan.machineId', created);
    return created;
  }

  async loadDevices(token: string, currentDeviceId: string): Promise<{ devices: Device[]; currentDeviceId: string }> {
    const devices = (await this.api.listDevices(token)).items;
    const resolvedCurrentDeviceId = currentDeviceId || devices[0]?.deviceId || '';
    if (resolvedCurrentDeviceId) {
      this.persistCurrentDeviceId(resolvedCurrentDeviceId);
    }
    return { devices, currentDeviceId: resolvedCurrentDeviceId };
  }

  async ensureManagementDevice(input: {
    token: string;
    currentDeviceId: string;
    callbackDeviceId: string;
    preferredMachineId: string;
    clientPlatform: string;
    clientName: string;
  }): Promise<ManagedDeviceState> {
    if (!input.token) {
      throw new Error('missing access token');
    }
    if (input.callbackDeviceId) {
      const loaded = await this.loadDevices(input.token, input.currentDeviceId);
      return {
        deviceId: input.callbackDeviceId,
        devices: loaded.devices,
        currentDeviceId: loaded.currentDeviceId,
        callbackDeviceId: input.callbackDeviceId,
      };
    }
    if (input.preferredMachineId) {
      const resolved = await this.api.registerDevice(input.token, {
        name: input.clientName,
        platform: input.clientPlatform,
        machineId: input.preferredMachineId,
        publicKey: `web-console-${input.preferredMachineId}`,
      });
      const loaded = await this.loadDevices(input.token, resolved.deviceId);
      return {
        deviceId: resolved.deviceId,
        devices: loaded.devices,
        currentDeviceId: resolved.deviceId,
        callbackDeviceId: resolved.deviceId,
      };
    }

    if (input.currentDeviceId) {
      const loaded = await this.loadDevices(input.token, input.currentDeviceId);
      const stillExists = loaded.devices.some((item) => item.deviceId === input.currentDeviceId);
      if (stillExists) {
        return {
          deviceId: input.currentDeviceId,
          devices: loaded.devices,
          currentDeviceId: loaded.currentDeviceId,
          callbackDeviceId: '',
        };
      }
      this.persistCurrentDeviceId('');
      input.currentDeviceId = '';
    }

    const loaded = await this.loadDevices(input.token, input.currentDeviceId);
    if (loaded.currentDeviceId) {
      return {
        deviceId: loaded.currentDeviceId,
        devices: loaded.devices,
        currentDeviceId: loaded.currentDeviceId,
        callbackDeviceId: '',
      };
    }

    const machineId = this.ensureMachineId();
    const created = await this.api.registerDevice(input.token, {
      name: 'Web Console Device',
      platform: 'web',
      machineId,
      publicKey: `web-console-${machineId}`,
    });
    const devices = [...loaded.devices, created];
    this.persistCurrentDeviceId(created.deviceId);
    return {
      deviceId: created.deviceId,
      devices,
      currentDeviceId: created.deviceId,
      callbackDeviceId: '',
    };
  }

  buildCallbackPayload(input: {
    callbackId: string;
    auth: AuthResponse;
    deviceId?: string;
    userLabel?: string;
    action?: string;
  }): {
    callbackId: string;
    accessToken: string;
    userId: string;
    refreshToken?: string;
    expiresIn: number;
    deviceId?: string;
    userLabel?: string;
    action?: string;
  } {
    return {
      callbackId: input.callbackId.trim(),
      accessToken: input.auth.accessToken,
      userId: input.auth.userId,
      refreshToken: input.auth.refreshToken?.trim() || undefined,
      expiresIn: input.auth.expiresIn,
      deviceId: input.deviceId?.trim() || undefined,
      userLabel: input.userLabel?.trim() || undefined,
      action: input.action?.trim() || undefined,
    };
  }
}
