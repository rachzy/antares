import { API_ROUTES } from './api.routes';

describe('API_ROUTES', () => {
  it('encodes the subscription token', () => {
    expect(API_ROUTES.subscription('a b')).toBe('/subscription/a%20b');
  });

  it('keeps every static route rooted at /', () => {
    for (const [name, path] of Object.entries(API_ROUTES)) {
      if (typeof path === 'string') {
        expect(path.startsWith('/'), name).toBe(true);
      }
    }
  });
});
