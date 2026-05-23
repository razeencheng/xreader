'use client';

import { useMemo } from 'react';
import { useUIStore } from '@/stores/useUIStore';
import { useAuthStore } from '@/stores/useAuthStore';
import { useQuery } from '@tanstack/react-query';
import { apiFetch } from '@/lib/api-client';
import { useI18n } from '@/lib/i18n';
import { getSourceColor, orderSourceGroups } from '@/lib/source-meta';
import type { Source } from '@/lib/types';


function UnreadBadge({ count }: { count?: number }) {
  if (!count) return null;

  return (
    <span className="inline-flex h-[18px] min-w-[18px] items-center justify-center rounded-full bg-[var(--accent-solid)] px-[5px] text-[10px] font-semibold leading-none text-[var(--accent-on-solid)]">
      {count}
    </span>
  );
}

function SourceButton({
  title,
  unreadCount,
  color,
  active,
  indent = false,
  onClick,
}: {
  title: string;
  unreadCount?: number;
  color: string;
  active: boolean;
  indent?: boolean;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={`relative flex w-full items-center gap-[9px] border-none text-left transition-colors ${
        indent ? 'px-4 py-[7px] pl-[22px]' : 'px-4 py-[8px]'
      } ${active ? 'bg-[var(--bg-selected)]' : 'bg-transparent hover:bg-[var(--bg-hover)]'}`}
    >
      {active ? <span className="absolute inset-y-[20%] left-0 w-[2.5px] rounded-r bg-[var(--accent)]" /> : null}
      <span className="inline-block h-[10px] w-[10px] shrink-0 rounded-[2px]" style={{ backgroundColor: color }} />
      <span
        className={`flex-1 truncate text-[13px] ${
          unreadCount ? 'font-semibold text-[var(--text)]' : 'font-normal text-[var(--text-2)]'
        }`}
      >
        {title}
      </span>
      <UnreadBadge count={unreadCount} />
    </button>
  );
}

export function SourceBrowser() {
  const { t } = useI18n();
  const { user } = useAuthStore();
  const selectedSourceId = useUIStore((state) => state.selectedSourceId);
  const setCurrentView = useUIStore((state) => state.setCurrentView);

  const { data: sources = [], isLoading } = useQuery<Source[]>({
    queryKey: ['sources'],
    queryFn: () => apiFetch<Source[]>('/api/sources'),
    enabled: !!user,
  });

  const groupedSources = useMemo(() => orderSourceGroups(sources), [sources]);
  const totalUnread = useMemo(
    () => sources.reduce((sum, source) => sum + (source.unread_count ?? 0), 0),
    [sources],
  );

  if (isLoading) {
    return (
      <div className="flex h-full w-full flex-col gap-3 bg-[var(--bg)] px-4 py-4 lg:w-[300px] lg:border-r lg:border-[var(--border)]">
        <div className="h-10 animate-pulse rounded-xl bg-[var(--bg-hover)]" />
        <div className="h-9 animate-pulse rounded-xl bg-[var(--bg-hover)]" />
        <div className="space-y-2 pt-2">
          {[1, 2, 3, 4].map((item) => (
            <div key={item} className="h-8 animate-pulse rounded-lg bg-[var(--bg-hover)]" />
          ))}
        </div>
      </div>
    );
  }

  return (
    <section
      className="flex h-full w-full flex-col overflow-hidden bg-[var(--bg)] lg:w-[300px] lg:border-r lg:border-[var(--border)]"
    >
      <header className="shrink-0 border-b border-[var(--border-light)] px-4 pb-[10px] pt-[11px]">
        <div className="min-w-0">
          <h1 className="text-[14px] font-semibold text-[var(--text)]">{t('sources.title')}</h1>
          <p className="mt-[2px] text-[11.5px] text-[var(--text-3)]">
            {t('sources.unreadFeeds', { unread: totalUnread, feeds: sources.length })}
          </p>
        </div>
      </header>

      <div className="flex-1 overflow-y-auto pb-2">
        <button
          type="button"
          onClick={() => setCurrentView('sources', null)}
          className={`relative flex w-full items-center gap-[9px] border-b border-[var(--border-light)] px-4 py-[9px] text-left transition-colors ${
            selectedSourceId === null ? 'bg-[var(--bg-selected)]' : 'bg-transparent hover:bg-[var(--bg-hover)]'
          }`}
        >
          {selectedSourceId === null ? <span className="absolute inset-y-[20%] left-0 w-[2.5px] rounded-r bg-[var(--accent)]" /> : null}
          <span className="flex-1 text-[13px] font-semibold text-[var(--text)]">{t('sources.allSources')}</span>
          <UnreadBadge count={totalUnread} />
        </button>

        {groupedSources.map(([category, items]) => (
          <section key={category} className="mt-[6px]">
            <div className="px-4 pb-[3px] pt-[5px] text-[10.5px] font-medium uppercase tracking-[0.07em] text-[var(--text-3)]">
              {category}
            </div>
            {items.map((source) => (
              <SourceButton
                key={source.id}
                title={source.title}
                unreadCount={source.unread_count}
                color={getSourceColor(source)}
                active={selectedSourceId === source.id}
                indent
                onClick={() => setCurrentView('sources', source.id)}
              />
            ))}
          </section>
        ))}
      </div>
    </section>
  );
}
