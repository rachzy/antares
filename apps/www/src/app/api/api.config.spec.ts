import { describe, expect, it } from 'vitest';
import { resolveApiBaseUrl } from './api.config';

describe('resolveApiBaseUrl', () => {
  it('falls back to the local default when nothing is configured', () => {
    expect(resolveApiBaseUrl(undefined, 'http://host')).toBe(
      'http://localhost:8080',
    );
  });

  it('keeps an absolute URL', () => {
    expect(resolveApiBaseUrl('http://localhost:8081', 'http://host')).toBe(
      'http://localhost:8081',
    );
  });

  it('resolves a path against the page origin', () => {
    expect(resolveApiBaseUrl('/api', 'https://antares.example')).toBe(
      'https://antares.example/api',
    );
  });
});
