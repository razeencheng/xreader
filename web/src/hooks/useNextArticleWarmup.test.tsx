import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render } from '@testing-library/react';
import type { ReactNode } from 'react';
import { act } from 'react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

const warm = vi.hoisted(() => ({
  startBodyWarmup: vi.fn(),
  scheduleCancelBodyWarmup: vi.fn(),
}));
vi.mock('@/lib/body-translation-warmup', () => warm);

const apiFetch = vi.hoisted(() => vi.fn().mockResolvedValue({}));
vi.mock('@/lib/api-client', () => ({ apiFetch }));

import { useNextArticleWarmup } from './useNextArticleWarmup';

type Params = Parameters<typeof useNextArticleWarmup>[0];

function Harness(props: Params) {
  useNextArticleWarmup(props);
  return null;
}

function renderHook(props: Params) {
  const client = new QueryClient();
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={client}>{children}</QueryClientProvider>
  );
  return render(<Harness {...props} />, { wrapper });
}

const NEXT = { id: 42, language: 'en' };

beforeEach(() => {
  vi.useFakeTimers();
});
afterEach(() => {
  vi.runOnlyPendingTimers();
  vi.useRealTimers();
  warm.startBodyWarmup.mockReset();
  warm.scheduleCancelBodyWarmup.mockReset();
  apiFetch.mockReset().mockResolvedValue({});
});

describe('useNextArticleWarmup', () => {
  it('does not warm before the dwell gate (loaded, low progress, <1s)', () => {
    renderHook({ currentId: '1', next: NEXT, nativeLanguage: 'zh-CN', articleLoaded: true, progress: 0 });
    act(() => { vi.advanceTimersByTime(500); });
    expect(warm.startBodyWarmup).not.toHaveBeenCalled();
  });

  it('warms after 1s dwell', () => {
    renderHook({ currentId: '1', next: NEXT, nativeLanguage: 'zh-CN', articleLoaded: true, progress: 0 });
    act(() => { vi.advanceTimersByTime(1000); });
    expect(warm.startBodyWarmup).toHaveBeenCalledWith(42, 'zh-CN', 5);
  });

  it('warms immediately when progress > 0.2 even before dwell', () => {
    renderHook({ currentId: '1', next: NEXT, nativeLanguage: 'zh-CN', articleLoaded: true, progress: 0.5 });
    act(() => { vi.advanceTimersByTime(0); });
    expect(warm.startBodyWarmup).toHaveBeenCalledWith(42, 'zh-CN', 5);
  });

  it('does not warm a same-language next article', () => {
    renderHook({ currentId: '1', next: { id: 42, language: 'zh' }, nativeLanguage: 'zh-CN', articleLoaded: true, progress: 0.9 });
    act(() => { vi.advanceTimersByTime(2000); });
    expect(warm.startBodyWarmup).not.toHaveBeenCalled();
  });

  it('does nothing when there is no next article', () => {
    renderHook({ currentId: '1', next: null, nativeLanguage: 'zh-CN', articleLoaded: true, progress: 0.9 });
    act(() => { vi.advanceTimersByTime(2000); });
    expect(warm.startBodyWarmup).not.toHaveBeenCalled();
  });

  it('does not warm while the current article is still loading', () => {
    renderHook({ currentId: '1', next: NEXT, nativeLanguage: 'zh-CN', articleLoaded: false, progress: 0.9 });
    act(() => { vi.advanceTimersByTime(2000); });
    expect(warm.startBodyWarmup).not.toHaveBeenCalled();
  });

  it('schedules a deferred teardown of the previously-warmed article when next changes', () => {
    const { rerender } = renderHook({ currentId: '1', next: NEXT, nativeLanguage: 'zh-CN', articleLoaded: true, progress: 0.9 });
    act(() => { vi.advanceTimersByTime(1000); });
    expect(warm.startBodyWarmup).toHaveBeenCalledWith(42, 'zh-CN', 5);
    rerender(
      <QueryClientProvider client={new QueryClient()}>
        <Harness currentId="1" next={{ id: 99, language: 'en' }} nativeLanguage="zh-CN" articleLoaded progress={0.9} />
      </QueryClientProvider>,
    );
    expect(warm.scheduleCancelBodyWarmup).toHaveBeenCalledWith(42, 'zh-CN');
  });

  it('schedules a deferred teardown of the warmed article on unmount', () => {
    const { unmount } = renderHook({ currentId: '1', next: NEXT, nativeLanguage: 'zh-CN', articleLoaded: true, progress: 0.9 });
    act(() => { vi.advanceTimersByTime(1000); });
    expect(warm.startBodyWarmup).toHaveBeenCalledWith(42, 'zh-CN', 5);
    unmount();
    expect(warm.scheduleCancelBodyWarmup).toHaveBeenCalledWith(42, 'zh-CN');
  });

  it('prefetches the next article detail (no gate) when next is present', async () => {
    renderHook({ currentId: '1', next: NEXT, nativeLanguage: 'zh-CN', articleLoaded: true, progress: 0 });
    await act(async () => { await Promise.resolve(); });
    expect(apiFetch).toHaveBeenCalledWith('/api/articles/42');
  });

  it('does not warm the old article when next changes before the dwell elapses (rapid H/L)', () => {
    const { rerender } = renderHook({ currentId: '1', next: NEXT, nativeLanguage: 'zh-CN', articleLoaded: true, progress: 0 });
    act(() => { vi.advanceTimersByTime(500); }); // not yet 1s
    rerender(
      <QueryClientProvider client={new QueryClient()}>
        <Harness currentId="1" next={{ id: 99, language: 'en' }} nativeLanguage="zh-CN" articleLoaded progress={0} />
      </QueryClientProvider>,
    );
    act(() => { vi.advanceTimersByTime(1000); }); // old dwell would have fired here if not cleared
    expect(warm.startBodyWarmup).not.toHaveBeenCalledWith(42, 'zh-CN', 5);
    expect(warm.startBodyWarmup).toHaveBeenCalledWith(99, 'zh-CN', 5);
  });
});
