import { Injectable } from '@angular/core';

import { ConsoleApiService } from './console-api.service';

export type CallbackTrackingHandle = {
  dispose(): void;
};

type StartTrackingInput = {
  callbackId: string;
  fallbackDelayMs?: number;
  pollIntervalMs?: number;
  onAcknowledged(): void;
  onFallback(): void;
};

@Injectable({ providedIn: 'root' })
export class ConsoleCallbackService {
  constructor(private readonly api: ConsoleApiService) {}

  copyTarget(target: string): Promise<void> {
    return navigator.clipboard.writeText(target);
  }

  startTracking(input: StartTrackingInput): CallbackTrackingHandle {
    const fallbackTimer = setTimeout(() => {
      input.onFallback();
    }, input.fallbackDelayMs ?? 1500);

    const statusTimer = setInterval(async () => {
      try {
        const status = await this.api.getCallbackStatus(input.callbackId);
        if (!status.received) {
          return;
        }
        clearTimeout(fallbackTimer);
        clearInterval(statusTimer);
        input.onAcknowledged();
      } catch (_) {
        // Keep polling quietly; the login result remains visible in the UI.
      }
    }, input.pollIntervalMs ?? 1000);

    return {
      dispose(): void {
        clearTimeout(fallbackTimer);
        clearInterval(statusTimer);
      }
    };
  }
}
