import { type InfiniteData, type QueryClient } from '@tanstack/react-query';
import type { ArticleItem, ArticleTab, ArticleListResponse } from '@/lib/types';

export interface ArticleStateChange {
  articleId: number;
  is_read?: boolean;
  is_starred?: boolean;
}

type ArticleListData = InfiniteData<ArticleListResponse>;
const ARTICLE_TABS: ArticleTab[] = ['today', 'stream', 'starred'];

function mergeArticleItem<T extends { id: number; is_read?: boolean; is_starred?: boolean }>(
  item: T,
  change: ArticleStateChange,
): T {
  return {
    ...item,
    ...(change.is_read === undefined ? {} : { is_read: change.is_read }),
    ...(change.is_starred === undefined ? {} : { is_starred: change.is_starred }),
  };
}

function updateArticlePage(
  page: ArticleListResponse,
  change: ArticleStateChange,
  tab: ArticleTab,
  articleDetail?: ArticleItem,
) {
  const itemIndex = page.items.findIndex((item) => item.id === change.articleId);
  if (itemIndex < 0) {
    if (tab === 'starred' && change.is_starred === true && articleDetail) {
      return {
        ...page,
        items: [mergeArticleItem(articleDetail, change), ...page.items],
      };
    }

    return page;
  }

  if (tab === 'starred' && change.is_starred === false) {
    return {
      ...page,
      items: page.items.filter((item) => item.id !== change.articleId),
    };
  }

  return {
    ...page,
    items: page.items.map((item) =>
      item.id === change.articleId ? mergeArticleItem(item, change) : item,
    ),
  };
}

function updateArticleListData(
  existing: ArticleListData | undefined,
  change: ArticleStateChange,
  tab: ArticleTab,
  articleDetail?: ArticleItem,
) {
  if (!existing) {
    return existing;
  }

  const previousItem =
    existing.pages.flatMap((page) => page.items).find((item) => item.id === change.articleId) ??
    articleDetail;
  const shouldAdjustReadCounts =
    change.is_read !== undefined &&
    previousItem?.is_read !== undefined &&
    Boolean(previousItem.is_read) !== change.is_read;
  const readDelta = shouldAdjustReadCounts ? (change.is_read ? 1 : -1) : 0;

  const pages = existing.pages.map((page) => updateArticlePage(page, change, tab, articleDetail));

  if (shouldAdjustReadCounts && pages[0]?.counts) {
    pages[0] = {
      ...pages[0],
      counts: {
        all: pages[0].counts.all,
        unread: Math.max(0, pages[0].counts.unread - readDelta),
        read: Math.max(0, pages[0].counts.read + readDelta),
      },
    };
  }

  return { ...existing, pages };
}

export function applyArticleStateChange(queryClient: QueryClient, change: ArticleStateChange) {
  const detailKey = ['article', String(change.articleId)] as const;
  const articleDetail = queryClient.getQueryData<ArticleItem>(detailKey);

  queryClient.setQueryData<ArticleItem | undefined>(detailKey, (existing) => {
    if (!existing) {
      return existing;
    }

    return mergeArticleItem(existing, change);
  });

  for (const [queryKey, existing] of queryClient.getQueriesData<ArticleListData | undefined>({
    queryKey: ['articles'],
  })) {
    const [, tab] = queryKey as [string, ArticleTab];
    if (!ARTICLE_TABS.includes(tab)) {
      continue;
    }

    queryClient.setQueryData(
      queryKey,
      updateArticleListData(existing, change, tab, articleDetail),
    );
  }
}
