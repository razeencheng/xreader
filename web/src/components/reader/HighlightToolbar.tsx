'use client';

import { useEffect, useState } from 'react';
import { computeAnchor, type HighlightAnchor } from './highlightAnchor';
import { useI18n } from '@/lib/i18n';
import { createHighlight } from '@/lib/queries/highlights';

interface Props {
  articleId: number;
  onHighlightCreated?: () => void;
}

import { Highlighter, MessageSquare } from 'lucide-react';
import { motion, AnimatePresence } from 'framer-motion';

export function HighlightToolbar({ articleId, onHighlightCreated }: Props) {
  const { t } = useI18n();
  const [anchor, setAnchor] = useState<HighlightAnchor | null>(null);
  const [position, setPosition] = useState<{ top: number; left: number } | null>(null);
  const [isNoteEditorOpen, setIsNoteEditorOpen] = useState(false);
  const [noteDraft, setNoteDraft] = useState('');

  useEffect(() => {
    const handlePointerUp = (event: PointerEvent) => {
      const target = event.target instanceof HTMLElement ? event.target : null;
      if (target?.closest('[data-highlight-ui="true"]')) {
        return;
      }
      const selection = window.getSelection();
      if (!selection || selection.isCollapsed || !selection.rangeCount) {
        setAnchor(null);
        setPosition(null);
        setIsNoteEditorOpen(false);
        return;
      }

      const range = selection.getRangeAt(0);
      const computed = computeAnchor(range);
      if (!computed) {
        setAnchor(null);
        setPosition(null);
        setIsNoteEditorOpen(false);
        return;
      }

      const rect = range.getBoundingClientRect();
      setAnchor(computed);
      setPosition({
        top: rect.top - 50, // 50px above selection
        left: rect.left + rect.width / 2,
      });
      setIsNoteEditorOpen(false);
    };

    document.addEventListener('pointerup', handlePointerUp);
    return () => document.removeEventListener('pointerup', handlePointerUp);
  }, []);

  const save = async (note?: string) => {
    if (!anchor) return;
    await createHighlight({
      article_id: articleId,
      layer: anchor.layer,
      paragraph_index: anchor.paragraph_index,
      text_start_offset: anchor.text_start_offset,
      text_end_offset: anchor.text_end_offset,
      quoted_text: anchor.quoted_text,
      note: note?.trim() || undefined,
    });
    setAnchor(null);
    setPosition(null);
    setIsNoteEditorOpen(false);
    setNoteDraft('');
    window.getSelection()?.removeAllRanges();
    onHighlightCreated?.();
  };

  return (
    <AnimatePresence>
      {anchor && position && (
        <motion.div
          data-highlight-ui="true"
          initial={{ opacity: 0, y: 10, scale: 0.9 }}
          animate={{ opacity: 1, y: 0, scale: 1 }}
          exit={{ opacity: 0, y: 10, scale: 0.9 }}
          className="fixed z-[100] flex items-center gap-1 rounded-[10px] border border-[var(--border)] bg-[color-mix(in_oklch,var(--bg-panel)_85%,transparent)] p-1 shadow-2xl backdrop-blur-xl"
          style={{ top: position.top, left: position.left, transform: 'translateX(-50%)' }}
        >
          <button
            onClick={() => save()}
            className="inline-flex min-h-11 min-w-11 items-center justify-center rounded-[10px] p-3 text-[var(--accent)] transition-colors hover:bg-[var(--accent-bg)]"
            title={t('reader.highlight')}
            aria-label={t('reader.highlight')}
          >
            <Highlighter size={16} strokeWidth={2.5} />
          </button>
          <div className="w-[1px] h-4 bg-[var(--border)]" />
          <button
            onClick={() => {
              setNoteDraft('');
              setIsNoteEditorOpen(true);
            }}
            className="inline-flex min-h-11 min-w-11 items-center justify-center rounded-[10px] p-3 text-[var(--accent)] transition-colors hover:bg-[var(--accent-bg)]"
            title={t('reader.highlightWithNote')}
            aria-label={t('reader.highlightWithNote')}
          >
            <MessageSquare size={16} strokeWidth={2.5} />
          </button>
        </motion.div>
      )}
      {anchor && position && isNoteEditorOpen ? (
        <motion.div
          data-highlight-ui="true"
          role="dialog"
          aria-label={t('reader.highlightNoteTitle')}
          initial={{ opacity: 0, y: 8, scale: 0.96 }}
          animate={{ opacity: 1, y: 0, scale: 1 }}
          exit={{ opacity: 0, y: 8, scale: 0.96 }}
          className="fixed z-[101] w-[min(340px,calc(100vw-32px))] rounded-[14px] border border-[var(--border)] bg-[var(--bg-panel)] p-3 shadow-[0_18px_54px_rgba(0,0,0,0.18)]"
          style={{ top: position.top + 48, left: position.left, transform: 'translateX(-50%)' }}
        >
          <label className="block">
            <span className="mb-1 block text-[11px] font-medium text-[var(--text-3)]">{t('reader.noteLabel')}</span>
            <textarea
              aria-label={t('reader.noteLabel')}
              value={noteDraft}
              onChange={(event) => setNoteDraft(event.target.value)}
              className="min-h-[84px] w-full resize-y rounded-[10px] border border-[var(--border)] bg-[var(--bg)] px-3 py-2 text-sm leading-relaxed text-[var(--text)] outline-none focus:border-[var(--accent)]"
            />
          </label>
          <div className="mt-3 flex justify-end gap-2">
            <button type="button" onClick={() => setIsNoteEditorOpen(false)} className="inline-flex min-h-11 items-center justify-center rounded-[9px] px-4 py-2 text-xs font-medium text-[var(--text-3)] hover:bg-[var(--bg-hover)]">
              {t('reader.cancelNote')}
            </button>
            <button type="button" onClick={() => save(noteDraft)} className="inline-flex min-h-11 items-center justify-center rounded-[9px] bg-[var(--accent)] px-4 py-2 text-xs font-semibold text-white hover:opacity-90">
              {t('reader.saveNote')}
            </button>
          </div>
        </motion.div>
      ) : null}
    </AnimatePresence>
  );
}
