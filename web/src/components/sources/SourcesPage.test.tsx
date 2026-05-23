import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useUIStore } from '@/stores/useUIStore';

const queryState = vi.hoisted(() => ({
  sources: [] as unknown[],
  importJob: {
    data: null as null | { status: 'pending' | 'running' | 'done' | 'failed'; total?: number; succeeded?: number; failed?: number; skipped?: number },
    isError: false,
    error: null as Error | null,
    isFetching: false,
  },
  createSource: {
    mutateAsync: vi.fn(),
    isPending: false,
  },
  refreshSource: {
    mutateAsync: vi.fn(),
    isPending: false,
  },
  deleteSource: {
    mutateAsync: vi.fn(),
    isPending: false,
  },
}));

vi.mock('@/lib/queries/sources', async () => {
  const actual = await vi.importActual<typeof import('@/lib/queries/sources')>('@/lib/queries/sources');
  return {
    ...actual,
    useSources: () => ({ data: queryState.sources, isLoading: false, isFetching: false }),
    useCreateSource: () => queryState.createSource,
    useRenameSource: () => ({ mutateAsync: vi.fn(), isPending: false }),
    useDeleteSource: () => queryState.deleteSource,
    useRefreshSource: () => queryState.refreshSource,
    useSourceImportJob: () => queryState.importJob,
  };
});

vi.mock('@/lib/api-client', () => ({
  apiFetch: vi.fn(),
  ApiError: class ApiError extends Error {
    status: number;
    constructor(message: string, status: number) {
      super(message);
      this.status = status;
    }
  },
}));

import { SourcesPage } from '@/app/(app)/sources/page';
import { ApiError, apiFetch } from '@/lib/api-client';

beforeEach(() => {
  queryState.sources = [];
  queryState.createSource.mutateAsync.mockReset();
  queryState.createSource.mutateAsync.mockResolvedValue({
    id: 99,
    title: 'Example',
    url: 'https://example.com/feed.xml',
  });
  queryState.createSource.isPending = false;
  queryState.refreshSource.mutateAsync.mockReset();
  queryState.refreshSource.mutateAsync.mockResolvedValue(undefined);
  queryState.refreshSource.isPending = false;
  queryState.deleteSource.mutateAsync.mockReset();
  queryState.deleteSource.mutateAsync.mockResolvedValue(undefined);
  queryState.deleteSource.isPending = false;
  queryState.importJob.data = null;
  queryState.importJob.isError = false;
  queryState.importJob.error = null;
  queryState.importJob.isFetching = false;
  vi.mocked(apiFetch).mockReset();
  useUIStore.setState({ nativeLanguage: 'zh-CN', sourceImportJob: null });
});

function wrapper({ children }: { children: React.ReactNode }) {
  const qc = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });

  return <QueryClientProvider client={qc}>{children}</QueryClientProvider>;
}

test('SourcesPage renders one unified management surface', () => {
  render(<SourcesPage />, { wrapper });

  expect(screen.getByRole('heading', { name: '订阅源' })).toBeInTheDocument();
  expect(screen.getByText('通过 URL 订阅，管理阅读来源，导入或导出 OPML。')).toBeInTheDocument();
  expect(screen.getByPlaceholderText('粘贴网站、RSS 或 Atom 地址…')).toBeInTheDocument();
  expect(screen.getByRole('button', { name: /OPML/ })).toBeInTheDocument();
  expect(screen.getByRole('button', { name: /导出/ })).toBeInTheDocument();
  expect(screen.queryByRole('heading', { name: '添加订阅源' })).not.toBeInTheDocument();
  expect(screen.queryByRole('heading', { name: '订阅源列表' })).not.toBeInTheDocument();
  expect(screen.queryByRole('heading', { name: /OPML 导入/ })).not.toBeInTheDocument();
});

test('SourcesPage empty state offers starter packs', () => {
  render(<SourcesPage />, { wrapper });

  expect(screen.getByText('还没有订阅')).toBeInTheDocument();
  expect(screen.getByText('在上方粘贴网站地址，我们会自动寻找订阅源。也可以先试试这些来源。')).toBeInTheDocument();
  expect(screen.getByRole('button', { name: /simonwillison.net/ })).toBeInTheDocument();
  expect(screen.getByRole('button', { name: /Overreacted/ })).toBeInTheDocument();
  expect(screen.getByRole('button', { name: /Joel on Software/ })).toBeInTheDocument();
});

test('SourcesPage debounces URL discovery and subscribes from the preview', async () => {
  const user = userEvent.setup();

  render(<SourcesPage />, { wrapper });

  await user.type(screen.getByPlaceholderText('粘贴网站、RSS 或 Atom 地址…'), 'example.com');
  expect(screen.getByLabelText('正在发现订阅源')).toBeInTheDocument();

  expect(await screen.findByText('Example')).toBeInTheDocument();
  expect(screen.getByText('https://example.com/feed.xml')).toBeInTheDocument();

  await user.click(screen.getByRole('button', { name: /订阅/ }));

  expect(queryState.createSource.mutateAsync).toHaveBeenCalledWith('https://example.com/feed.xml');
  expect(await screen.findByText('已订阅 Example')).toBeInTheDocument();
});

test('SourcesPage stores the active OPML import job globally', async () => {
  const user = userEvent.setup();
  vi.mocked(apiFetch).mockResolvedValueOnce({ job_id: 'import-123' });

  render(<SourcesPage />, { wrapper });

  const fileInput = document.querySelector('input[type="file"]') as HTMLInputElement;
  const file = new File(['<opml version="2.0"><body /></opml>'], 'feeds.opml', { type: 'text/x-opml' });
  await user.upload(fileInput, file);

  await waitFor(() => {
    expect(useUIStore.getState().sourceImportJob).toMatchObject({
      id: 'import-123',
      fileName: 'feeds.opml',
    });
  });
  expect(apiFetch).toHaveBeenCalledWith('/api/sources/import', expect.objectContaining({
    method: 'POST',
    headers: { 'Content-Type': 'text/x-opml; charset=utf-8' },
  }));
});

test('SourcesPage clears a stale persisted OPML import job', async () => {
  queryState.importJob.isError = true;
  queryState.importJob.error = new ApiError('job not found', 404);
  useUIStore.setState({
    sourceImportJob: {
      id: 'import-123',
      fileName: 'feeds.opml',
      startedAt: Date.now(),
    },
  });

  render(<SourcesPage />, { wrapper });

  await waitFor(() => {
    expect(useUIStore.getState().sourceImportJob).toBeNull();
  });
  expect(screen.queryByText(/导入失败/)).not.toBeInTheDocument();
});

test('SourcesPage filters, searches, and sorts populated sources', async () => {
  queryState.sources = [
    {
      id: 1,
      title: 'Healthy Feed',
      url: 'https://healthy.example/feed.xml',
      category: 'General',
      icon_url: null,
      unread_count: 5,
      last_fetched_at: new Date().toISOString(),
      last_success_at: new Date().toISOString(),
      consecutive_fails: 0,
      health: 'ok',
    },
    {
      id: 2,
      title: 'Broken Feed',
      url: 'https://broken.example/feed.xml',
      category: 'General',
      icon_url: null,
      unread_count: 0,
      last_fetched_at: null,
      last_success_at: null,
      consecutive_fails: 8,
      health: 'error',
    },
  ];
  const user = userEvent.setup();

  render(<SourcesPage />, { wrapper });

  expect(screen.getByRole('button', { name: /全部 2/ })).toBeInTheDocument();
  expect(screen.getByRole('button', { name: /健康 1/ })).toBeInTheDocument();
  expect(screen.getByRole('button', { name: /错误 1/ })).toBeInTheDocument();

  await user.click(screen.getByRole('button', { name: /错误 1/ }));

  expect(screen.getByText('Broken Feed')).toBeInTheDocument();
  expect(screen.queryByText('Healthy Feed')).not.toBeInTheDocument();
  expect(screen.getByText(/Last fetch failed:/)).toBeInTheDocument();

  await user.type(screen.getByPlaceholderText('过滤订阅源'), 'healthy');

  expect(screen.getByText('没有订阅源匹配当前筛选。')).toBeInTheDocument();
});

test('SourcesPage requires inline confirmation before deleting a source', async () => {
  queryState.sources = [
    {
      id: 1,
      title: "Let's Encrypt",
      url: 'https://letsencrypt.org/feed.xml',
      category: 'General',
      icon_url: null,
      unread_count: 0,
      last_fetched_at: null,
      last_success_at: null,
      consecutive_fails: 0,
      health: 'unknown',
    },
  ];
  const user = userEvent.setup();

  render(<SourcesPage />, { wrapper });

  const row = screen.getByText("Let's Encrypt").closest('[data-source-row]');
  expect(row).not.toBeNull();

  await user.click(within(row as HTMLElement).getByRole('button', { name: '取消订阅' }));
  expect(queryState.deleteSource.mutateAsync).not.toHaveBeenCalled();

  await user.click(within(row as HTMLElement).getByRole('button', { name: '确认删除' }));

  expect(queryState.deleteSource.mutateAsync).toHaveBeenCalledWith(1);
  expect(await screen.findByText("已取消订阅 Let's Encrypt")).toBeInTheDocument();
});
