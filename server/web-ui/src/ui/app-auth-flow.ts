import { ApiAuthResponse } from './app.models';

const BROWSER_AUTH_KEY = 'slan.clientWeb.auth';
const CLIENT_LOGIN_SOURCE = 'client';

export type ClientLoginTarget = {
  authMode: string;
  deviceId: string;
  source: string;
};

export type ConsoleLoginTarget = {
  loginKey: string;
};

export function readStoredBrowserAuth(): ApiAuthResponse | null {
  const payload = window.sessionStorage.getItem(BROWSER_AUTH_KEY);
  if (!payload) {
    return null;
  }
  const auth = JSON.parse(payload) as ApiAuthResponse;
  if (!isValidAuth(auth)) {
    clearBrowserAuth();
    return null;
  }
  return auth;
}

export function persistBrowserAuth(auth: ApiAuthResponse): void {
  window.sessionStorage.setItem(BROWSER_AUTH_KEY, JSON.stringify(auth));
}

export function clearBrowserAuth(): void {
  window.sessionStorage.removeItem(BROWSER_AUTH_KEY);
}

export function clientLoginTarget(params = currentParams()): ClientLoginTarget | null {
  const authMode = params.get('auth')?.trim() ?? '';
  const deviceId = params.get('deviceId')?.trim() ?? '';
  const source = params.get('source')?.trim() ?? '';
  if (authMode !== 'login' || !deviceId) {
    return null;
  }
  return { authMode, deviceId, source: source || CLIENT_LOGIN_SOURCE };
}

export function consoleLoginTarget(params = currentParams()): ConsoleLoginTarget | null {
  const loginKey = params.get('consoleLoginKey')?.trim() ?? '';
  return loginKey ? { loginKey } : null;
}

export function sanitizedHomeParams(params = currentParams()): URLSearchParams {
  const next = new URLSearchParams(params);
  next.delete('auth');
  next.delete('deviceId');
  next.delete('source');
  next.delete('consoleLoginKey');
  return next;
}

export function replaceUrl(path: string, params = new URLSearchParams()): void {
  const query = params.toString();
  window.history.replaceState({}, '', `${path}${query ? `?${query}` : ''}`);
}

function currentParams(): URLSearchParams {
  return new URLSearchParams(window.location.search);
}

function isValidAuth(auth: ApiAuthResponse | null | undefined): auth is ApiAuthResponse {
  return Boolean(auth?.session?.token && auth?.user?.userId && auth?.user?.email);
}
