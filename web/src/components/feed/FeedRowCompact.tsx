'use client';

import { CircleCheck } from 'lucide-react';
import { motion } from 'framer-motion';
import { formatRelativeTime, getDisplayTitle, getOriginalTitle } from '@/lib/article-meta';
import { useI18n } from '@/lib/i18n';
import { getSourceColor } from '@/lib/source-meta';
import type { ArticleItem } from '@/lib/types';

interface Props {
  item: ArticleItem & { is_read?: boolean; is_starred?: boolean };
  selected?: boolean;
  pendingRead?: boolean;
  onClick?: () => void;
  onMarkRead?: () => void;
  onUndoRead?: () => void;
}

export function FeedRowCompact({ item, selected = false, pendingRead = false, onClick, onMarkRead, onUndoRead }: Props) {
  const { t } = useI18n();
  const sourceName = (item.source_title?.trim() || t('article.untitledSource')).toUpperCase();
  const relativeTime = formatRelativeTime(item.published_at);
  const displayTitle = getDisplayTitle(item);
  const originalTitle = getOriginalTitle(item);
  const sourceColor = getSourceColor(item.source_title);
  const dimmed = (item.is_read || pendingRead) && !selected;

  return (
    <div
      role="button"
      aria-current={selected ? 'true' : undefined}
      tabIndex={0}
      onClick={onClick}
      onKeyDown={(event) => {
        if (event.key === 'Enter' || event.key === ' ') {
          event.preventDefault();
          onClick?.();
        }
      }}
      className={`group relative cursor-pointer border-b border-[var(--border-light)] px-[14px] py-[9px] transition-[background,opacity] duration-150 ${
        selected ? 'bg-[var(--bg-selected)]' : 'hover:bg-[var(--bg-hover)]'
      } ${dimmed ? 'opacity-[0.52]' : 'opacity-100'}`}
    >
      {selected ? (
        <motion.div
          layoutId="active-article-indicator"
          className="absolute inset-y-[22%] left-0 w-[2.5px] rounded-r bg-[var(--accent)]"
        />
      ) : null}

      <article>
        <div className="mb-1 flex items-center gap-[5px]">
          <span className="inline-block h-[10px] w-[10px] shrink-0 rounded-[2px]" style={{ backgroundColor: sourceColor }} />
          <span className="flex-1 truncate text-[10.5px] font-medium uppercase tracking-[0.03em] text-[var(--text-3)]">
            {sourceName}
          </span>
          {relativeTime ? <span className="text-[11px] text-[var(--text-3)]">{relativeTime}</span> : null}
        </div>

        <div className="text-[13px] font-semibold leading-[1.38] text-[var(--text)]">{displayTitle}</div>
        {originalTitle ? <div className="mt-[3px] text-[11px] italic leading-[1.35] text-[var(--text-3)]">{originalTitle}</div> : null}
        {pendingRead ? (
          <button
            type="button"
            aria-label={t('feed.undoReadAria')}
            onClick={(event) => {
              event.stopPropagation();
              onUndoRead?.();
            }}
            className="mt-[6px] inline-flex min-h-11 min-w-11 items-center justify-center rounded-[10px] bg-[var(--bg-elevated)] px-3 py-2 text-[10.5px] font-medium text-[var(--text-3)] shadow-[inset_0_0_0_1px_var(--border)] transition-colors hover:text-[var(--accent)] md:min-h-0 md:min-w-0 md:px-2 md:py-[3px]"
          >
            {t('feed.readUndo')}
          </button>
        ) : onMarkRead && !item.is_read ? (
          <button
            type="button"
            aria-label={t('feed.markRead')}
            onClick={(event) => {
              event.stopPropagation();
              onMarkRead();
            }}
            className="mt-[6px] inline-flex min-h-11 min-w-11 items-center justify-center rounded p-[3px] text-[var(--text-3)] opacity-100 transition-[color,opacity] hover:text-[var(--accent)] md:min-h-0 md:min-w-0 md:opacity-0 md:group-hover:opacity-100 md:focus:opacity-100"
          >
            <CircleCheck size={15} strokeWidth={1.5} />
          </button>
        ) : null}
      </article>
    </div>
  );
}
