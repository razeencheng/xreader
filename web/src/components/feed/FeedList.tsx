'use client';

import { useEffect, useMemo, useState } from 'react';
import Link from 'next/link';
import { useInView } from 'react-intersection-observer';
import { ChevronLeft, PlusCircle, Upload } from 'lucide-react';
import { useQuery } from '@tanstack/react-query';
import { apiFetch } from '@/lib/api-client';
import { useI18n } from '@/lib/i18n';
import { useArticles } from '@/lib/queries/articles';
import { useBulkRead } from '@/hooks/useBulkRead';
import { useUIStore } from '@/stores/useUIStore';
import { ReadFilterSegmentedControl } from './ReadFilterSegmentedControl';
import { FeedRowComfortable } from './FeedRowComfortable';
import { FeedRowCompact } from './FeedRowCompact';
import { FeedSkeleton, CompactSkeleton } from './FeedSkeleton';
import { getSourceColor } from '@/lib/source-meta';
import type { ArticleItem, ArticleTab, Source } from '@/lib/types';

type FeedArticleItem = ArticleItem & {
  is_starred?: boolean;
  is_read?: boolean;
};

interface FeedListProps {
  onOpenArticle?: (article: ArticleItem) => void;
  selectedArticleId?: number | null;
}

export function FeedList({ onOpenArticle, selectedArticleId = null }: FeedListProps) {
  const { t } = useI18n();
  const currentView = useUIStore((state) => state.currentView);
  const selectedSourceId = useUIStore((state) => state.selectedSourceId);
  const density = useUIStore((state) => state.density);
  const readFilter = useUIStore((state) => state.readFilter);
  const setCurrentView = useUIStore((state) => state.setCurrentView);
  const setReadFilter = useUIStore((state) => state.setReadFilter);

  const tab: ArticleTab = currentView === 'starred' ? 'starred' : currentView === 'today' ? 'today' : 'stream';

  const articleReadFilter = currentView !== 'starred' && readFilter !== 'all' ? readFilter : undefined;
  const { data, fetchNextPage, hasNextPage, isFetchingNextPage, isLoading } = useArticles(
    tab,
    currentView === 'sources' ? selectedSourceId : null,
    articleReadFilter,
  );

  const { data: sourceData, isLoading: isSourceListLoading } = useQuery<Source[]>({
    queryKey: ['sources'],
    queryFn: () => apiFetch<Source[]>('/api/sources'),
  });
  const sources = useMemo(() => (Array.isArray(sourceData) ? sourceData : []), [sourceData]);
  const hasLoadedSourceList = Array.isArray(sourceData);

  const { ref, inView } = useInView({ rootMargin: '200px 0px' });
  const items = useMemo(() => data?.pages.flatMap((page) => page.items) ?? [], [data]);
  const selectedSource = useMemo(
    () => sources.find((source) => source.id === selectedSourceId) ?? null,
    [selectedSourceId, sources],
  );
  const [selectedIndex, setSelectedIndex] = useState(0);

  const {
    pendingReadIds,
    bulkReadUndo,
    isBulkUpdating,
    openBulkConfirmScope,
    setOpenBulkConfirmScope,
    bulkScope,
    updateArticleReadState,
    handleBulkMarkRead,
    handleUndoBulkMarkRead,
  } = useBulkRead(items as FeedArticleItem[]);

  useEffect(() => {
    if (!inView || !hasNextPage || isFetchingNextPage) return;
    void fetchNextPage();
  }, [fetchNextPage, hasNextPage, inView, isFetchingNextPage]);

  const counts = useMemo(() => {
    const serverCounts = data?.pages[0]?.counts;
    if (serverCounts) {
      return serverCounts;
    }

    return {
      unread: items.filter((item) => !item.is_read).length,
      read: items.filter((item) => item.is_read).length,
      all: items.length,
    };
  }, [data?.pages, items]);

  const filteredItems = useMemo(() => {
    if (currentView === 'starred') return items;
    if (articleReadFilter === 'unread') {
      return items.filter((item) => !item.is_read || pendingReadIds.has(item.id));
    }
    if (articleReadFilter === 'read') {
      return items.filter((item) => item.is_read);
    }
    if (readFilter === 'unread') return items.filter((item) => !item.is_read || pendingReadIds.has(item.id));
    if (readFilter === 'read') return items.filter((item) => item.is_read);
    return items;
  }, [articleReadFilter, currentView, items, pendingReadIds, readFilter]);

  const externalIndex = selectedArticleId == null ? -1 : filteredItems.findIndex((item) => item.id === selectedArticleId);
  const activeIndex =
    externalIndex >= 0 ? externalIndex : filteredItems.length === 0 ? -1 : Math.min(selectedIndex, filteredItems.length - 1);

  const RowComponent = density === 'compact' ? FeedRowCompact : FeedRowComfortable;
  const headerLabel =
    currentView === 'today'
      ? t('nav.today')
      : currentView === 'starred'
        ? t('nav.starred')
        : currentView === 'sources'
          ? selectedSource?.title ?? t('sources.title')
          : t('nav.all');
  const showReadFilters = currentView !== 'starred';
  const sourceColor = selectedSource ? getSourceColor(selectedSource) : null;
  const showBulkRead = Boolean(showReadFilters && readFilter === 'unread' && bulkScope && counts.unread > 0);
  const isBulkConfirmOpen = Boolean(showBulkRead && bulkScope && openBulkConfirmScope === bulkScope.scope);
  const showSourceOnboarding =
    hasLoadedSourceList && !isSourceListLoading && sources.length === 0 && filteredItems.length === 0;

  return (
    <div className="flex h-full w-full flex-col overflow-hidden bg-[var(--bg)] lg:w-[300px] lg:border-r lg:border-[var(--border)]">
      <header className="shrink-0 border-b border-[var(--border-light)] px-3 bg-[var(--bg)] pb-2 pt-[10px] md:pt-[10px] pt-[6px]">
        {currentView === 'sources' && selectedSource ? (
          <button
            type="button"
            onClick={() => setCurrentView('sources', null)}
            className="hidden md:flex items-center gap-1 pb-[6px] text-[11.5px] text-[var(--text-3)] transition-colors hover:text-[var(--text-2)]"
          >
            <ChevronLeft size={13} />
            {t('feed.allSources')}
          </button>
        ) : null}

        <div className={`hidden md:flex items-center gap-[7px] ${showReadFilters ? 'mb-[7px]' : ''}`}>
          {sourceColor ? <span className="inline-block h-[9px] w-[9px] rounded-[2px]" style={{ backgroundColor: sourceColor }} /> : null}
          <span className="text-[14px] font-semibold text-[var(--text)]">{headerLabel}</span>
        </div>

        {showReadFilters ? (
          <>
            <div className="flex items-center justify-between gap-2">
              <ReadFilterSegmentedControl
                fullWidth
                value={readFilter}
                counts={counts}
                onChange={(nextReadFilter) => {
                  setReadFilter(nextReadFilter);
                  setOpenBulkConfirmScope(null);
                }}
              />
              {showBulkRead ? (
                <div className="relative shrink-0 self-stretch">
                  <button
                    type="button"
                    aria-label={t('feed.allRead')}
                    aria-expanded={isBulkConfirmOpen}
                    onClick={() => setOpenBulkConfirmScope((scope) => (scope === bulkScope?.scope ? null : (bulkScope?.scope ?? null)))}
                    disabled={isBulkUpdating}
                    className="inline-flex h-full items-center rounded-[10px] bg-[var(--bg-panel)] px-2.5 text-[13px] font-semibold leading-none text-[var(--text-3)] ring-1 ring-inset ring-[var(--border-light)] transition-colors hover:text-[var(--text-2)] disabled:cursor-not-allowed disabled:opacity-58"
                  >
                    {t('feed.allRead')}
                  </button>
                  {isBulkConfirmOpen ? (
                    <div className="absolute right-0 top-[calc(100%+6px)] z-50 w-[238px] rounded-[10px] border border-[var(--border)] bg-[var(--bg-elevated)] p-2 shadow-[0_18px_48px_rgba(30,24,16,0.16)]">
                      <div className="px-2 pb-2 pt-1">
                        <div className="text-[11.5px] font-semibold text-[var(--text)]">{t('feed.confirmAllRead')}</div>
                        <div className="mt-0.5 text-[10.5px] leading-4 text-[var(--text-3)]">
                          {t('feed.confirmAllReadDescription')}
                        </div>
                      </div>
                      <div className="flex items-center justify-end gap-1.5 px-1 pt-1">
                        <button
                          type="button"
                          onClick={() => setOpenBulkConfirmScope(null)}
                          disabled={isBulkUpdating}
                          className="ui-btn-ghost min-h-11 rounded-[10px] px-3 py-1.5 text-[11px] font-medium md:h-8 md:min-h-0"
                        >
                          {t('feed.cancel')}
                        </button>
                        <button
                          type="button"
                          aria-label={t('feed.confirmAllReadAria')}
                          onClick={() => void handleBulkMarkRead()}
                          disabled={isBulkUpdating}
                          className="ui-btn-solid min-h-11 rounded-[10px] px-3 py-1.5 text-[11px] font-semibold md:h-8 md:min-h-0"
                        >
                          {t('feed.confirm')}
                        </button>
                      </div>
                    </div>
                  ) : null}
                </div>
              ) : null}
            </div>
            {bulkReadUndo ? (
              <div className="mt-2 rounded-xl border border-[var(--accent-border)] bg-[var(--accent-soft)] px-3 py-2 text-[11.5px] text-[var(--accent-text)]">
                <span>
                  {t('feed.bulkReadNotice', { scope: bulkReadUndo.label, count: bulkReadUndo.articleIds.length })}
                </span>
                <button
                  type="button"
                  aria-label={t('feed.undoBulkAria')}
                  onClick={() => void handleUndoBulkMarkRead()}
                  disabled={isBulkUpdating}
                  className="ml-2 font-semibold underline-offset-2 hover:underline disabled:opacity-60"
                >
                  {t('feed.undo')}
                </button>
              </div>
            ) : null}
          </>
        ) : null}
      </header>

      <div className="flex-1 overflow-y-auto overscroll-none">
        {isLoading && items.length === 0 ? (
          density === 'compact' ? <CompactSkeleton /> : <FeedSkeleton />
        ) : filteredItems.length === 0 ? (
          showSourceOnboarding ? (
            <div className="flex h-full items-center justify-center px-7 py-10 text-center">
              <div className="w-full max-w-[238px]">
                <div className="mx-auto mb-4 flex h-10 w-10 items-center justify-center rounded-[10px] border border-[var(--accent-border)] bg-[var(--accent-soft)] text-[var(--accent)]">
                  <PlusCircle size={18} strokeWidth={1.8} />
                </div>
                <div className="font-serif text-[18px] font-semibold tracking-tight text-[var(--text)]">
                  {t('feed.emptySourcesTitle')}
                </div>
                <p className="mt-2 text-[12.5px] leading-5 text-[var(--text-3)]">
                  {t('feed.emptySourcesDescription')}
                </p>
                <div className="mt-5 flex flex-col gap-2">
                  <Link href="/sources#add-source" className="ui-btn-primary h-9 rounded-[10px] px-3 py-2 text-[12px]">
                    <PlusCircle size={14} strokeWidth={1.9} />
                    {t('sources.addTitle')}
                  </Link>
                  <Link href="/sources#opml" className="ui-btn-ghost min-h-11 rounded-[10px] px-3 py-1.5 text-[12px] md:h-8 md:min-h-0">
                    <Upload size={14} strokeWidth={1.9} />
                    {t('sources.importOpmlAction')}
                  </Link>
                </div>
                <p className="mt-4 text-[11.5px] leading-5 text-[var(--text-3)]">
                  {t('feed.emptySourcesHint')}
                </p>
              </div>
            </div>
          ) : (
            <div className="px-7 py-10 text-center text-[13px] text-[var(--text-3)]">
              {readFilter === 'unread' ? t('feed.allCaughtUp') : t('feed.nothingHere')}
            </div>
          )
        ) : (
          <div className="flex flex-col">
            {filteredItems.map((item, index) => (
              <RowComponent
                key={item.id}
                item={item as FeedArticleItem}
                selected={index === activeIndex}
                pendingRead={pendingReadIds.has(item.id)}
                onClick={() => {
                  setSelectedIndex(index);
                  onOpenArticle?.(item);
                }}
                onMarkRead={() => void updateArticleReadState(item as FeedArticleItem, true)}
                onUndoRead={() => void updateArticleReadState(item as FeedArticleItem, false)}
              />
            ))}
            <div ref={ref} className="h-10" />
            {isFetchingNextPage ? (
              <div className="flex justify-center py-8">
                <div className="h-4 w-4 animate-spin rounded-full border-2 border-[var(--accent)] border-t-transparent" />
              </div>
            ) : null}
          </div>
        )}
      </div>
    </div>
  );
}
