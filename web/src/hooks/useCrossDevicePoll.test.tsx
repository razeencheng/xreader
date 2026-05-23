import { renderHook, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { ReactNode } from 'react';

vi.mock('@/lib/api-client', () => ({ apiFetch: vi.fn() }));
const applyArticleStateChange = vi.fn();
vi.mock('@/lib/article-state-cache', () => ({
  applyArticleStateChange: (...a: unknown[]) => applyArticleStateChange(...a),
}));

import { apiFetch } from '@/lib/api-client';
import { useCrossDevicePoll } from './useCrossDevicePoll';

function wrapper({ children }: { children: ReactNode }) {
  return <QueryClientProvider client={new QueryClient()}>{children}</QueryClientProvider>;
}

describe('useCrossDevicePoll', () => {
  beforeEach(() => {
    applyArticleStateChange.mockClear();
    vi.mocked(apiFetch).mockResolvedValue({
      items: [{ article_id: 42, changed_at: '2026-05-16T00:00:00Z', is_read: true, is_starred: false }],
    });
    Object.defineProperty(document, 'visibilityState', { value: 'visible', configurable: true });
  });
  afterEach(() => vi.clearAllMocks());

  it('maps snake_case backend fields to an ArticleStateChange', async () => {
    renderHook(() => useCrossDevicePoll(true), { wrapper });
    await waitFor(() => expect(applyArticleStateChange).toHaveBeenCalled());
    expect(applyArticleStateChange.mock.calls[0][1]).toMatchObject({
      articleId: 42, is_read: true, is_starred: false,
    });
  });

  it('applies every change from the poll without local-echo gating', async () => {
    vi.mocked(apiFetch).mockResolvedValue({
      items: [
        { article_id: 1, changed_at: '2026-05-16T00:00:00Z', is_read: true, is_starred: false },
        { article_id: 2, changed_at: '2026-05-16T00:00:01Z', is_read: false, is_starred: true },
      ],
    });
    renderHook(() => useCrossDevicePoll(true), { wrapper });
    await waitFor(() => expect(applyArticleStateChange).toHaveBeenCalledTimes(2));
    expect(applyArticleStateChange.mock.calls.map((c) => c[1])).toEqual([
      { articleId: 1, is_read: true, is_starred: false },
      { articleId: 2, is_read: false, is_starred: true },
    ]);
  });
});
