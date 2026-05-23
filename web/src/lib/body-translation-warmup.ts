import { createSSEClient, type SSEClient, type SSEParagraphEvent } from '@/lib/sse-client';

type Subscriber = (paragraph: SSEParagraphEvent) => void;

interface WarmupEntry {
  translations: Map<number, string>;
  done: boolean;
  claimed: boolean;
  client: SSEClient;
  subscribers: Set<Subscriber>;
  cancelTimer: ReturnType<typeof setTimeout> | null;
}

const entries = new Map<string, WarmupEntry>();

/**
 * Deferred-cancel delay. Must outlast the reader's AnimatePresence exit
 * (page.tsx: mode="wait", transition duration 0.15s) PLUS the next reader and
 * its BilingualBody mounting and calling claimBodyWarmup. 800ms is comfortably
 * safe and still well under any realistic user dwell on the new article.
 */
export const DEFAULT_CANCEL_DELAY_MS = 800;

function keyOf(articleId: number, nativeLanguage: string): string {
  return `${articleId}:${nativeLanguage}`;
}

function clearCancelTimer(entry: WarmupEntry): void {
  if (entry.cancelTimer !== null) {
    clearTimeout(entry.cancelTimer);
    entry.cancelTimer = null;
  }
}

function teardown(key: string): void {
  const entry = entries.get(key);
  if (!entry) {
    return;
  }
  clearCancelTimer(entry);
  entry.client?.close(); // close() is idempotent and also clears the SSE reconnect timer
  entries.delete(key);
}

/**
 * Start warming the first `count` paragraphs of an article's body translation
 * into the module store, reusing the existing body-translation SSE (which
 * triggers AI generation on a cache miss). De-duplicated by (articleId,
 * nativeLanguage): a second call while an entry exists is a no-op AND aborts
 * any pending scheduled teardown (the warm-up is wanted again), so the gate
 * re-firing or rapid nav cannot double the AI cost or kill a live stream.
 */
export function startBodyWarmup(articleId: number, nativeLanguage: string, count: number): void {
  const key = keyOf(articleId, nativeLanguage);
  const existing = entries.get(key);
  if (existing) {
    clearCancelTimer(existing);
    return;
  }
  const params = new URLSearchParams({ start: '0', count: String(count) });
  const client = createSSEClient(`/api/articles/${articleId}/body-translation?${params.toString()}`);
  const entry: WarmupEntry = {
    translations: new Map(),
    done: false,
    claimed: false,
    client,
    subscribers: new Set(),
    cancelTimer: null,
  };
  entries.set(key, entry);

  client.onParagraph((paragraph) => {
    entry.translations.set(paragraph.index, paragraph.translation);
    entry.subscribers.forEach((cb) => cb(paragraph));
  });
  // onDone (server "done" or "same-language") is the ONLY terminal signal.
  // Keep the client reference so a teardown can still close() it.
  client.onDone(() => {
    entry.done = true;
  });
  // Do NOT finalize on the first onError: createSSEClient retries once itself
  // and a successful reconnect resumes delivery through the same onParagraph
  // listener. If the reconnect also fails the client just goes silent (no
  // event); the entry stays not-done with whatever it collected and the
  // mounted BilingualBody falls back to its own range requests for the
  // missing indices. Registering a no-op keeps that listener wired.
  client.onError(() => {});
}

/** Snapshot of the paragraphs warmed so far, or undefined if not warming. */
export function getBodyWarmup(
  articleId: number,
  nativeLanguage: string,
): { translations: Map<number, string>; done: boolean } | undefined {
  const entry = entries.get(keyOf(articleId, nativeLanguage));
  if (!entry) {
    return undefined;
  }
  return { translations: new Map(entry.translations), done: entry.done };
}

/**
 * Subscribe to remaining in-flight warmed paragraphs (adoption by the mounted
 * BilingualBody). Returns an unsubscribe fn. No-op subscription if the entry is
 * missing or already done (caller should seed from getBodyWarmup in that case).
 */
export function subscribeBodyWarmup(
  articleId: number,
  nativeLanguage: string,
  cb: Subscriber,
): () => void {
  const entry = entries.get(keyOf(articleId, nativeLanguage));
  if (!entry || entry.done) {
    return () => {};
  }
  entry.subscribers.add(cb);
  return () => {
    entry.subscribers.delete(cb);
  };
}

/**
 * Mark an entry as adopted by a mounted consumer (the user opened it) and abort
 * any pending scheduled teardown. After claim, only releaseBodyWarmup tears it
 * down (an explicit "done with it"), never a scheduled cancel.
 */
export function claimBodyWarmup(articleId: number, nativeLanguage: string): void {
  const entry = entries.get(keyOf(articleId, nativeLanguage));
  if (entry) {
    entry.claimed = true;
    clearCancelTimer(entry);
  }
}

/**
 * Schedule teardown of a speculative warm-up the user navigated away from.
 * Deferred (not immediate) because under AnimatePresence mode="wait" the old
 * reader unmounts BEFORE the new one mounts: if the user actually navigated
 * INTO this article, the new BilingualBody mounts and calls claimBodyWarmup
 * (which clears this timer) within the delay, surviving the gap. A genuinely
 * skipped warm-up is torn down when the timer fires. No-op if already claimed,
 * already gone, or a teardown is already scheduled.
 */
export function scheduleCancelBodyWarmup(
  articleId: number,
  nativeLanguage: string,
  delayMs: number = DEFAULT_CANCEL_DELAY_MS,
): void {
  const key = keyOf(articleId, nativeLanguage);
  const entry = entries.get(key);
  if (!entry || entry.claimed || entry.cancelTimer !== null) {
    return;
  }
  entry.cancelTimer = setTimeout(() => {
    const current = entries.get(key);
    if (current && !current.claimed) {
      teardown(key);
    }
  }, delayMs);
}

/**
 * Immediate teardown after a mounted consumer is done with it (BilingualBody
 * unmount / content swap). Bounds the store to at most the open article plus
 * the single speculative next.
 */
export function releaseBodyWarmup(articleId: number, nativeLanguage: string): void {
  teardown(keyOf(articleId, nativeLanguage));
}

/** Test-only: clear all module state between tests. */
export function __resetBodyWarmupForTests(): void {
  entries.forEach((entry) => {
    clearCancelTimer(entry);
    entry.client?.close();
  });
  entries.clear();
}
