'use client';

import { useMemo, useState } from 'react';
import { usePathname, useRouter } from 'next/navigation';
import { BadgePlus, Globe, Highlighter, Keyboard, Settings } from 'lucide-react';
import { motion } from 'framer-motion';
import { LanguageModal } from '@/components/layout/LanguageModal';
import { getLanguageOption, PRIMARY_NAV_ITEMS } from '@/components/layout/navigationConfig';
import { useI18n } from '@/lib/i18n';
import { useUIStore } from '@/stores/useUIStore';

const NAV_LABEL_KEYS = {
  today: 'nav.today',
  all: 'nav.all',
  starred: 'nav.starred',
  sources: 'nav.sources',
} as const;

export function Sidebar({ className = '' }: { className?: string }) {
  const [isLanguageOpen, setIsLanguageOpen] = useState(false);
  const router = useRouter();
  const pathname = usePathname();
  const currentView = useUIStore((state) => state.currentView);
  const setCurrentView = useUIStore((state) => state.setCurrentView);
  const nativeLanguage = useUIStore((state) => state.nativeLanguage);
  const setNativeLanguage = useUIStore((state) => state.setNativeLanguage);
  const openShortcuts = useUIStore((state) => state.openShortcuts);
  const { t } = useI18n();

  const currentLanguage = useMemo(
    () => getLanguageOption(nativeLanguage),
    [nativeLanguage],
  );

  const handleSelectView = (view: typeof PRIMARY_NAV_ITEMS[number]['id']) => {
    setCurrentView(view, view === 'sources' ? null : undefined);
    if (pathname !== '/') {
      router.push('/');
    }
  };

  return (
    <>
      <aside className={`flex h-full w-[52px] flex-col items-center gap-1 border-r border-[var(--border)] bg-[var(--bg-panel)] px-0 py-[10px] ${className}`}>
        <div className="mb-[14px] select-none font-serif text-[15px] font-semibold italic tracking-[-0.08em] text-[var(--accent)]">
          x
        </div>

        <nav className="flex w-full flex-1 flex-col items-center gap-1">
          {PRIMARY_NAV_ITEMS.map((item) => {
            const active = currentView === item.id;

            return (
              <button
                key={item.id}
                type="button"
                title={t(NAV_LABEL_KEYS[item.id])}
                onClick={() => handleSelectView(item.id)}
                className={`relative flex h-10 w-10 items-center justify-center rounded-[9px] transition-colors ${
                  active
                    ? 'text-[var(--accent)]'
                    : 'text-[var(--text-3)] hover:bg-[var(--bg-hover)] hover:text-[var(--text-2)]'
                }`}
              >
                {active ? <motion.span layoutId="sidebar-active-bg" className="absolute inset-0 rounded-[9px] bg-[var(--accent-bg)]" /> : null}
                <item.icon size={17} strokeWidth={1.75} className="relative z-10" />
              </button>
            );
          })}

          <button
            type="button"
            title={t('sources.addTitle')}
            onClick={() => router.push('/sources#add-source')}
            className="relative flex h-10 w-10 items-center justify-center rounded-[9px] text-[var(--text-3)] transition-colors hover:bg-[var(--bg-hover)] hover:text-[var(--text-2)]"
          >
            <BadgePlus size={17} strokeWidth={1.75} />
          </button>
        </nav>

        <button
          type="button"
          title={t('shortcuts.open')}
          onClick={openShortcuts}
          className="mb-1 flex h-10 w-10 items-center justify-center rounded-[9px] text-[var(--text-3)] transition-colors hover:bg-[var(--bg-hover)] hover:text-[var(--text-2)]"
        >
          <Keyboard size={16} strokeWidth={1.75} />
        </button>

        <button
          type="button"
          title={t('nav.highlights')}
          onClick={() => router.push('/highlights')}
          className="mb-1 flex h-10 w-10 items-center justify-center rounded-[9px] text-[var(--text-3)] transition-colors hover:bg-[var(--bg-hover)] hover:text-[var(--text-2)]"
        >
          <Highlighter size={16} strokeWidth={1.75} />
        </button>

        <button
          type="button"
          title={t('nav.nativeLanguageTitle', { language: currentLanguage.name })}
          onClick={() => setIsLanguageOpen(true)}
          className="flex h-10 w-10 flex-col items-center justify-center gap-[1px] rounded-[9px] text-[var(--text-3)] transition-colors hover:bg-[var(--bg-hover)] hover:text-[var(--text-2)]"
        >
          <Globe size={15} strokeWidth={1.75} />
          <span className="text-[8px] font-semibold leading-none tracking-[0.03em] text-[var(--accent)]">
            {currentLanguage.short}
          </span>
        </button>

        <button
          type="button"
          title={t('nav.settings')}
          onClick={() => router.push('/settings')}
          className="flex h-10 w-10 items-center justify-center rounded-[9px] text-[var(--text-3)] transition-colors hover:bg-[var(--bg-hover)] hover:text-[var(--text-2)]"
        >
          <Settings size={17} strokeWidth={1.75} />
        </button>
      </aside>

      {isLanguageOpen ? (
        <LanguageModal
          currentLanguage={nativeLanguage}
          onSelect={setNativeLanguage}
          onClose={() => setIsLanguageOpen(false)}
        />
      ) : null}
    </>
  );
}
