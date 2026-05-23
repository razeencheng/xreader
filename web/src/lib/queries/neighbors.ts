import { useMemo } from 'react';
import { useArticles } from './articles';
import type { ArticleItem, ArticleTab } from '@/lib/types';

export interface ArticleNeighbors {
  prev: ArticleItem | null;
  next: ArticleItem | null;
  position?: number;
  total: number;
}

export function useArticleNeighbors(articleId: number, tab: ArticleTab): ArticleNeighbors {
  const { data } = useArticles(tab);

  return useMemo(() => {
    const items = data?.pages.flatMap((page) => page.items) ?? [];
    const currentIndex = items.findIndex((item) => item.id === articleId);

    if (currentIndex < 0) {
      return {
        prev: null,
        next: null,
        position: undefined,
        total: items.length,
      };
    }

    return {
      prev: items[currentIndex - 1] ?? null,
      next: items[currentIndex + 1] ?? null,
      position: currentIndex + 1,
      total: items.length,
    };
  }, [articleId, data]);
}
