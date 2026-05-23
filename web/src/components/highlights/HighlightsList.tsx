'use client';

import { useState, useEffect } from 'react';
import { apiFetch } from '@/lib/api-client';
import { useI18n } from '@/lib/i18n';

interface HighlightRow {
  id: number;
  article_id: number;
  quoted_text: string;
  note?: string;
  paragraph_index: number;
  created_at: string;
  article_title?: string;
  article_link?: string;
}

interface HighlightsResponse {
  items: HighlightRow[];
}

export function HighlightsList() {
  const { t } = useI18n();
  const [highlights, setHighlights] = useState<HighlightRow[]>([]);
  const [query, setQuery] = useState('');
  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    const params = new URLSearchParams();
    if (query) params.set('q', query);
    params.set('limit', '50');

    let cancelled = false;
    queueMicrotask(() => {
      if (!cancelled) setIsLoading(true);
    });
    apiFetch<HighlightsResponse>(`/api/highlights?${params}`)
      .then((response) => {
        if (!cancelled) setHighlights(response?.items ?? []);
      })
      .catch(() => {
        if (!cancelled) setHighlights([]);
      })
      .finally(() => {
        if (!cancelled) setIsLoading(false);
      });

    return () => {
      cancelled = true;
    };
  }, [query]);

  return (
    <div>
      <div className="mb-4">
        <input
          type="text"
          placeholder={t('highlights.searchPlaceholder')}
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          className="w-full rounded-lg border border-[var(--border-default)] bg-[var(--bg-input)] px-4 py-2 text-sm text-[var(--text-body)] placeholder-[var(--text-faint)] outline-none focus:border-[var(--border-accent)]"
        />
      </div>

      {isLoading ? (
        <div className="py-8 text-center text-sm text-[var(--text-muted)]">{t('common.loading')}</div>
      ) : highlights.length === 0 ? (
        <div className="py-8 text-center text-sm text-[var(--text-muted)]">
          {query ? t('highlights.noResults') : t('highlights.empty')}
        </div>
      ) : (
        <div className="divide-y divide-[var(--border-default)]">
          {highlights.map((h) => (
            <a
              key={h.id}
              href={`/?article=${h.article_id}#highlight-${h.id}`}
              className="block py-4 hover:bg-[var(--bg-badge-starred)] -mx-4 px-4 rounded"
            >
              <div className="mb-1 text-xs text-[var(--text-muted)]">
                {h.article_title || t('highlights.articleFallback', { id: h.article_id })}
              </div>
              <div className="mb-1 border-l-2 border-[var(--border-accent)] pl-3 text-sm text-[var(--text-body)]">
                &ldquo;{h.quoted_text}&rdquo;
              </div>
              {h.note && (
                <div className="pl-3 text-xs italic text-[var(--text-muted)]">
                  📝 {h.note}
                </div>
              )}
              <div className="mt-1 text-[10px] text-[var(--text-faint)]">
                {new Date(h.created_at).toLocaleString()}
              </div>
            </a>
          ))}
        </div>
      )}
    </div>
  );
}
