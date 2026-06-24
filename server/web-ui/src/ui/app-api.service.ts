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

export class ApiHttpError extends Error {
  constructor(
    public readonly method: string,
    public readonly path: string,
    public readonly status: number,
    public readonly code = '',
    message = `${method} ${path} ${status}`,
  ) {
    super(message);
  }
}

@Injectable({ providedIn: 'root' })
export class AppApiClient {
  async get<T>(path: string): Promise<T> {
    const response = await fetch(`${apiBase()}${path}`);
    if (!response.ok) {
      throw await this.httpError('GET', path, response);
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
      throw await this.httpError('POST', path, response);
    }
    return response.json() as Promise<T>;
  }

  async postAuthorized<T>(path: string, bearerToken: string, body: unknown): Promise<T> {
    const response = await fetch(`${apiBase()}${path}`, {
      method: 'POST',
      headers: {
        'Authorization': `Bearer ${bearerToken}`,
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(body),
    });
    if (!response.ok) {
      throw await this.httpError('POST', path, response);
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
      throw await this.httpError('PATCH', path, response);
    }
    return response.json() as Promise<T>;
  }

  async delete<T>(path: string): Promise<T> {
    const response = await fetch(`${apiBase()}${path}`, { method: 'DELETE' });
    if (!response.ok) {
      throw await this.httpError('DELETE', path, response);
    }
    if (response.status === 204) {
      return undefined as T;
    }
    return response.json() as Promise<T>;
  }

  async put<T>(path: string, body: unknown): Promise<T> {
    const response = await fetch(`${apiBase()}${path}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    });
    if (!response.ok) {
      throw await this.httpError('PUT', path, response);
    }
    return response.json() as Promise<T>;
  }

  private async httpError(method: string, path: string, response: Response): Promise<ApiHttpError> {
    let code = '';
    try {
      const body = await response.json() as { error?: unknown };
      code = typeof body.error === 'string' ? body.error : '';
    } catch {
      code = '';
    }
    const suffix = code ? ` ${code}` : '';
    return new ApiHttpError(method, path, response.status, code, `${method} ${path} ${response.status}${suffix}`);
  }
}
