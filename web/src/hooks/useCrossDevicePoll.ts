'use client';

import { useEffect, useRef } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { apiFetch } from '@/lib/api-client';
import { applyArticleStateChange, type ArticleStateChange } from '@/lib/article-state-cache';

interface ServerArticleChange {
  article_id: number;
  changed_at: string;
  is_read?: boolean;
  is_starred?: boolean;
}

interface ArticleChangeResponse {
  items: ServerArticleChange[];
}

function toStateChange(item: ServerArticleChange): ArticleStateChange {
  return { articleId: item.article_id, is_read: item.is_read, is_starred: item.is_starred };
}

const POLL_INTERVAL_MS = 30_000;

function laterTimestamp(left: string, right: string) {
  return left > right ? left : right;
}

export function useCrossDevicePoll(enabled = true) {
  const queryClient = useQueryClient();
  const sinceRef = useRef<string>(new Date(0).toISOString());
  const inFlightRef = useRef(false);

  useEffect(() => {
    if (!enabled) {
      return;
    }

    let cancelled = false;

    const poll = async () => {
      if (document.visibilityState !== 'visible' || inFlightRef.current) {
        return;
      }

      const pollStartedAt = new Date().toISOString();
      const pollSince = sinceRef.current;
      inFlightRef.current = true;

      try {
        const response = await apiFetch<ArticleChangeResponse>(
          `/api/articles/changes?since=${encodeURIComponent(pollSince)}`,
        );

        if (cancelled) {
          return;
        }

        // /changes returns the authoritative current state per changed article, so
        // applying every snapshot is idempotent and convergent (incl. this tab's own
        // changes). Gating on local-echo dedup is unnecessary and could drop a
        // genuine remote change that shares an article with a recent local change.
        let newestChangeAt = pollStartedAt;
        for (const item of response.items ?? []) {
          newestChangeAt = laterTimestamp(newestChangeAt, item.changed_at);
          applyArticleStateChange(queryClient, toStateChange(item));
        }

        sinceRef.current = newestChangeAt;
      } catch {
        // Ignore transient polling failures.
      } finally {
        inFlightRef.current = false;
      }
    };

    void poll();

    const intervalId = window.setInterval(() => {
      void poll();
    }, POLL_INTERVAL_MS);

    const handleVisibilityChange = () => {
      if (document.visibilityState === 'visible') {
        void poll();
      }
    };

    document.addEventListener('visibilitychange', handleVisibilityChange);

    return () => {
      cancelled = true;
      window.clearInterval(intervalId);
      document.removeEventListener('visibilitychange', handleVisibilityChange);
    };
  }, [enabled, queryClient]);
}
