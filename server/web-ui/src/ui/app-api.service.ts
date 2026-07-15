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
  private accessToken: () => string = () => '';
  private refreshSession: (() => Promise<void>) | null = null;
  private refreshPromise: Promise<void> | null = null;

  configureAuth(
    accessToken: () => string,
    refreshSession: () => Promise<void>,
  ): void {
    this.accessToken = accessToken;
    this.refreshSession = refreshSession;
  }

  async get<T>(path: string): Promise<T> {
    return this.request<T>('GET', path);
  }

  async post<T>(path: string, body: unknown): Promise<T> {
    return this.request<T>('POST', path, body);
  }

  async postAuthorized<T>(path: string, bearerToken: string, body: unknown): Promise<T> {
    return this.request<T>('POST', path, body, bearerToken, false);
  }

  async patch<T>(path: string, body: unknown): Promise<T> {
    return this.request<T>('PATCH', path, body);
  }

  async delete<T>(path: string): Promise<T> {
    return this.request<T>('DELETE', path);
  }

  async put<T>(path: string, body: unknown): Promise<T> {
    return this.request<T>('PUT', path, body);
  }

  private async request<T>(
    method: string,
    path: string,
    body?: unknown,
    explicitToken = '',
    allowRefresh = true,
  ): Promise<T> {
    const response = await this.fetch(method, path, body, explicitToken || this.accessToken());
    if (response.status === 401 && allowRefresh && this.accessToken() && this.refreshSession) {
      await this.refreshOnce();
      const retried = await this.fetch(method, path, body, this.accessToken());
      return this.readResponse<T>(method, path, retried);
    }
    return this.readResponse<T>(method, path, response);
  }

  private fetch(method: string, path: string, body: unknown, token: string): Promise<Response> {
    const headers: Record<string, string> = {};
    if (token) {
      headers['Authorization'] = `Bearer ${token}`;
    }
    if (body !== undefined) {
      headers['Content-Type'] = 'application/json';
    }
    return fetch(`${apiBase()}${path}`, {
      method,
      headers,
      body: body === undefined ? undefined : JSON.stringify(body),
    });
  }

  private async readResponse<T>(method: string, path: string, response: Response): Promise<T> {
    if (!response.ok) {
      throw await this.httpError(method, path, response);
    }
    if (response.status === 204) {
      return undefined as T;
    }
    return response.json() as Promise<T>;
  }

  private refreshOnce(): Promise<void> {
    if (!this.refreshPromise) {
      this.refreshPromise = this.refreshSession!().finally(() => {
        this.refreshPromise = null;
      });
    }
    return this.refreshPromise;
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
