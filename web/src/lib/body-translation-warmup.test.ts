import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

const sse = vi.hoisted(() => {
  type Rec = {
    url: string;
    paragraph: ((p: { index: number; translation: string }) => void) | null;
    done: (() => void) | null;
    error: ((e: Event) => void) | null;
    close: ReturnType<typeof vi.fn>;
  };
  const clients: Rec[] = [];
  const createSSEClient = vi.fn((url: string) => {
    const rec: Rec = { url, paragraph: null, done: null, error: null, close: vi.fn() };
    clients.push(rec);
    return {
      onParagraph: (cb: NonNullable<Rec['paragraph']>) => { rec.paragraph = cb; },
      onDone: (cb: NonNullable<Rec['done']>) => { rec.done = cb; },
      onError: (cb: NonNullable<Rec['error']>) => { rec.error = cb; },
      close: rec.close,
    };
  });
  return {
    createSSEClient,
    clients,
    reset() { clients.splice(0, clients.length); createSSEClient.mockClear(); },
  };
});

vi.mock('@/lib/sse-client', () => ({ createSSEClient: sse.createSSEClient }));

import {
  startBodyWarmup,
  getBodyWarmup,
  subscribeBodyWarmup,
  claimBodyWarmup,
  scheduleCancelBodyWarmup,
  releaseBodyWarmup,
  __resetBodyWarmupForTests,
} from './body-translation-warmup';

beforeEach(() => {
  vi.useFakeTimers();
});
afterEach(() => {
  vi.runOnlyPendingTimers();
  vi.useRealTimers();
  __resetBodyWarmupForTests();
  sse.reset();
});

describe('body-translation-warmup', () => {
  it('opens one SSE for start=0 and the requested count, collecting paragraphs', () => {
    startBodyWarmup(7, 'zh-CN', 5);
    expect(sse.createSSEClient).toHaveBeenCalledWith(
      '/api/articles/7/body-translation?start=0&count=5',
    );
    sse.clients[0].paragraph!({ index: 0, translation: '零' });
    sse.clients[0].paragraph!({ index: 1, translation: '一' });
    const snap = getBodyWarmup(7, 'zh-CN');
    expect(snap?.translations.get(0)).toBe('零');
    expect(snap?.translations.get(1)).toBe('一');
    expect(snap?.done).toBe(false);
  });

  it('dedupes: a second startBodyWarmup for the same key does not open another SSE', () => {
    startBodyWarmup(7, 'zh-CN', 5);
    startBodyWarmup(7, 'zh-CN', 5);
    expect(sse.createSSEClient).toHaveBeenCalledTimes(1);
  });

  it('keys by native language: different language is a distinct warm-up', () => {
    startBodyWarmup(7, 'zh-CN', 5);
    startBodyWarmup(7, 'ja-JP', 5);
    expect(sse.createSSEClient).toHaveBeenCalledTimes(2);
  });

  it('subscribers receive in-flight paragraphs until unsubscribed', () => {
    startBodyWarmup(7, 'zh-CN', 5);
    const seen: number[] = [];
    const unsub = subscribeBodyWarmup(7, 'zh-CN', (p) => seen.push(p.index));
    sse.clients[0].paragraph!({ index: 0, translation: '零' });
    sse.clients[0].paragraph!({ index: 1, translation: '一' });
    unsub();
    sse.clients[0].paragraph!({ index: 2, translation: '二' });
    expect(seen).toEqual([0, 1]);
  });

  it('onDone marks the entry done (client retained so teardown can still close it)', () => {
    startBodyWarmup(7, 'zh-CN', 5);
    sse.clients[0].done!();
    expect(getBodyWarmup(7, 'zh-CN')?.done).toBe(true);
    releaseBodyWarmup(7, 'zh-CN');
    expect(sse.clients[0].close).toHaveBeenCalledTimes(1);
    expect(getBodyWarmup(7, 'zh-CN')).toBeUndefined();
  });

  it('does NOT finalize on the first onError: a reconnect still delivers paragraphs', () => {
    startBodyWarmup(7, 'zh-CN', 5);
    const seen: number[] = [];
    subscribeBodyWarmup(7, 'zh-CN', (p) => seen.push(p.index));
    sse.clients[0].error!(new Event('error'));
    expect(getBodyWarmup(7, 'zh-CN')?.done).toBe(false);
    sse.clients[0].paragraph!({ index: 0, translation: '零' });
    expect(seen).toEqual([0]);
    expect(getBodyWarmup(7, 'zh-CN')?.done).toBe(false);
  });

  it('releaseBodyWarmup tears down immediately and drops the entry', () => {
    startBodyWarmup(7, 'zh-CN', 5);
    releaseBodyWarmup(7, 'zh-CN');
    expect(sse.clients[0].close).toHaveBeenCalledTimes(1);
    expect(getBodyWarmup(7, 'zh-CN')).toBeUndefined();
  });

  it('scheduleCancel tears down only after the delay, for an unclaimed entry', () => {
    startBodyWarmup(7, 'zh-CN', 5);
    scheduleCancelBodyWarmup(7, 'zh-CN', 800);
    vi.advanceTimersByTime(799);
    expect(getBodyWarmup(7, 'zh-CN')).toBeDefined();
    vi.advanceTimersByTime(1);
    expect(sse.clients[0].close).toHaveBeenCalledTimes(1);
    expect(getBodyWarmup(7, 'zh-CN')).toBeUndefined();
  });

  it('claim before the scheduled-cancel fires aborts the teardown (navigation handoff)', () => {
    startBodyWarmup(7, 'zh-CN', 5);
    scheduleCancelBodyWarmup(7, 'zh-CN', 800);
    claimBodyWarmup(7, 'zh-CN');
    vi.advanceTimersByTime(2000);
    expect(sse.clients[0].close).not.toHaveBeenCalled();
    expect(getBodyWarmup(7, 'zh-CN')).toBeDefined();
  });

  it('re-warming an entry with a pending scheduled-cancel aborts that teardown', () => {
    startBodyWarmup(7, 'zh-CN', 5);
    scheduleCancelBodyWarmup(7, 'zh-CN', 800);
    startBodyWarmup(7, 'zh-CN', 5);
    vi.advanceTimersByTime(2000);
    expect(getBodyWarmup(7, 'zh-CN')).toBeDefined();
    expect(sse.createSSEClient).toHaveBeenCalledTimes(1);
  });

  it('scheduleCancelBodyWarmup is a no-op when already claimed', () => {
    startBodyWarmup(7, 'zh-CN', 5);
    claimBodyWarmup(7, 'zh-CN');
    scheduleCancelBodyWarmup(7, 'zh-CN', 800);
    vi.advanceTimersByTime(2000);
    expect(sse.clients[0].close).not.toHaveBeenCalled();
    expect(getBodyWarmup(7, 'zh-CN')).toBeDefined();
  });

  it('releaseBodyWarmup clears a pending scheduled cancel (no dangling timer)', () => {
    startBodyWarmup(7, 'zh-CN', 5);
    scheduleCancelBodyWarmup(7, 'zh-CN', 800);
    releaseBodyWarmup(7, 'zh-CN');
    expect(sse.clients[0].close).toHaveBeenCalledTimes(1);
    vi.advanceTimersByTime(2000);
    expect(sse.clients[0].close).toHaveBeenCalledTimes(1);
  });
});
