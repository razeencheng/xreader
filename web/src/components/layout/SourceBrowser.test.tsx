import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';

vi.mock('@/lib/api-client', () => ({
  apiFetch: vi.fn(),
}));

import { apiFetch } from '@/lib/api-client';
import { SourceBrowser } from './SourceBrowser';
import { useAuthStore } from '@/stores/useAuthStore';
import { useUIStore } from '@/stores/useUIStore';

function createWrapper() {
  const client = new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
      },
    },
  });

  return function Wrapper({ children }: { children: React.ReactNode }) {
    return <QueryClientProvider client={client}>{children}</QueryClientProvider>;
  };
}

beforeEach(() => {
  vi.mocked(apiFetch).mockReset();
  useAuthStore.setState({
    user: {
      id: 1,
      github_username: 'jin',
      role: 'user',
      native_language: 'zh-CN',
      density_pref: 'comfortable',
      theme_pref: 'system',
    },
    isLoading: false,
  });
  useUIStore.setState({ currentView: 'sources', selectedSourceId: null, readFilter: 'unread', nativeLanguage: 'zh-CN' });
});

afterEach(() => {
  useAuthStore.setState({ user: null, isLoading: false });
  useUIStore.setState({ currentView: 'today', selectedSourceId: null, readFilter: 'unread', nativeLanguage: 'zh-CN' });
});

test('renders grouped sources with all-sources summary and unread badges', async () => {
  vi.mocked(apiFetch).mockResolvedValue([
    {
      id: 11,
      title: 'Hacker News',
      category: 'Technology',
      unread_count: 2,
      icon_url: null,
    },
    {
      id: 12,
      title: 'Bloomberg',
      category: 'Finance',
      unread_count: 1,
      icon_url: null,
    },
  ]);

  render(<SourceBrowser />, { wrapper: createWrapper() });

  expect(await screen.findByText('订阅源')).toBeInTheDocument();
  expect(screen.getByText('3 未读 · 2 个订阅源')).toBeInTheDocument();
  expect(screen.getByRole('button', { name: /所有订阅源/i })).toBeInTheDocument();
  expect(screen.getByText('Technology')).toBeInTheDocument();
  expect(screen.getByText('Finance')).toBeInTheDocument();

  const hackerNewsRow = screen.getByRole('button', { name: /Hacker News/i });
  expect(within(hackerNewsRow).getByText('2')).toBeInTheDocument();
  expect(screen.queryByRole('link', { name: '管理' })).not.toBeInTheDocument();
});

test('uses a single-column shell below desktop widths', async () => {
  vi.mocked(apiFetch).mockResolvedValue([]);

  const { container } = render(<SourceBrowser />, { wrapper: createWrapper() });
  await screen.findByText('订阅源');

  const shell = container.firstElementChild;
  expect(shell).toHaveClass('lg:border-r');
  expect(shell).toHaveClass('lg:w-[300px]');
  expect(shell).not.toHaveClass('border-r');
  expect(shell).not.toHaveClass('md:w-[300px]');
});

test('selects a source when clicking a grouped row', async () => {
  vi.mocked(apiFetch).mockResolvedValue([
    {
      id: 21,
      title: 'Cloudflare',
      category: 'Infrastructure',
      unread_count: 4,
      icon_url: null,
    },
  ]);

  const user = userEvent.setup();
  render(<SourceBrowser />, { wrapper: createWrapper() });

  const row = await screen.findByRole('button', { name: /Cloudflare/i });
  await user.click(row);

  expect(useUIStore.getState().currentView).toBe('sources');
  expect(useUIStore.getState().selectedSourceId).toBe(21);
});
