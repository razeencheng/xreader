'use client';

import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { motion } from 'framer-motion';
import { ApiError, apiFetch } from '@/lib/api-client';
import { applyArticleStateChange } from '@/lib/article-state-cache';
import { broadcast } from '@/lib/broadcast';
import { estimateReadMinutes, formatRelativeTime, getDisplayTitle, isLikelySummaryOnly, isSameLanguage } from '@/lib/article-meta';
import { useI18n } from '@/lib/i18n';
import { getActiveReaderLayout, toggleReaderFocusMode } from '@/lib/reader-layout';
import { useHeldScroll } from '@/hooks/useHeldScroll';
import { useNextArticleWarmup } from '@/hooks/useNextArticleWarmup';
import { useReaderGestures } from '@/hooks/useReaderGestures';
import { useUIStore } from '@/stores/useUIStore';
import { KeyPointsCallout } from '@/components/reader/KeyPointsCallout';
import { BilingualBody } from '@/components/reader/BilingualBody';
import { HighlightLayer } from '@/components/reader/HighlightLayer';
import { OriginalArticleButton } from '@/components/reader/OriginalArticleButton';
import { ReaderHeader } from '@/components/reader/ReaderHeader';
import { ReaderGestureHint } from '@/components/reader/ReaderGestureHint';
import { SourceExcerptNotice } from '@/components/reader/SourceExcerptNotice';
import { TweaksPanel } from '@/components/reader/TweaksPanel';
import type { ArticleItem } from '@/lib/types';

export interface ArticleDetail extends ArticleItem {
  content_html?: string;
  content_text?: string;
  is_read?: boolean;
  is_starred?: boolean;
}

interface ArticleAI {
  title_translated?: string;
  summary?: string;
}

interface OriginalContent {
  url: string;
  title?: string;
  content_html: string;
  content_text: string;
}

export interface ArticleReaderProps {
  id: string;
  onClose?: () => void;
  onNext?: () => void;
  onPrev?: () => void;
  hasNext?: boolean;
  hasPrev?: boolean;
  /** Optional position/total for display in ReaderHeader */
  position?: number;
  total?: number;
  /** Content rendered after the article body (e.g. NextUpCard) */
  afterBody?: React.ReactNode;
  /** Content rendered after the scroll area (e.g. PrevNextBar) */
  afterScroll?: React.ReactNode;
  className?: string;
  onNotFound?: () => void;
  /**
   * Next article (id + language) for #3 prefetch/warm-up; null when none.
   * Must be referentially stable (memoize at the call site) — the hook's 1s
   * dwell debounce resets on every identity change.
   */
  next?: { id: number; language: string } | null;
}

export function ArticleReader({
  id,
  onClose,
  onNext,
  onPrev,
  hasNext,
  hasPrev,
  position,
  total,
  afterBody,
  afterScroll,
  className = '',
  onNotFound,
  next,
}: ArticleReaderProps) {
  const queryClient = useQueryClient();
  const { t } = useI18n();
  const scrollRef = useRef<HTMLDivElement>(null);
  const autoMarkedArticleIds = useRef(new Set<number>());
  const [progressState, setProgressState] = useState({ articleId: id, value: 0 });
  const [originalContentState, setOriginalContentState] = useState<{ articleId: string; content: OriginalContent } | null>(null);
  const [loadingOriginalId, setLoadingOriginalId] = useState<string | null>(null);
  const [originalErrorState, setOriginalErrorState] = useState<{ articleId: string; message: string } | null>(null);
  const [tweaksOpen, setTweaksOpen] = useState(false);

  const nativeLanguage = useUIStore((state) => state.nativeLanguage);
  const fontSize = useUIStore((state) => state.fontSize);
  const layout = useUIStore((state) => state.layout);
  const focusMode = useUIStore((state) => state.focusMode);
  const setFocusMode = useUIStore((state) => state.setFocusMode);
  const setLayout = useUIStore((state) => state.setLayout);
  const isShortcutsOpen = useUIStore((state) => state.isShortcutsOpen);

  const activeLayout = getActiveReaderLayout(layout, focusMode);
  useHeldScroll(scrollRef, { disabled: isShortcutsOpen });

  const { data: article, isLoading, error } = useQuery({
    queryKey: ['article', id],
    queryFn: () => apiFetch<ArticleDetail>(`/api/articles/${id}`),
  });

  useEffect(() => {
    if (!error || !onNotFound) {
      return;
    }

    const status = error instanceof ApiError ? error.status : undefined;
    if (status === 404) {
      onNotFound();
    }
  }, [error, onNotFound]);

  const titleNeedsTranslation = article ? !isSameLanguage(article.language, nativeLanguage) : false;
  const { data: ai, isFetching: isFetchingAI } = useQuery({
    queryKey: ['article-ai', id, nativeLanguage],
    queryFn: () => apiFetch<ArticleAI>(`/api/articles/${id}/ai?lang=${nativeLanguage}`).catch(() => null),
    enabled: !!article && titleNeedsTranslation,
  });

  const progress = progressState.articleId === id ? progressState.value : 0;
  useNextArticleWarmup({
    currentId: id,
    next,
    nativeLanguage,
    articleLoaded: !!article,
    progress,
  });
  const originalContent = originalContentState?.articleId === id ? originalContentState.content : null;
  const isLoadingOriginal = loadingOriginalId === id;
  const originalError = originalErrorState?.articleId === id ? originalErrorState.message : null;

  const handleScroll = useCallback(() => {
    const element = scrollRef.current;
    if (!element) return;

    const scrollHeight = element.scrollHeight - element.clientHeight;
    if (scrollHeight <= 0) {
      setProgressState({ articleId: id, value: 1 });
      return;
    }

    setProgressState({ articleId: id, value: Math.min(1, element.scrollTop / scrollHeight) });
  }, [id]);

  useEffect(() => {
    if (!article) return;
    const frame = requestAnimationFrame(() => handleScroll());
    return () => cancelAnimationFrame(frame);
  }, [article, handleScroll]);

  // Auto-mark as read at 75% scroll
  useEffect(() => {
    if (!article || article.is_read || progress < 0.75 || autoMarkedArticleIds.current.has(article.id)) {
      return;
    }

    autoMarkedArticleIds.current.add(article.id);
    applyArticleStateChange(queryClient, { articleId: article.id, is_read: true });

    void apiFetch(`/api/articles/${article.id}/state`, {
      method: 'PATCH',
      body: JSON.stringify({ is_read: true }),
    })
      .then(() => {
        broadcast({ type: 'state-change', articleId: article.id, is_read: true });
      })
      .catch(() => {
        autoMarkedArticleIds.current.delete(article.id);
        applyArticleStateChange(queryClient, { articleId: article.id, is_read: article.is_read });
      });
  }, [article, progress, queryClient]);

  const handleToggleStar = useCallback(async () => {
    if (!article) return;

    const nextStarred = !article.is_starred;
    applyArticleStateChange(queryClient, { articleId: article.id, is_starred: nextStarred });

    try {
      await apiFetch(`/api/articles/${article.id}/state`, {
        method: 'PATCH',
        body: JSON.stringify({ is_starred: nextStarred }),
      });
      broadcast({ type: 'state-change', articleId: article.id, is_starred: nextStarred });
    } catch {
      applyArticleStateChange(queryClient, { articleId: article.id, is_starred: article.is_starred });
    }
  }, [article, queryClient]);

  const handleShare = useCallback(async () => {
    if (!article) return;

    try {
      await navigator.clipboard.writeText(article.link);
    } catch {
      window.open(article.link, '_blank', 'noopener,noreferrer');
    }
  }, [article]);

  const handleOpenOriginal = useCallback(() => {
    if (!article) return;
    window.open(article.link, '_blank', 'noopener,noreferrer');
  }, [article]);

  const handleToggleFocus = useCallback(() => {
    toggleReaderFocusMode(focusMode, layout, setLayout, setFocusMode);
  }, [focusMode, layout, setFocusMode, setLayout]);

  const handleLoadOriginal = useCallback(async () => {
    if (!article) return;

    setLoadingOriginalId(id);
    setOriginalErrorState(null);
    try {
      const content = await apiFetch<OriginalContent>(`/api/articles/${article.id}/original`, { method: 'POST' });
      setOriginalContentState({ articleId: id, content });
      queryClient.setQueryData<ArticleDetail>(['article', id], (previous) =>
        previous ? { ...previous, content_html: content.content_html, content_text: content.content_text } : previous,
      );
      setProgressState({ articleId: id, value: 0 });
      scrollRef.current?.scrollTo({ top: 0, behavior: 'smooth' });
    } catch (error) {
      const message = error instanceof Error ? error.message : t('reader.originalLoadError');
      setOriginalErrorState({ articleId: id, message });
    } finally {
      setLoadingOriginalId(null);
    }
  }, [article, id, queryClient, t]);

  const displayTitle = article
    ? ai?.title_translated || article.title_translated || getDisplayTitle(article)
    : '';
  const showOriginalTitle = Boolean(article && titleNeedsTranslation && displayTitle !== article.title);
  const titleLoading = Boolean(article && titleNeedsTranslation && !displayTitle && isFetchingAI);
  const summary = ai?.summary || article?.summary || '';
  const relativeTime = article?.published_at ? formatRelativeTime(article.published_at) : '';
  const showSourceExcerptNotice = article ? isLikelySummaryOnly(article) && !originalContent : false;
  const contentHtml = originalContent?.content_html || article?.content_html || '';
  const contentText = originalContent?.content_text || article?.content_text || '';
  const readMinutes = article ? estimateReadMinutes({ ...article, content_html: contentHtml, content_text: contentText }) : null;

  const resolvedHasNext = hasNext ?? Boolean(onNext);
  const resolvedHasPrev = hasPrev ?? Boolean(onPrev);

  const { gestureHint, touchHandlers } = useReaderGestures({
    scrollRef,
    progress,
    hasNext: resolvedHasNext,
    hasPrev: resolvedHasPrev,
    onNext: () => onNext?.(),
    onPrev: () => onPrev?.(),
    onBack: () => onClose?.(),
  });

  const bylineItems = useMemo(() => {
    if (!article) return [] as Array<{ key: string; content: React.ReactNode }>;

    const items: Array<{ key: string; content: React.ReactNode }> = [];
    if (article.source_title) {
      items.push({ key: 'source', content: <span>{article.source_title}</span> });
    }
    if (relativeTime) {
      items.push({ key: 'age', content: <span>{t('article.ago', { time: relativeTime })}</span> });
    }
    if (readMinutes) {
      items.push({ key: 'time', content: <span>{t('article.minRead', { count: readMinutes })}</span> });
    }

    return items;
  }, [article, readMinutes, relativeTime, t]);

  if (isLoading) {
    return (
      <div className={`flex h-full flex-col bg-[var(--bg)] ${className}`}>
        <div className="mx-auto w-full max-w-[680px] space-y-6 px-7 py-16">
          <div className="h-4 w-28 animate-pulse rounded bg-[var(--bg-hover)]" />
          <div className="h-12 w-full animate-pulse rounded bg-[var(--bg-hover)]" />
          <div className="h-12 w-2/3 animate-pulse rounded bg-[var(--bg-hover)]" />
          <div className="space-y-4 pt-8">
            <div className="h-4 w-full animate-pulse rounded bg-[var(--bg-hover)]" />
            <div className="h-4 w-full animate-pulse rounded bg-[var(--bg-hover)]" />
            <div className="h-4 w-5/6 animate-pulse rounded bg-[var(--bg-hover)]" />
          </div>
        </div>
      </div>
    );
  }

  if (!article) return null;

  return (
    <div className={`relative flex h-full flex-col overflow-hidden bg-[var(--bg)] ${className}`}>
      <div className="absolute left-0 right-0 top-0 z-[60] h-[2.5px] bg-[var(--border-light)]">
        <motion.div className="h-full rounded-r-[2px] bg-[var(--accent)]" initial={{ width: 0 }} animate={{ width: `${progress * 100}%` }} />
      </div>

      <ReaderHeader
        article={{ ...article, title_translated: displayTitle }}
        position={position}
        total={total}
        progress={progress}
        focusMode={focusMode}
        onBack={onClose}
        onToggleStar={handleToggleStar}
        onToggleFocus={handleToggleFocus}
        onShare={handleShare}
        onOpenTweaks={() => setTweaksOpen((v) => !v)}
      />

      <ReaderGestureHint hint={gestureHint} />

      <div
        ref={scrollRef}
        data-reader-scroll="true"
        onScroll={handleScroll}
        className="flex-1 overflow-y-auto overflow-x-hidden overscroll-none touch-pan-y"
        {...touchHandlers}
      >
        <HighlightLayer articleId={Number(id)}>
          <div className={`pb-12 pt-[44px] ${activeLayout === 'wide' ? 'px-7 md:px-14' : 'px-7 md:px-7'}`}>
            <article className={activeLayout === 'wide' ? 'mx-auto max-w-[960px]' : 'mx-auto max-w-[680px]'}>
              {titleLoading ? (
                <div className="mb-3 inline-flex items-center gap-2 font-serif text-[18px] text-[var(--text-3)]">
                  <span className="h-4 w-4 animate-spin rounded-full border-2 border-[var(--accent)] border-t-transparent" />
                  {t('reader.translatingTitle')}
                </div>
              ) : (
                <h1
                  className="font-serif font-semibold leading-[1.22] tracking-[-0.03em] text-[var(--text)]"
                  style={{ fontSize: `${fontSize + 10}px` }}
                >
                  {displayTitle}
                </h1>
              )}

              {showOriginalTitle ? (
                <p className="mb-3 mt-[6px] font-serif italic leading-[1.4] text-[var(--text-3)]" style={{ fontSize: `${fontSize + 1}px` }}>
                  {article.title}
                </p>
              ) : null}

              {bylineItems.length > 0 ? (
                <div className="mb-9 flex flex-wrap items-center gap-[10px] text-[12.5px] text-[var(--text-3)]">
                  {bylineItems.map((item, index) => (
                    <div key={item.key} className="inline-flex items-center gap-[10px]">
                      {index > 0 ? <span className="text-[var(--border)]">·</span> : null}
                      {item.content}
                    </div>
                  ))}
                  <span className="text-[var(--border)]">·</span>
                  <OriginalArticleButton href={article.link} onOpen={handleOpenOriginal} />
                </div>
              ) : null}

              {summary ? <KeyPointsCallout text={summary} /> : null}

              {showSourceExcerptNotice ? (
                <SourceExcerptNotice
                  error={originalError}
                  isLoading={isLoadingOriginal}
                  onLoadOriginal={handleLoadOriginal}
                />
              ) : null}

              <div className="font-reader-text min-w-0 max-w-full text-[var(--text)]" style={{ fontSize: `${fontSize}px`, lineHeight: 1.9 }}>
                {contentHtml ? (
                  <BilingualBody
                    articleId={article.id}
                    contentHtml={contentHtml}
                    language={article.language}
                    nativeLanguage={nativeLanguage}
                  />
                ) : (
                  <p className="overflow-wrap-anywhere">{contentText}</p>
                )}
              </div>

              {afterBody}
            </article>
          </div>
        </HighlightLayer>
      </div>

      {afterScroll}
      <TweaksPanel externalOpen={tweaksOpen} onExternalClose={() => setTweaksOpen(false)} />
    </div>
  );
}
