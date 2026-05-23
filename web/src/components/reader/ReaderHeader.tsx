'use client';

import { useEffect, useRef, useState } from 'react';
import { ArrowLeft, EllipsisVertical, Maximize2, Minimize2, Settings, Share2, Star } from 'lucide-react';
import { estimateReadMinutes } from '@/lib/article-meta';
import { useI18n } from '@/lib/i18n';
import { getSourceColor } from '@/lib/source-meta';
import type { ArticleItem } from '@/lib/types';

interface Props {
  article: ArticleItem & {
    content_html?: string;
    content_text?: string;
    is_read?: boolean;
    is_starred?: boolean;
  };
  position?: number;
  total?: number;
  nativeLanguage?: string;
  onBack?: () => void;
  onToggleStar?: () => void;
  onToggleFocus?: () => void;
  onShare?: () => void;
  onOpenTweaks?: () => void;
  focusMode?: boolean;
  progress?: number;
  isCompact?: boolean;
}

const iconButtonClass =
  'flex h-10 w-10 items-center justify-center rounded-[9px] border-none bg-transparent text-[var(--text-3)] transition-colors hover:bg-[var(--bg-hover)] hover:text-[var(--text-2)] md:h-[30px] md:w-[30px] md:rounded-[7px]';

export function ReaderHeader({
  article,
  onBack,
  onToggleStar,
  onToggleFocus,
  onShare,
  onOpenTweaks,
  focusMode = false,
  progress = 0,
}: Props) {
  const { t } = useI18n();
  const sourceColor = getSourceColor(article.source_title);
  const sourceTitle = article.source_title?.trim() || t('common.source');
  const readMinutes = estimateReadMinutes(article);
  const showReadState = progress > 0.75 || article.is_read;
  const [menuOpen, setMenuOpen] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!menuOpen) return;
    const handleClick = (event: MouseEvent) => {
      if (menuRef.current && !menuRef.current.contains(event.target as Node)) {
        setMenuOpen(false);
      }
    };
    document.addEventListener('mousedown', handleClick);
    return () => document.removeEventListener('mousedown', handleClick);
  }, [menuOpen]);

  return (
    <div className="flex shrink-0 items-center gap-3 bg-[var(--bg)] px-5 py-[9px] border-b border-[var(--border-light)]">
      {onBack ? (
        <button
          type="button"
          onClick={onBack}
          className={`${iconButtonClass} mr-1 w-auto gap-1 px-2 md:w-[30px] md:px-0`}
          title={t('reader.back')}
          aria-label={t('reader.backToList')}
        >
          <ArrowLeft size={15} strokeWidth={1.8} />
          <span className="text-[12px] font-medium md:hidden">{t('reader.back')}</span>
        </button>
      ) : null}

      <div className="min-w-0 flex-1 overflow-hidden text-[12px] text-[var(--text-3)]">
        <div className="flex items-center gap-1.5 overflow-hidden whitespace-nowrap">
          <span className="inline-block h-[10px] w-[10px] shrink-0 rounded-[2px]" style={{ backgroundColor: sourceColor }} />
          <span className="truncate font-medium text-[var(--text-2)]">{sourceTitle}</span>
          {readMinutes ? <span>· {t('article.minRead', { count: readMinutes })}</span> : null}
          {showReadState ? <span className="font-medium text-[var(--accent)]">· {t('reader.readState')}</span> : null}
        </div>
      </div>

      {onToggleStar ? (
        <button
          type="button"
          onClick={onToggleStar}
          className={`${iconButtonClass} ${article.is_starred ? 'text-[var(--star)] hover:text-[var(--star)]' : ''}`}
          title={t('reader.star')}
        >
          <Star size={15} fill={article.is_starred ? 'currentColor' : 'none'} strokeWidth={article.is_starred ? 0 : 1.8} />
        </button>
      ) : null}

      {/* Desktop: show all actions inline */}
      {onShare ? (
        <button type="button" onClick={onShare} className={`${iconButtonClass} hidden md:flex`} title={t('reader.share')}>
          <Share2 size={15} strokeWidth={1.8} />
        </button>
      ) : null}

      {onToggleFocus ? (
        <button
          type="button"
          onClick={onToggleFocus}
          className={`${iconButtonClass} hidden md:flex ${focusMode ? 'bg-[var(--accent-bg)] text-[var(--accent)] hover:bg-[var(--accent-bg)] hover:text-[var(--accent)]' : ''}`}
          title={focusMode ? t('reader.exitFocusMode') : t('reader.focusMode')}
        >
          {focusMode ? <Minimize2 size={15} strokeWidth={1.8} /> : <Maximize2 size={15} strokeWidth={1.8} />}
        </button>
      ) : null}

      {onOpenTweaks ? (
        <button
          type="button"
          onClick={onOpenTweaks}
          className={`${iconButtonClass} hidden md:flex`}
          title={t('tweaks.open')}
        >
          <Settings size={15} strokeWidth={1.8} />
        </button>
      ) : null}

      {/* Mobile: overflow menu */}
      <div ref={menuRef} className="relative md:hidden">
        <button
          type="button"
          onClick={() => setMenuOpen((v) => !v)}
          className={iconButtonClass}
          aria-label={t('reader.moreActions')}
        >
          <EllipsisVertical size={16} strokeWidth={1.8} />
        </button>

        {menuOpen ? (
          <div className="absolute right-0 top-full z-50 mt-1 min-w-[160px] rounded-[10px] border border-[var(--border)] bg-[var(--bg)] py-1 shadow-[0_8px_24px_rgba(0,0,0,0.12)]">
            {onShare ? (
              <button
                type="button"
                onClick={() => { onShare(); setMenuOpen(false); }}
                className="flex w-full items-center gap-3 px-4 py-[10px] text-[13px] text-[var(--text-2)] hover:bg-[var(--bg-hover)]"
              >
                <Share2 size={15} strokeWidth={1.8} />
                {t('reader.share')}
              </button>
            ) : null}
            {onToggleFocus ? (
              <button
                type="button"
                onClick={() => { onToggleFocus(); setMenuOpen(false); }}
                className="flex w-full items-center gap-3 px-4 py-[10px] text-[13px] text-[var(--text-2)] hover:bg-[var(--bg-hover)]"
              >
                {focusMode ? <Minimize2 size={15} strokeWidth={1.8} /> : <Maximize2 size={15} strokeWidth={1.8} />}
                {focusMode ? t('reader.exitFocusMode') : t('reader.focusMode')}
              </button>
            ) : null}
            {onOpenTweaks ? (
              <button
                type="button"
                onClick={() => { onOpenTweaks(); setMenuOpen(false); }}
                className="flex w-full items-center gap-3 px-4 py-[10px] text-[13px] text-[var(--text-2)] hover:bg-[var(--bg-hover)]"
              >
                <Settings size={15} strokeWidth={1.8} />
                {t('tweaks.open')}
              </button>
            ) : null}
          </div>
        ) : null}
      </div>
    </div>
  );
}
