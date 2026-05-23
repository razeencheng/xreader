import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { SourceImportStatus } from './SourceImportStatus';
import { useUIStore } from '@/stores/useUIStore';
import { ApiError } from '@/lib/api-client';

const routeState = vi.hoisted(() => ({
  pathname: '/',
}));

const importQueryState = vi.hoisted(() => ({
  data: null as null | {
    status: 'pending' | 'running' | 'done' | 'failed';
    total?: number;
    succeeded?: number;
    failed?: number;
    skipped?: number;
    progress?: number;
  },
  isError: false,
  error: null as Error | null,
}));

vi.mock('next/navigation', () => ({
  usePathname: () => routeState.pathname,
}));

vi.mock('@/lib/queries/sources', async () => {
  const actual = await vi.importActual<typeof import('@/lib/queries/sources')>('@/lib/queries/sources');
  return {
    ...actual,
    useSourceImportJob: () => ({
      data: importQueryState.data,
      isError: importQueryState.isError,
      error: importQueryState.error,
      isFetching: false,
    }),
  };
});

beforeEach(() => {
  routeState.pathname = '/';
  importQueryState.data = null;
  importQueryState.isError = false;
  importQueryState.error = null;
  useUIStore.setState({
    nativeLanguage: 'zh-CN',
    sourceImportJob: {
      id: 'import-123',
      fileName: 'feeds.opml',
      startedAt: 1,
    },
  });
});

function wrapper({ children }: { children: React.ReactNode }) {
  const qc = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });

  return <QueryClientProvider client={qc}>{children}</QueryClientProvider>;
}

test('renders active OPML import progress outside the sources page', () => {
  importQueryState.data = {
    status: 'running',
    total: 10,
    succeeded: 3,
    failed: 1,
    skipped: 1,
  };

  render(<SourceImportStatus />, { wrapper });

  expect(screen.getByRole('status')).toHaveTextContent('正在导入 feeds.opml');
  expect(screen.getByRole('status')).toHaveTextContent('5 / 10 个源 · 轮询中…');
});

test('does not duplicate the import panel on the sources page', () => {
  routeState.pathname = '/sources';
  importQueryState.data = {
    status: 'running',
    total: 10,
    succeeded: 2,
  };

  render(<SourceImportStatus />, { wrapper });

  expect(screen.queryByRole('status')).not.toBeInTheDocument();
});

test('lets the user dismiss a completed import job', async () => {
  const user = userEvent.setup();
  importQueryState.data = {
    status: 'done',
    total: 4,
    succeeded: 4,
  };

  render(<SourceImportStatus />, { wrapper });

  await user.click(screen.getByRole('button', { name: '关闭' }));

  expect(useUIStore.getState().sourceImportJob).toBeNull();
});

test('clears a persisted import job when the server no longer knows it', async () => {
  importQueryState.isError = true;
  importQueryState.error = new ApiError(404, 'UNKNOWN', 'job not found');

  render(<SourceImportStatus />, { wrapper });

  expect(screen.queryByRole('status')).not.toBeInTheDocument();
  await waitFor(() => {
    expect(useUIStore.getState().sourceImportJob).toBeNull();
  });
});
