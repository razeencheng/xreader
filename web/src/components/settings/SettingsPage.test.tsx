import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import type { ReactNode } from 'react';

const invalidateQueries = vi.fn();
const fetchMe = vi.fn();
const toggleDensity = vi.fn();
const setTheme = vi.fn();
const apiFetch = vi.fn();

let authState = {
  user: {
    id: 1,
    github_username: 'razeencheng',
    role: 'admin',
    native_language: 'zh-CN',
    density_pref: 'comfortable',
    theme_pref: 'system',
  },
  fetchMe,
};

let uiState = {
  density: 'comfortable' as const,
  theme: 'system' as const,
  nativeLanguage: 'zh-CN',
  toggleDensity,
  setTheme,
  hydrate: vi.fn(),
};

vi.mock('next/link', () => ({
  default: ({ children, href, ...props }: { children?: ReactNode; href: string; [key: string]: unknown }) => (
    <a href={href} {...props}>
      {children}
    </a>
  ),
}));

vi.mock('@/lib/api-client', () => ({
  apiFetch: (...args: unknown[]) => apiFetch(...args),
}));

vi.mock('@/stores/useAuthStore', () => ({
  useAuthStore: Object.assign(
    (selector: (state: typeof authState) => unknown) => selector(authState),
    {
      getState: () => authState,
    },
  ),
}));

vi.mock('@/stores/useUIStore', () => ({
  useUIStore: (selector: (state: typeof uiState) => unknown) => selector(uiState),
}));

vi.mock('@tanstack/react-query', async () => {
  const actual = await vi.importActual<typeof import('@tanstack/react-query')>('@tanstack/react-query');

  return {
    ...actual,
    useQueryClient: () => ({
      invalidateQueries,
    }),
  };
});

import SettingsPage from '@/app/(app)/settings/page';

afterEach(() => {
  vi.clearAllMocks();
  authState = {
    ...authState,
    fetchMe,
  };
  uiState = {
    ...uiState,
    toggleDensity,
    setTheme,
  };
});

beforeEach(() => {
  apiFetch.mockImplementation(async (input: unknown, init?: RequestInit) => {
    if (input === '/api/users/me' && (!init || !init.method || init.method === 'GET')) {
      return {
        native_language: 'zh-CN',
        density_pref: 'comfortable',
        theme_pref: 'system',
      };
    }

    if (input === '/api/ai/settings' && (!init || !init.method || init.method === 'GET')) {
      return {
        endpoint: 'https://api.example.com/v1',
        model: 'qwen-turbo',
        api_key_set: true,
        api_key_hint: 'sk-...test',
      };
    }

    return {};
  });
});

function renderPage() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });

  return render(
    <QueryClientProvider client={queryClient}>
      <SettingsPage />
    </QueryClientProvider>,
  );
}

test('renders settings shell without duplicated reader preference controls', async () => {
  renderPage();

  expect(await screen.findByRole('heading', { name: '设置' })).toBeInTheDocument();
  expect(screen.queryByLabelText('母语')).not.toBeInTheDocument();
  expect(screen.queryByRole('heading', { name: '显示密度' })).not.toBeInTheDocument();
  expect(screen.queryByRole('heading', { name: '主题' })).not.toBeInTheDocument();
  expect(screen.queryByRole('button', { name: '保存设置' })).not.toBeInTheDocument();
});

test('renders and saves model integration settings', async () => {
  const user = userEvent.setup();
  renderPage();

  expect(await screen.findByRole('heading', { name: '模型接入设置' })).toBeInTheDocument();
  expect(await screen.findByDisplayValue('https://api.example.com/v1')).toBeInTheDocument();
  expect(screen.getByLabelText('模型')).toHaveValue('qwen-turbo');
  expect(screen.getByText('当前 API Key：sk-...test')).toBeInTheDocument();

  await user.clear(screen.getByLabelText('OpenAI 接入点'));
  await user.type(screen.getByLabelText('OpenAI 接入点'), 'https://relay.example.com/v1');
  await user.clear(screen.getByLabelText('模型'));
  await user.type(screen.getByLabelText('模型'), 'deepseek-chat');
  await user.type(screen.getByLabelText('API Key'), 'sk-new');
  await user.click(screen.getByRole('button', { name: '保存模型接入设置' }));

  await waitFor(() => {
    expect(apiFetch).toHaveBeenCalledWith('/api/ai/settings', {
      method: 'PATCH',
      body: JSON.stringify({
        endpoint: 'https://relay.example.com/v1',
        model: 'deepseek-chat',
        api_key: 'sk-new',
      }),
    });
  });
});

test('renders model integration settings as read-only for non-admin users', async () => {
  authState = {
    ...authState,
    user: {
      ...authState.user,
      role: 'user',
    },
  };
  renderPage();

  expect(await screen.findByDisplayValue('https://api.example.com/v1')).toHaveAttribute('readOnly');
  expect(screen.getByLabelText('模型')).toHaveAttribute('readOnly');
  expect(screen.getByLabelText('API Key')).toHaveAttribute('readOnly');
  expect(screen.getByText('仅管理员可以修改模型接入设置。')).toBeInTheDocument();
  expect(screen.queryByRole('button', { name: '保存模型接入设置' })).not.toBeInTheDocument();
});
