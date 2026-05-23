'use client';

import { useEffect, useMemo, useRef, useState, type MouseEvent, type ReactNode } from 'react';
import { useI18n } from '@/lib/i18n';
import { useFocusTrap } from '@/hooks/useFocusTrap';
import { deleteHighlight, fetchHighlights, updateHighlightNote, type Highlight } from '@/lib/queries/highlights';
import { HighlightToolbar } from './HighlightToolbar';

interface HighlightEditorDialogProps {
  editor: { highlight: Highlight; top: number; left: number };
  noteDraft: string;
  setNoteDraft: (value: string) => void;
  onClose: () => void;
  onSaveNote: () => void;
  onRemoveHighlight: () => void;
  onCopyQuote: () => void;
}

function HighlightEditorDialog({
  editor,
  noteDraft,
  setNoteDraft,
  onClose,
  onSaveNote,
  onRemoveHighlight,
  onCopyQuote,
}: HighlightEditorDialogProps) {
  const { t } = useI18n();
  const trapRef = useFocusTrap<HTMLDivElement>({ onEscape: onClose });

  return (
    <div
      ref={trapRef}
      role="dialog"
      aria-label={t('reader.highlightNoteTitle')}
      className="fixed z-[120] w-[min(360px,calc(100vw-32px))] rounded-[14px] border border-[var(--border)] bg-[var(--bg-panel)] p-3 shadow-[0_18px_54px_rgba(0,0,0,0.18)]"
      style={{ top: editor.top, left: editor.left, transform: 'translateX(-50%)' }}
    >
      <div className="mb-2 line-clamp-3 rounded-[9px] bg-[var(--bg-hover)] px-3 py-2 text-[12px] leading-relaxed text-[var(--text-2)]">
        {editor.highlight.quoted_text}
      </div>
      <label className="block">
        <span className="mb-1 block text-[11px] font-medium text-[var(--text-3)]">{t('reader.noteLabel')}</span>
        <textarea
          aria-label={t('reader.noteLabel')}
          value={noteDraft}
          onChange={(event) => setNoteDraft(event.target.value)}
          className="min-h-[84px] w-full resize-y rounded-[10px] border border-[var(--border)] bg-[var(--bg)] px-3 py-2 text-sm leading-relaxed text-[var(--text)] outline-none focus:border-[var(--accent)]"
        />
      </label>
      <div className="mt-3 flex flex-wrap items-center justify-between gap-2">
        <div className="flex items-center gap-2">
          <button type="button" onClick={onCopyQuote} className="inline-flex min-h-11 items-center justify-center rounded-[9px] px-4 py-2 text-xs font-medium text-[var(--text-3)] hover:bg-[var(--bg-hover)]">
            {t('reader.copyQuote')}
          </button>
          <button type="button" onClick={onRemoveHighlight} className="inline-flex min-h-11 items-center justify-center rounded-[9px] px-4 py-2 text-xs font-medium text-[var(--text-error)] hover:bg-[var(--bg-hover)]">
            {t('reader.deleteHighlight')}
          </button>
        </div>
        <div className="flex items-center gap-2">
          <button type="button" onClick={onClose} className="inline-flex min-h-11 items-center justify-center rounded-[9px] px-4 py-2 text-xs font-medium text-[var(--text-3)] hover:bg-[var(--bg-hover)]">
            {t('reader.cancelNote')}
          </button>
          <button type="button" onClick={onSaveNote} className="inline-flex min-h-11 items-center justify-center rounded-[9px] bg-[var(--accent)] px-4 py-2 text-xs font-semibold text-white hover:opacity-90">
            {t('reader.saveNote')}
          </button>
        </div>
      </div>
    </div>
  );
}

interface Props {
  articleId: number;
  refreshKey?: number;
  children?: ReactNode;
}

export function HighlightLayer({ articleId, refreshKey, children }: Props) {
  const [highlights, setHighlights] = useState<Highlight[]>([]);
  const [editor, setEditor] = useState<{ highlight: Highlight; top: number; left: number } | null>(null);
  const [noteDraft, setNoteDraft] = useState('');
  const containerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    let cancelled = false;
    fetchHighlights(articleId)
      .then((items) => {
        if (!cancelled) setHighlights(items);
      })
      .catch(() => undefined);

    return () => {
      cancelled = true;
    };
  }, [articleId, refreshKey]);

  const grouped = useMemo(() => {
    const map = new Map<string, Highlight[]>();
    for (const highlight of highlights) {
      const key = `${highlight.layer}:${highlight.paragraph_index}`;
      const list = map.get(key) ?? [];
      list.push(highlight);
      map.set(key, list);
    }
    return map;
  }, [highlights]);

  useEffect(() => {
    const root = containerRef.current;
    if (!root) return;

    const marks = Array.from(root.querySelectorAll('[data-highlight-id]')) as HTMLElement[];
    for (const mark of marks) {
      const parent = mark.parentElement;
      if (parent?.dataset.highlightContainer === 'true') {
        const text = mark.textContent ?? '';
        mark.replaceWith(document.createTextNode(text));
      }
    }

    for (const [key, paragraphHighlights] of grouped.entries()) {
      const [layer, paragraphIndex] = key.split(':');
      const paragraph = root.querySelector(`[data-layer="${layer}"][data-paragraph-index="${paragraphIndex}"]`);
      if (!paragraph) continue;

      const textNodes: Array<{ node: Text; start: number; end: number }> = [];
      const walker = document.createTreeWalker(paragraph, NodeFilter.SHOW_TEXT);
      let node: Node | null;
      let offset = 0;
      while ((node = walker.nextNode())) {
        const text = node.textContent ?? '';
        textNodes.push({ node: node as Text, start: offset, end: offset + text.length });
        offset += text.length;
      }

      paragraphHighlights.forEach((highlight) => {
        const range = document.createRange();
        const startNode = textNodes.find((entry) => highlight.text_start_offset >= entry.start && highlight.text_start_offset <= entry.end);
        const endNode = textNodes.find((entry) => highlight.text_end_offset >= entry.start && highlight.text_end_offset <= entry.end);
        if (!startNode || !endNode) return;

        const startOffset = highlight.text_start_offset - startNode.start;
        const endOffset = highlight.text_end_offset - endNode.start;
        if (
          startOffset < 0 ||
          endOffset < 0 ||
          startOffset > startNode.node.length ||
          endOffset > endNode.node.length ||
          (startNode.node === endNode.node && endOffset <= startOffset)
        ) {
          return;
        }

        try {
          range.setStart(startNode.node, startOffset);
          range.setEnd(endNode.node, endOffset);
        } catch {
          // Skip highlights whose stored offsets no longer map to the rendered DOM.
          return;
        }

        const mark = document.createElement('mark');
        mark.dataset.highlightId = String(highlight.id);
        mark.id = `highlight-${highlight.id}`;
        mark.className = 'bg-[var(--bg-highlight-yellow)] cursor-pointer rounded-[2px] px-0.5';
        if (highlight.note) mark.title = highlight.note;
        try {
          range.surroundContents(mark);
          if (typeof window !== 'undefined' && window.location.hash === `#highlight-${highlight.id}`) {
            window.requestAnimationFrame(() => mark.scrollIntoView({ block: 'center' }));
          }
        } catch {
          // Ignore invalid ranges on partially rendered DOM.
        }
      });
    }
  }, [children, grouped]);

  const reload = async () => {
    try {
      setHighlights(await fetchHighlights(articleId));
    } catch {
      // ignore
    }
  };

  const openEditor = (mark: HTMLElement, highlight: Highlight) => {
    const rect = mark.getBoundingClientRect();
    setNoteDraft(highlight.note ?? '');
    setEditor({
      highlight,
      top: Math.min(window.innerHeight - 220, Math.max(16, rect.bottom + 8)),
      left: Math.min(window.innerWidth - 180, Math.max(180, rect.left + rect.width / 2)),
    });
  };

  const handleMarkOpen = (event: MouseEvent<HTMLDivElement>) => {
    const target = event.target as HTMLElement | null;
    const mark = target?.closest('mark[data-highlight-id]') as HTMLElement | null;
    if (!mark) return;

    event.preventDefault();
    const id = Number(mark.dataset.highlightId);
    const highlight = highlights.find((item) => item.id === id);
    if (!highlight) return;

    openEditor(mark, highlight);
  };

  const saveNote = async () => {
    if (!editor) return;
    await updateHighlightNote(editor.highlight.id, noteDraft);
    setEditor(null);
    await reload();
  };

  const removeHighlight = async () => {
    if (!editor) return;
    await deleteHighlight(editor.highlight.id);
    setEditor(null);
    await reload();
  };

  const copyQuote = async () => {
    if (!editor) return;
    try {
      await navigator.clipboard.writeText(editor.highlight.quoted_text);
    } catch {
      // ignore
    }
  };

  return (
    <div
      ref={containerRef}
      onClick={handleMarkOpen}
      onContextMenu={handleMarkOpen}
      data-highlight-container="true"
    >
      <HighlightToolbar articleId={articleId} onHighlightCreated={reload} />
      {children}
      {editor ? (
        <HighlightEditorDialog
          editor={editor}
          noteDraft={noteDraft}
          setNoteDraft={setNoteDraft}
          onClose={() => setEditor(null)}
          onSaveNote={saveNote}
          onRemoveHighlight={removeHighlight}
          onCopyQuote={copyQuote}
        />
      ) : null}
    </div>
  );
}
