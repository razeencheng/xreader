import {
  useInfiniteQuery,
  type InfiniteData,
  type UseInfiniteQueryOptions,
} from '@tanstack/react-query';
import { apiFetch } from '@/lib/api-client';
import type { ArticleListResponse, ArticleTab } from '@/lib/types';

type ArticlesQueryKey = ['articles', ArticleTab, number | null | undefined, string | undefined];

type ArticlesQueryOptions = Omit<
  UseInfiniteQueryOptions<
    ArticleListResponse,
    Error,
    InfiniteData<ArticleListResponse>,
    ArticlesQueryKey,
    string | undefined
  >,
  'queryKey' | 'queryFn' | 'initialPageParam' | 'getNextPageParam'
>;

export function useArticles(tab: ArticleTab, sourceId?: number | null, filter?: string, options?: ArticlesQueryOptions) {
  return useInfiniteQuery({
    queryKey: ['articles', tab, sourceId, filter],
    queryFn: async ({ pageParam }) => {
      const params = new URLSearchParams({ tab });
      if (sourceId) params.set('source_id', sourceId.toString());
      if (filter) params.set('filter', filter);
      if (pageParam) params.set('cursor', pageParam);
      
      return apiFetch<ArticleListResponse>(`/api/articles?${params.toString()}`);
    },
    initialPageParam: undefined,
    getNextPageParam: (lastPage) => lastPage.next_cursor ?? undefined,
    ...options,
  });
}
