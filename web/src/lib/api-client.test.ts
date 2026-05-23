import { apiFetch } from './api-client';

afterEach(() => {
  vi.restoreAllMocks();
});

test('apiFetch throws ApiError with code on 400', async () => {
  globalThis.fetch = vi.fn(async () =>
    new Response(JSON.stringify({ code: 'VALIDATION_ERROR', message: 'bad' }), { status: 400 }),
  ) as typeof fetch;
  await expect(apiFetch('/x')).rejects.toMatchObject({ code: 'VALIDATION_ERROR' });
});

test('apiFetch redirects to /login on 401', async () => {
  const loc: Pick<Location, 'href'> = { href: '' };
  vi.stubGlobal('location', loc);
  globalThis.fetch = vi.fn(async () => new Response('', { status: 401 })) as typeof fetch;
  await apiFetch('/x').catch(() => {});
  expect(loc.href).toBe('/login');
});

test('apiFetch can suppress redirect on background 401', async () => {
  const loc: Pick<Location, 'href'> = { href: '' };
  vi.stubGlobal('location', loc);
  globalThis.fetch = vi.fn(async () => new Response('', { status: 401 })) as typeof fetch;

  await expect(apiFetch('/x', { redirectOnUnauthorized: false })).rejects.toMatchObject({ status: 401 });

  expect(loc.href).toBe('');
});

test('apiFetch sets X-Requested-With on POST', async () => {
  globalThis.fetch = vi.fn(async () =>
    new Response(JSON.stringify({}), { status: 200 }),
  ) as typeof fetch;
  await apiFetch('/x', { method: 'POST', body: '{}' });
  const call = vi.mocked(globalThis.fetch).mock.calls[0];
  const headers = call[1].headers;
  expect(headers.get('X-Requested-With')).toBe('xhr');
});

test('apiFetch sends credentials include', async () => {
  globalThis.fetch = vi.fn(async () =>
    new Response(JSON.stringify({}), { status: 200 }),
  ) as typeof fetch;
  await apiFetch('/x');
  expect(vi.mocked(globalThis.fetch).mock.calls[0][1].credentials).toBe('include');
});

test('apiFetch returns undefined for 204', async () => {
  globalThis.fetch = vi.fn(async () =>
    new Response(null, { status: 204 }),
  ) as typeof fetch;
  const result = await apiFetch('/x');
  expect(result).toBeUndefined();
});
