import { InjectionToken } from '@angular/core';

declare global {
  interface Window {
    ANTARES_CONFIG?: { apiBaseUrl?: string };
  }
}

const DEFAULT_API_BASE_URL = 'http://localhost:8080';

/** Resolves the runtime `apiBaseUrl` (absolute, or a path such as `/api`) against the page origin. */
export function resolveApiBaseUrl(
  configured: string | undefined,
  origin: string,
): string {
  if (!configured) return DEFAULT_API_BASE_URL;
  return new URL(configured, origin).toString().replace(/\/$/, '');
}

export const API_BASE_URL = new InjectionToken<string>('API_BASE_URL', {
  providedIn: 'root',
  factory: () =>
    typeof window === 'undefined'
      ? DEFAULT_API_BASE_URL
      : resolveApiBaseUrl(
          window.ANTARES_CONFIG?.apiBaseUrl,
          window.location.origin,
        ),
});
