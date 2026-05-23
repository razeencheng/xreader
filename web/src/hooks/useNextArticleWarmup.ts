'use client';

import { useEffect, useRef } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { apiFetch } from '@/lib/api-client';
import { isSameLanguage } from '@/lib/article-meta';
import { startBodyWarmup, scheduleCancelBodyWarmup } from '@/lib/body-translation-warmup';

export const WARMUP_PARAGRAPHS = 5;
const DWELL_MS = 1000;
const PROGRESS_GATE = 0.2;

interface NextArticleWarmupParams {
  /** The currently-open article id (string, as used by the article-detail query key). */
  currentId: string;
  /** The next article (id + language), or null when there is none. */
  next: { id: number; language: string } | null | undefined;
  /** The reader's native language (same value BilingualBody seeds with). */
  nativeLanguage: string;
  /** True once the current article's detail query has resolved. */
  articleLoaded: boolean;
  /** Current article scroll progress (0..1). */
  progress: number;
  /** Master switch (default true). */
  enabled?: boolean;
}

/**
 * Prefetch the next article (#3): React Query prefetch of its detail (no gate,
 * cheap, no AI), plus a gated/debounced body-translation warm-up of its first
 * WARMUP_PARAGRAPHS paragraphs. Only N+1. The warm-up store de-dupes the AI
 * call; this hook adds the trigger gate (loaded + dwell>1s OR progress>20%),
 * debounce (rapid H/L never reaches the dwell), and a DEFERRED teardown of a
 * speculative warm-up the user never opened (deferred because under
 * AnimatePresence mode="wait" the old reader unmounts before the next one
 * mounts; the opened article's BilingualBody calls claimBodyWarmup to abort
 * the scheduled teardown).
 */
export function useNextArticleWarmup({
  currentId,
  next,
  nativeLanguage,
  articleLoaded,
  progress,
  enabled = true,
}: NextArticleWarmupParams): void {
  const queryClient = useQueryClient();
  const warmedRef = useRef<{ id: number; lang: string } | null>(null);

  // Phase 1: prefetch next-article detail (no gate — cheap, no AI).
  useEffect(() => {
    if (!enabled || !next) {
      return;
    }
    void queryClient.prefetchQuery({
      queryKey: ['article', String(next.id)],
      queryFn: () => apiFetch(`/api/articles/${next.id}`),
    });
  }, [enabled, next, queryClient]);

  // Phase 2: gated/debounced body-translation warm-up of the next article.
  useEffect(() => {
    if (
      !enabled ||
      !articleLoaded ||
      !next ||
      isSameLanguage(next.language, nativeLanguage)
    ) {
      return;
    }

    const fire = () => {
      startBodyWarmup(next.id, nativeLanguage, WARMUP_PARAGRAPHS);
      warmedRef.current = { id: next.id, lang: nativeLanguage };
    };

    if (progress > PROGRESS_GATE) {
      // Re-runs each scroll tick while progress stays > PROGRESS_GATE; safe
      // because startBodyWarmup de-dupes (a one Map lookup no-op when already
      // warming this key).
      fire();
      return;
    }

    const timer = setTimeout(fire, DWELL_MS);
    return () => clearTimeout(timer);
    // currentId is included so the dwell timer restarts when the user navigates.
  }, [enabled, articleLoaded, next, nativeLanguage, progress, currentId]);

  // Schedule a DEFERRED teardown of the previously-warmed article when `next`
  // changes or on unmount. It must be deferred, not immediate: under the
  // reader's AnimatePresence mode="wait", this old reader unmounts BEFORE the
  // next reader (and its BilingualBody) mounts. An immediate cancel here would
  // tear down the very stream the user just navigated into. Instead the store
  // schedules teardown after a delay; if the user opened it, the new
  // BilingualBody's claimBodyWarmup aborts that teardown. The hook never tries
  // to decide "is this the current article" (currentId is stale in this
  // cleanup under mode="wait") — claim/abort owns the handoff.
  useEffect(() => {
    return () => {
      const warmed = warmedRef.current;
      if (warmed) {
        scheduleCancelBodyWarmup(warmed.id, warmed.lang);
        // Clear so the upcoming phase-2 run (for the new `next`) starts fresh.
        warmedRef.current = null;
      }
    };
  }, [next]);
}
