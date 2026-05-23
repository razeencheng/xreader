'use client';

import { useRouter, useSearchParams } from 'next/navigation';
import { estimateReadMinutes, formatRelativeTime, getDisplayTitle, getOriginalTitle } from '@/lib/article-meta';
import { useI18n } from '@/lib/i18n';
import { getSourceColor } from '@/lib/source-meta';
import type { ArticleItem } from '@/lib/types';

interface Props {
  next: ArticleItem;
  currentId: number;
  markRead: (id: number) => void | Promise<void>;
}

function buildHref(articleId: number, searchParams: URLSearchParams) {
  const params = new URLSearchParams(searchParams.toString());
  params.set('article', String(articleId));
  return `/?${params.toString()}`;
}

function langLabel(lang?: string) {
  if (!lang) return null;
  const map: Record<string, string> = { en: 'EN', ja: 'JA', zh: '中', 'zh-cn': '中', ko: 'KO' };
  return map[lang.toLowerCase()] ?? lang.slice(0, 2).toUpperCase();
}

export function NextUpCard({ next, currentId, markRead }: Props) {
  const { t } = useI18n();
  const router = useRouter();
  const searchParams = useSearchParams();
  const displayTitle = getDisplayTitle(next);
  const originalTitle = getOriginalTitle(next);
  const publishedAt = formatRelativeTime(next.published_at);
  const readMinutes = estimateReadMinutes(next);
  const summary = next.summary?.trim();
  const sourceColor = getSourceColor(next.source_title);
  const meta = [
    publishedAt ? t('article.ago', { time: publishedAt }) : null,
    readMinutes ? t('article.minRead', { count: readMinutes }) : null,
    langLabel(next.language),
  ]
    .filter(Boolean)
    .join(' · ');

  const handleClick = async () => {
    try {
      await markRead(currentId);
    } catch {
      // Keep navigation responsive even if the background mark-read call fails.
    }

    router.push(buildHref(next.id, searchParams));
  };

  return (
    <div className="mb-7 font-sans">
      <button
        type="button"
        onClick={handleClick}
        className="group w-full rounded-2xl border border-[var(--border-strong)] bg-[var(--bg-callout)] px-4 py-3 text-left text-sm leading-6 text-[var(--text-secondary)] transition-all hover:-translate-y-[1px] hover:border-[var(--accent)]"
      >
        <div className="min-w-0">
          <div className="mb-2 flex items-center gap-2 text-[10px] font-semibold tracking-[0.18em] text-[var(--text-3)]">
            <span>{t('reader.nextArticle')}</span>
            <span className="text-[var(--border-strong)]">·</span>
            <span>{t('reader.pressNext')}</span>
          </div>
          <div className="mb-1 font-serif text-[23px] font-semibold leading-[1.24] tracking-[-0.02em] text-[var(--text-body)]">{displayTitle}</div>
          {originalTitle ? (
            <div className="mb-2 font-serif text-[15px] italic leading-[1.45] text-[var(--text-3)]">{originalTitle}</div>
          ) : null}
          <div className="mb-3 flex flex-wrap items-center gap-x-2 gap-y-1 text-[12px] text-[var(--text-3)]">
            <span className="inline-flex items-center gap-2 rounded-[10px] bg-[var(--bg-elevated)] px-2.5 py-1 text-[11px] font-medium text-[var(--text-2)]">
              <span className="inline-block h-2.5 w-2.5 rounded-full" style={{ backgroundColor: sourceColor }} />
              {next.source_title || t('common.source')}
            </span>
            {meta ? <span>{meta}</span> : null}
          </div>
        </div>
      </button>
    </div>
  );
}
