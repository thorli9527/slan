import { Injectable } from '@angular/core';

import { ConsoleApiService } from './console-api.service';
import { AuthResponse, Device } from './api-contracts';
import { AuthMode } from './ui-models';

export type LoginClientContext = {
  authMode?: AuthMode;
  callbackId?: string;
  consoleLoginKey?: string;
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
    const consoleLoginKey = params.get('consoleLoginKey');
    const clientPlatform = params.get('clientPlatform');
    const clientName = params.get('clientName');
    const resolvedDeviceId = this.usableClientDeviceId(deviceId || clientDeviceId || '');
    const resolvedCallbackId = callbackId?.trim() || undefined;
    return {
      authMode: authMode === 'login' || authMode === 'register' ? authMode : undefined,
      callbackId: resolvedCallbackId,
      consoleLoginKey: consoleLoginKey?.trim() || undefined,
      deviceId: resolvedDeviceId,
      clientPlatform: clientPlatform?.trim() || this.detectClientPlatform(),
      clientName: clientName?.trim() || 'SLAN Client',
    };
  }

  usableClientDeviceId(deviceId: string | null | undefined): string {
    const value = (deviceId || '').trim();
    if (!value) {
      return '';
    }
    const lower = value.toLowerCase();
    if (lower === 'authcallbackid' || lower === 'windows-plugin-login' || lower === 'macos-plugin-login') {
      return '';
    }
    if (lower.startsWith('cb-')) {
      return '';
    }
    return value;
  }

  detectClientPlatform(): string {
    const value = `${navigator.platform || ''} ${navigator.userAgent || ''}`.toLowerCase();
    if (value.includes('win')) {
      return 'windows';
    }
    if (value.includes('mac')) {
      return 'macos';
    }
    if (value.includes('linux')) {
      return 'linux';
    }
    if (value.includes('android')) {
      return 'android';
    }
    if (value.includes('iphone') || value.includes('ipad') || value.includes('ios')) {
      return 'ios';
    }
    return 'unknown';
  }

  persistAuth(auth: AuthResponse): void {
    localStorage.setItem('slan.accessToken', auth.accessToken);
    localStorage.setItem('slan.userId', auth.userId);
    if (auth.email?.trim()) {
      localStorage.setItem('slan.userEmail', auth.email.trim());
    }
  }

  clearCachedAuth(): void {
    localStorage.removeItem('slan.accessToken');
    localStorage.removeItem('slan.userId');
    localStorage.removeItem('slan.userEmail');
    localStorage.removeItem('slan.deviceId');
  }

  persistCurrentDeviceId(deviceId: string): void {
    if (deviceId.trim()) {
      localStorage.setItem('slan.deviceId', deviceId);
      return;
    }
    localStorage.removeItem('slan.deviceId');
  }

  ensureWebDeviceId(): string {
    const existing = localStorage.getItem('slan.webDeviceId');
    if (existing) {
      return existing;
    }
    const created = `web-${crypto.randomUUID()}`;
    localStorage.setItem('slan.webDeviceId', created);
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
    input.preferredMachineId = this.usableClientDeviceId(input.preferredMachineId);
    if (input.preferredMachineId) {
      const resolved = await this.api.registerDevice(input.token, {
        deviceId: input.preferredMachineId,
        name: input.clientName,
        platform: input.clientPlatform,
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

    const webDeviceId = this.ensureWebDeviceId();
    const created = await this.api.registerDevice(input.token, {
      deviceId: webDeviceId,
      name: 'Web Console Device',
      platform: 'web',
      publicKey: `web-console-${webDeviceId}`,
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
