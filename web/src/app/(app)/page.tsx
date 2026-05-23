'use client';

import { useCallback, useMemo, Suspense } from 'react';
import { useRouter, useSearchParams } from 'next/navigation';
import { useQueryClient } from '@tanstack/react-query';
import { motion, AnimatePresence } from 'framer-motion';
import { FeedList } from '@/components/feed/FeedList';
import { ArticleView } from '@/components/reader/ArticleView';
import { SourceBrowser } from '@/components/layout/SourceBrowser';
import { apiFetch } from '@/lib/api-client';
import { applyArticleStateChange } from '@/lib/article-state-cache';
import { broadcast } from '@/lib/broadcast';
import { useI18n } from '@/lib/i18n';
import { useReaderShortcuts } from '@/hooks/useReaderShortcuts';
import { useArticles } from '@/lib/queries/articles';
import { toggleReaderFocusMode } from '@/lib/reader-layout';
import { useUIStore } from '@/stores/useUIStore';
import type { ArticleItem, ArticleTab } from '@/lib/types';

function FeedPageContent() {
  const queryClient = useQueryClient();
  const router = useRouter();
  const searchParams = useSearchParams();
  const { t } = useI18n();
  const currentView = useUIStore((state) => state.currentView);
  const selectedSourceId = useUIStore((state) => state.selectedSourceId);
  const readFilter = useUIStore((state) => state.readFilter);
  const layout = useUIStore((state) => state.layout);
  const focusMode = useUIStore((state) => state.focusMode);
  const setLayout = useUIStore((state) => state.setLayout);
  const setFocusMode = useUIStore((state) => state.setFocusMode);

  const tab: ArticleTab = currentView === 'starred' ? 'starred' : currentView === 'today' ? 'today' : 'stream';
  const articleParam = searchParams.get('article');
  const selectedArticleId = articleParam ? Number(articleParam) : null;
  const selectedArticleIdForList =
    typeof selectedArticleId === 'number' && Number.isFinite(selectedArticleId) ? selectedArticleId : null;
  const selectedId = selectedArticleIdForList == null ? null : selectedArticleIdForList.toString();

  const buildReaderUrl = useCallback(
    (articleId: number) => {
      const params = new URLSearchParams(searchParams.toString());
      params.set('article', articleId.toString());
      params.set('ctx', tab);
      return `/?${params.toString()}`;
    },
    [searchParams, tab],
  );

  const handleOpenArticle = useCallback(
    (article: ArticleItem) => {
      router.push(buildReaderUrl(article.id));
    },
    [buildReaderUrl, router],
  );

  const closeArticle = useCallback(() => {
    if (focusMode) {
      setFocusMode(false);
      if (layout === 'focus') {
        setLayout('classic');
      }
    }

    window.location.href = '/';
  }, [focusMode, layout, setFocusMode, setLayout]);

  const showSourceBrowser = currentView === 'sources' && selectedSourceId === null;
  const articleReadFilter = currentView !== 'starred' && readFilter !== 'all' ? readFilter : undefined;
  const { data } = useArticles(tab, currentView === 'sources' ? selectedSourceId : null, articleReadFilter, {
    enabled: !showSourceBrowser,
  });
  const items = useMemo(() => data?.pages.flatMap((page) => page.items) ?? [], [data]);
  const filteredItems = useMemo(() => {
    if (currentView === 'starred') return items;
    if (readFilter === 'unread') return items.filter((item) => !item.is_read || item.id === selectedArticleIdForList);
    if (readFilter === 'read') return items.filter((item) => item.is_read);
    return items;
  }, [currentView, items, readFilter, selectedArticleIdForList]);
  const currentIndex = filteredItems.findIndex((item) => item.id === selectedArticleIdForList);
  const currentArticle = currentIndex >= 0 ? filteredItems[currentIndex] : null;

  const handleArticleNotFound = useCallback(() => {
    const fallbackArticle = filteredItems.find((item) => item.id !== selectedArticleIdForList) ?? null;
    if (fallbackArticle) {
      router.replace(buildReaderUrl(fallbackArticle.id), { scroll: false });
      return;
    }

    closeArticle();
  }, [buildReaderUrl, closeArticle, filteredItems, router, selectedArticleIdForList]);

  const updateArticleState = useCallback(
    async (
      articleId: number,
      nextState: { is_read?: boolean; is_starred?: boolean },
      previousState: { is_read?: boolean; is_starred?: boolean },
    ) => {
      applyArticleStateChange(queryClient, { articleId, ...nextState });

      try {
        await apiFetch(`/api/articles/${articleId}/state`, {
          method: 'PATCH',
          body: JSON.stringify(nextState),
        });
        broadcast({ type: 'state-change', articleId, ...nextState });
      } catch {
        applyArticleStateChange(queryClient, { articleId, ...previousState });
      }
    },
    [queryClient],
  );

  const handleMarkRead = useCallback(
    (article: ArticleItem | null) => {
      if (!article || article.is_read) return;
      void updateArticleState(article.id, { is_read: true }, { is_read: article.is_read });
    },
    [updateArticleState],
  );

  const handleToggleStar = useCallback(
    (article: ArticleItem | null) => {
      if (!article) return;

      void updateArticleState(
        article.id,
        { is_starred: !article.is_starred },
        { is_starred: article.is_starred },
      );
    },
    [updateArticleState],
  );

  const handleToggleFocus = useCallback(() => {
    toggleReaderFocusMode(focusMode, layout, setLayout, setFocusMode);
  }, [focusMode, layout, setFocusMode, setLayout]);

  const selectArticleAtIndex = useCallback(
    (index: number) => {
      const article = filteredItems[index];
      if (!article) return;
      router.push(buildReaderUrl(article.id));
    },
    [buildReaderUrl, filteredItems, router],
  );

  useReaderShortcuts({
    onNext: () => {
      if (currentIndex < filteredItems.length - 1) {
        selectArticleAtIndex(currentIndex + 1);
      }
    },
    onPrev: () => {
      if (currentIndex > 0) {
        selectArticleAtIndex(currentIndex - 1);
      }
    },
    onToggleStar: () => handleToggleStar(currentArticle),
    onMarkRead: () => handleMarkRead(currentArticle),
    onToggleFocus: handleToggleFocus,
  });

  return (
    <div className="flex h-full overflow-hidden bg-[var(--bg)]">
      <motion.div
        animate={{
          width: focusMode ? 0 : 'var(--list-width)',
          opacity: focusMode ? 0 : 1,
          pointerEvents: focusMode ? 'none' : 'auto',
        }}
        transition={{ duration: 0.28, ease: [0.32, 0.72, 0, 1] }}
        className={`relative z-20 h-full shrink-0 overflow-hidden bg-[var(--bg)] lg:border-r lg:border-[var(--border)] ${
          selectedId ? 'hidden lg:flex' : 'flex w-full lg:w-auto'
        }`}
      >
        {showSourceBrowser ? (
          <div className="h-full w-full">
            <SourceBrowser />
          </div>
        ) : (
          <div className="flex h-full w-full flex-col">
            <FeedList onOpenArticle={handleOpenArticle} selectedArticleId={selectedArticleIdForList} />
          </div>
        )}
      </motion.div>

      <main className={`relative h-full min-w-0 flex-1 overflow-hidden bg-[var(--bg)] ${selectedId ? 'block' : 'hidden lg:block'}`}>
        <AnimatePresence mode="wait">
          {selectedId ? (
            <motion.div
              key={selectedId}
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              exit={{ opacity: 0 }}
              transition={{ duration: 0.15 }}
              className="h-full"
            >
              <ArticleView
                id={selectedId}
                onClose={closeArticle}
                onNext={currentIndex >= 0 && currentIndex < filteredItems.length - 1 ? () => selectArticleAtIndex(currentIndex + 1) : undefined}
                onPrev={currentIndex > 0 ? () => selectArticleAtIndex(currentIndex - 1) : undefined}
                onNotFound={handleArticleNotFound}
                className="h-full"
                prev={currentIndex > 0 ? filteredItems[currentIndex - 1] : null}
                next={currentIndex >= 0 && currentIndex < filteredItems.length - 1 ? filteredItems[currentIndex + 1] : null}
                position={currentIndex >= 0 ? currentIndex + 1 : undefined}
                total={filteredItems.length}
              />
            </motion.div>
          ) : (
            <div className="flex h-full items-center justify-center px-12 text-center select-none">
              <div className="max-w-sm space-y-6 text-[var(--text-3)]">
                <div className="font-serif text-[22px] italic text-[var(--text-2)]">{t('feed.selectArticle')}</div>
                <p className="whitespace-pre-line text-sm leading-6">{t('feed.selectArticleHint')}</p>
              </div>
            </div>
          )}
        </AnimatePresence>
      </main>
    </div>
  );
}

export default function FeedPage() {
  return (
    <Suspense fallback={null}>
      <FeedPageContent />
    </Suspense>
  );
}
