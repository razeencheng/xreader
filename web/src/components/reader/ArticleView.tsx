'use client';

import { useCallback, useMemo } from 'react';
import { apiFetch } from '@/lib/api-client';
import { broadcast } from '@/lib/broadcast';
import { useQueryClient } from '@tanstack/react-query';
import { applyArticleStateChange } from '@/lib/article-state-cache';
import { ArticleReader } from '@/components/reader/ArticleReader';
import { NextUpCard } from '@/components/reader/NextUpCard';
import type { ArticleItem } from '@/lib/types';

interface ArticleViewProps {
  id: string;
  onClose?: () => void;
  onNext?: () => void;
  onPrev?: () => void;
  onNotFound?: () => void;
  className?: string;
  prev?: ArticleItem | null;
  next?: ArticleItem | null;
  position?: number;
  total?: number;
}

export function ArticleView({
  id,
  onClose,
  onNext,
  onPrev,
  onNotFound,
  className = '',
  prev,
  next,
  position,
  total,
}: ArticleViewProps) {
  const articleId = Number(id);
  const queryClient = useQueryClient();

  // Intentionally keyed on primitives, not the `next` object identity:
  // filteredItems can produce a new ArticleItem reference for the same article
  // on list refreshes; keying on id+language prevents churning the hook
  // effects (which would reset the 1s dwell debounce in useNextArticleWarmup).
  const nextForWarmup = useMemo(
    () => (next ? { id: next.id, language: next.language } : null),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [next?.id, next?.language],
  );

  const markRead = useCallback(
    async (articleIdToMark: number) => {
      const cached = queryClient.getQueryData<ArticleItem>(['article', String(articleIdToMark)]);
      const previousRead = cached?.is_read ?? false;
      if (previousRead) return; // already read; nothing to do

      applyArticleStateChange(queryClient, { articleId: articleIdToMark, is_read: true });
      try {
        await apiFetch(`/api/articles/${articleIdToMark}/state`, {
          method: 'PATCH',
          body: JSON.stringify({ is_read: true }),
        });
        broadcast({ type: 'state-change', articleId: articleIdToMark, is_read: true });
      } catch {
        applyArticleStateChange(queryClient, { articleId: articleIdToMark, is_read: previousRead });
      }
    },
    [queryClient],
  );

  return (
    <ArticleReader
      id={id}
      onClose={onClose}
      onNext={onNext}
      onPrev={onPrev}
      onNotFound={onNotFound}
      className={className}
      next={nextForWarmup}
      afterBody={
        next ? (
          <div className="mt-16 mb-12">
            <NextUpCard next={next} currentId={articleId} markRead={markRead} />
          </div>
        ) : undefined
      }
    />
  );
}
