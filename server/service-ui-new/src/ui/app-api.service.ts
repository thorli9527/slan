import { Injectable } from '@angular/core';

export const API_BASE = 'http://127.0.0.1:38080';

@Injectable({ providedIn: 'root' })
export class AppApiClient {
  async get<T>(path: string): Promise<T> {
    const response = await fetch(`${API_BASE}${path}`);
    if (!response.ok) {
      throw new Error(`GET ${path} ${response.status}`);
    }
    return response.json() as Promise<T>;
  }

  async post<T>(path: string, body: unknown): Promise<T> {
    const response = await fetch(`${API_BASE}${path}`, {
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
    const response = await fetch(`${API_BASE}${path}`, {
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
    const response = await fetch(`${API_BASE}${path}`, { method: 'DELETE' });
    if (!response.ok) {
      throw new Error(`DELETE ${path} ${response.status}`);
    }
    return response.json() as Promise<T>;
  }
}
