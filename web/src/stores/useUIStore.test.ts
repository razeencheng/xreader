import { useUIStore } from './useUIStore';

const storage = new Map<string, string>();

beforeAll(() => {
  vi.stubGlobal('localStorage', {
    getItem: (key: string) => storage.get(key) ?? null,
    setItem: (key: string, value: string) => storage.set(key, value),
    removeItem: (key: string) => storage.delete(key),
    clear: () => storage.clear(),
  });
});

beforeEach(() => {
  storage.clear();
  useUIStore.setState({ density: 'comfortable', theme: 'system', nativeLanguage: 'zh-CN', sourceImportJob: null, _hydrated: false });
  globalThis.fetch = vi.fn(async () => new Response('{}', { status: 200 })) as typeof fetch;
});

afterEach(() => {
  vi.restoreAllMocks();
});

test('toggleDensity flips value and persists to localStorage', () => {
  useUIStore.getState().toggleDensity();
  expect(useUIStore.getState().density).toBe('compact');
  expect(localStorage.getItem('xreader:density')).toBe('compact');

  useUIStore.getState().toggleDensity();
  expect(useUIStore.getState().density).toBe('comfortable');
  expect(localStorage.getItem('xreader:density')).toBe('comfortable');
});

test('setTheme persists to localStorage', () => {
  useUIStore.getState().setTheme('dark');
  expect(useUIStore.getState().theme).toBe('dark');
  expect(localStorage.getItem('xreader:theme')).toBe('dark');
});

test('hydrate loads from user prefs', () => {
  useUIStore.getState().hydrate({ density_pref: 'compact', theme_pref: 'dark', native_language: 'en' });
  expect(useUIStore.getState().density).toBe('compact');
  expect(useUIStore.getState().theme).toBe('dark');
  expect(useUIStore.getState().nativeLanguage).toBe('en');
});

test('source import job persists across page remounts', () => {
  useUIStore.getState().startSourceImport('import-123', 'feeds.opml');

  expect(useUIStore.getState().sourceImportJob).toMatchObject({
    id: 'import-123',
    fileName: 'feeds.opml',
  });
  expect(localStorage.getItem('xreader:sourceImportJobId')).toBe('import-123');
  expect(localStorage.getItem('xreader:sourceImportFileName')).toBe('feeds.opml');

  useUIStore.setState({ sourceImportJob: null, _hydrated: false });
  useUIStore.getState().hydrateFromLocalStorage();

  expect(useUIStore.getState().sourceImportJob).toMatchObject({
    id: 'import-123',
    fileName: 'feeds.opml',
  });
});

test('clearSourceImport clears active and persisted import job', () => {
  useUIStore.getState().startSourceImport('import-123', 'feeds.opml');

  useUIStore.getState().clearSourceImport();

  expect(useUIStore.getState().sourceImportJob).toBeNull();
  expect(localStorage.getItem('xreader:sourceImportJobId')).toBeNull();
  expect(localStorage.getItem('xreader:sourceImportFileName')).toBeNull();
});
