import { Injectable } from '@angular/core';

export function apiBase(): string {
  const { protocol, hostname } = window.location;
  if (hostname === 'web.dev.staticlss.com') {
    return `${protocol}//api.dev.staticlss.com`;
  }
  if (hostname.startsWith('web.')) {
    return `${protocol}//api.${hostname.slice(4)}`;
  }
  return '';
}

@Injectable({ providedIn: 'root' })
export class AppApiClient {
  async get<T>(path: string): Promise<T> {
    const response = await fetch(`${apiBase()}${path}`);
    if (!response.ok) {
      throw new Error(`GET ${path} ${response.status}`);
    }
    return response.json() as Promise<T>;
  }

  async post<T>(path: string, body: unknown): Promise<T> {
    const response = await fetch(`${apiBase()}${path}`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    });
    if (!response.ok) {
      throw new Error(`POST ${path} ${response.status}`);
    }
    return response.json() as Promise<T>;
  }

  async patch<T>(path: string, body: unknown): Promise<T> {
    const response = await fetch(`${apiBase()}${path}`, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    });
    if (!response.ok) {
      throw new Error(`PATCH ${path} ${response.status}`);
    }
    return response.json() as Promise<T>;
  }

  async delete<T>(path: string): Promise<T> {
    const response = await fetch(`${apiBase()}${path}`, { method: 'DELETE' });
    if (!response.ok) {
      throw new Error(`DELETE ${path} ${response.status}`);
    }
    return response.json() as Promise<T>;
  }
}
