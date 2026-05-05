import { Injectable } from '@angular/core';

@Injectable({ providedIn: 'root' })
export class OpsApiService {
  async request<T>(path: string, token: string, init: RequestInit = {}, auth = true): Promise<T> {
    const headers = new Headers(init.headers || {});
    headers.set('Content-Type', 'application/json');
    if (auth && token) {
      headers.set('Authorization', `Bearer ${token}`);
    }
    const response = await fetch(`/ops-api${path}`, { ...init, headers });
    const text = await response.text();
    const payload = text ? JSON.parse(text) : null;
    if (!response.ok) {
      throw new Error(payload?.message || `request failed: ${response.status}`);
    }
    return payload as T;
  }
}
