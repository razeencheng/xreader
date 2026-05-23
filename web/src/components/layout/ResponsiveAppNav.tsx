'use client';

import { useMemo, useState } from 'react';
import { usePathname, useRouter } from 'next/navigation';
import { ChevronDown, Globe, Highlighter, Keyboard, LogOut, PlusCircle, Settings, ShieldCheck } from 'lucide-react';
import { LanguageModal } from '@/components/layout/LanguageModal';
import { getLanguageOption, PRIMARY_NAV_ITEMS } from '@/components/layout/navigationConfig';
import { useI18n } from '@/lib/i18n';
import { useAuthStore } from '@/stores/useAuthStore';
import { useUIStore, type ViewTab } from '@/stores/useUIStore';

const NAV_LABEL_KEYS: Record<ViewTab, string> = {
  today: 'nav.today',
  all: 'nav.all',
  starred: 'nav.starred',
  sources: 'nav.sources',
};

function useAppNavigation() {
  const router = useRouter();
  const pathname = usePathname();
  const user = useAuthStore((state) => state.user);
  const logout = useAuthStore((state) => state.logout);
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

  const goToView = (view: ViewTab) => {
    setCurrentView(view, view === 'sources' ? null : undefined);
    if (pathname !== '/' || (typeof window !== 'undefined' && window.location.search.includes('article='))) {
      window.location.href = '/';
    }
  };

  const handleLogout = async () => {
    await logout();
    router.push('/login');
  };

  return {
    currentLanguage,
    currentView,
    goToView,
    handleLogout,
    isAdmin: user?.role === 'admin',
    nativeLanguage,
    openShortcuts,
    pathname,
    router,
    setNativeLanguage,
    t,
  };
}

function NavButton({
  active,
  icon: Icon,
  label,
  onClick,
}: {
  active: boolean;
  icon: typeof PRIMARY_NAV_ITEMS[number]['icon'];
  label: string;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-label={label}
      className={`inline-flex min-h-11 items-center gap-2 rounded-[10px] px-3 py-2 text-[13px] font-medium transition-colors ${
        active
          ? 'bg-[var(--accent-bg)] text-[var(--accent)]'
          : 'text-[var(--text-3)] hover:bg-[var(--bg-hover)] hover:text-[var(--text-2)]'
      }`}
    >
      <Icon size={16} strokeWidth={1.75} />
      <span className="hidden min-[900px]:inline">{label}</span>
    </button>
  );
}

export function TabletTopNav({ focusMode }: { focusMode: boolean }) {
  const [isLanguageOpen, setIsLanguageOpen] = useState(false);
  const nav = useAppNavigation();

  if (focusMode) {
    return null;
  }

  return (
    <>
      <header className="glass-effect hidden h-14 shrink-0 items-center justify-between gap-2 px-4 md:flex min-[900px]:gap-4 lg:hidden">
        <button
          type="button"
          onClick={() => nav.goToView('today')}
          className="font-serif text-[19px] font-semibold italic tracking-[-0.08em] text-[var(--accent)]"
        >
          xReader
        </button>

        <nav className="flex min-w-0 flex-1 items-center justify-center gap-1">
          {PRIMARY_NAV_ITEMS.map((item) => (
            <NavButton
              key={item.id}
              active={nav.currentView === item.id && nav.pathname === '/'}
              icon={item.icon}
              label={nav.t(NAV_LABEL_KEYS[item.id])}
              onClick={() => nav.goToView(item.id)}
            />
          ))}
        </nav>

        <div className="flex items-center gap-1">
          {nav.isAdmin ? (
            <button
              type="button"
              title={nav.t('nav.admin')}
              onClick={() => nav.router.push('/admin')}
              className={`flex h-10 w-10 items-center justify-center rounded-[10px] transition-colors ${
                nav.pathname === '/admin'
                  ? 'bg-[var(--accent-bg)] text-[var(--accent)]'
                  : 'text-[var(--text-3)] hover:bg-[var(--bg-hover)] hover:text-[var(--text-2)]'
              }`}
            >
              <ShieldCheck size={16} strokeWidth={1.75} />
            </button>
          ) : null}
          <button
            type="button"
            title={nav.t('nav.nativeLanguageTitle', { language: nav.currentLanguage.name })}
            onClick={() => setIsLanguageOpen(true)}
            className="flex h-10 items-center gap-1 rounded-[10px] px-2 text-[12px] font-semibold text-[var(--text-3)] transition-colors hover:bg-[var(--bg-hover)] hover:text-[var(--text-2)]"
          >
            <Globe size={15} strokeWidth={1.75} />
            {nav.currentLanguage.short}
          </button>
          <button
            type="button"
            title={nav.t('nav.highlights')}
            onClick={() => nav.router.push('/highlights')}
            className={`flex h-10 w-10 items-center justify-center rounded-[10px] transition-colors ${
              nav.pathname === '/highlights'
                ? 'bg-[var(--accent-bg)] text-[var(--accent)]'
                : 'text-[var(--text-3)] hover:bg-[var(--bg-hover)] hover:text-[var(--text-2)]'
            }`}
          >
            <Highlighter size={16} strokeWidth={1.75} />
          </button>
          <button
            type="button"
            title={nav.t('shortcuts.open')}
            onClick={nav.openShortcuts}
            className="flex h-10 w-10 items-center justify-center rounded-[10px] text-[var(--text-3)] transition-colors hover:bg-[var(--bg-hover)] hover:text-[var(--text-2)]"
          >
            <Keyboard size={16} strokeWidth={1.75} />
          </button>
          <button
            type="button"
            title={nav.t('nav.settings')}
            onClick={() => nav.router.push('/settings')}
            className={`flex h-10 w-10 items-center justify-center rounded-[10px] transition-colors ${
              nav.pathname === '/settings'
                ? 'bg-[var(--accent-bg)] text-[var(--accent)]'
                : 'text-[var(--text-3)] hover:bg-[var(--bg-hover)] hover:text-[var(--text-2)]'
            }`}
          >
            <Settings size={16} strokeWidth={1.75} />
          </button>
        </div>
      </header>

      {isLanguageOpen ? (
        <LanguageModal
          currentLanguage={nav.nativeLanguage}
          onSelect={nav.setNativeLanguage}
          onClose={() => setIsLanguageOpen(false)}
        />
      ) : null}
    </>
  );
}

export function MobileTopBar({ focusMode }: { focusMode: boolean }) {
  const [isMenuOpen, setIsMenuOpen] = useState(false);
  const [isLanguageOpen, setIsLanguageOpen] = useState(false);
  const nav = useAppNavigation();
  const isListPage = nav.pathname === '/';
  const menuLabel = isListPage ? nav.t(NAV_LABEL_KEYS[nav.currentView]) : nav.t('nav.more');

  if (focusMode) {
    return null;
  }

  return (
    <>
      <header className="glass-effect h-14 shrink-0 border-b border-[var(--border-light)] md:hidden">
        <div className="flex h-14 items-center justify-between px-4">
          <button
            type="button"
            onClick={() => nav.goToView('today')}
            className="font-serif text-xl font-bold italic tracking-tight text-[var(--accent)]"
          >
            xReader
          </button>
          <button
            type="button"
            onClick={() => setIsMenuOpen((value) => !value)}
            className="inline-flex min-h-11 items-center gap-1 rounded-[10px] border border-[var(--border)] bg-[var(--bg-panel)] px-3 py-1.5 text-[12px] font-semibold text-[var(--text-2)] shadow-[0_10px_30px_rgba(65,52,35,0.08)]"
            aria-expanded={isMenuOpen}
          >
            {menuLabel}
            <ChevronDown size={13} strokeWidth={1.8} />
          </button>
        </div>
      </header>

      {isMenuOpen ? (
        <>
          <button
            type="button"
            aria-label={nav.t('nav.closeMenu')}
            className="fixed inset-0 z-[120] bg-black/10 md:hidden"
            onClick={() => setIsMenuOpen(false)}
          />
          <div
            role="dialog"
            aria-label={nav.t('nav.mobileMenu')}
            className="fixed inset-x-0 bottom-0 z-[130] max-h-[82vh] overflow-y-auto rounded-t-[28px] border border-[var(--border)] bg-[var(--bg)] px-4 pb-[calc(env(safe-area-inset-bottom)+18px)] pt-4 shadow-[0_-22px_70px_rgba(0,0,0,0.18)] md:hidden"
          >
            <div className="mx-auto mb-4 h-1 w-10 rounded-full bg-[var(--border-strong)]" />

            <section>
              <div className="px-1 pb-2 text-[11px] font-semibold tracking-[0.16em] text-[var(--text-3)]">
                {nav.t('nav.viewSection')}
              </div>
              <div className="grid grid-cols-2 gap-2">
                {PRIMARY_NAV_ITEMS.map((item) => {
                  const Icon = item.icon;
                  const active = nav.currentView === item.id && isListPage;

                  return (
                    <button
                      key={item.id}
                      type="button"
                      onClick={() => {
                        nav.goToView(item.id);
                        setIsMenuOpen(false);
                      }}
                      className={`flex items-center gap-2 rounded-[10px] border px-3 py-3 text-left text-sm font-semibold transition-colors ${
                        active
                          ? 'border-[var(--accent-border)] bg-[var(--accent-soft)] text-[var(--text-accent)]'
                          : 'border-[var(--border-light)] bg-[var(--bg-panel)] text-[var(--text-2)] hover:bg-[var(--bg-hover)]'
                      }`}
                    >
                      <Icon size={17} strokeWidth={1.8} />
                      {nav.t(NAV_LABEL_KEYS[item.id])}
                    </button>
                  );
                })}
              </div>
            </section>

            <section className="mt-5">
              <div className="px-1 pb-2 text-[11px] font-semibold tracking-[0.16em] text-[var(--text-3)]">
                {nav.t('nav.toolSection')}
              </div>
              <div className="space-y-1.5">
                <button
                  type="button"
                  onClick={() => {
                    nav.router.push('/sources');
                    setIsMenuOpen(false);
                  }}
                  className="flex w-full items-center gap-3 rounded-[10px] px-3 py-3 text-left text-sm font-medium text-[var(--text-2)] hover:bg-[var(--bg-hover)]"
                >
                  <PlusCircle size={17} />
                  {nav.t('nav.manageSources')}
                </button>
                <button
                  type="button"
                  onClick={() => {
                    nav.router.push('/highlights');
                    setIsMenuOpen(false);
                  }}
                  className="flex w-full items-center gap-3 rounded-[10px] px-3 py-3 text-left text-sm font-medium text-[var(--text-2)] hover:bg-[var(--bg-hover)]"
                >
                  <Highlighter size={17} />
                  {nav.t('nav.highlights')}
                </button>
                <button
                  type="button"
                  onClick={() => {
                    nav.openShortcuts();
                    setIsMenuOpen(false);
                  }}
                  className="flex w-full items-center gap-3 rounded-[10px] px-3 py-3 text-left text-sm font-medium text-[var(--text-2)] hover:bg-[var(--bg-hover)]"
                >
                  <Keyboard size={17} />
                  {nav.t('shortcuts.title')}
                </button>
                <button
                  type="button"
                  onClick={() => {
                    setIsLanguageOpen(true);
                    setIsMenuOpen(false);
                  }}
                  className="flex w-full items-center justify-between gap-3 rounded-[10px] px-3 py-3 text-left text-sm font-medium text-[var(--text-2)] hover:bg-[var(--bg-hover)]"
                >
                  <span className="inline-flex items-center gap-3">
                    <Globe size={17} />
                    {nav.t('nav.nativeLanguage')}
                  </span>
                  <span className="text-[11px] font-semibold text-[var(--accent)]">{nav.currentLanguage.short}</span>
                </button>
                <button
                  type="button"
                  onClick={() => {
                    nav.router.push('/settings');
                    setIsMenuOpen(false);
                  }}
                  className="flex w-full items-center gap-3 rounded-[10px] px-3 py-3 text-left text-sm font-medium text-[var(--text-2)] hover:bg-[var(--bg-hover)]"
                >
                  <Settings size={17} />
                  {nav.t('nav.settings')}
                </button>
                {nav.isAdmin ? (
                  <button
                    type="button"
                    onClick={() => {
                      nav.router.push('/admin');
                      setIsMenuOpen(false);
                    }}
                    className="flex w-full items-center gap-3 rounded-[10px] px-3 py-3 text-left text-sm font-medium text-[var(--text-2)] hover:bg-[var(--bg-hover)]"
                  >
                    <ShieldCheck size={17} />
                    {nav.t('nav.admin')}
                  </button>
                ) : null}
              </div>
            </section>

            <button
              type="button"
              onClick={() => {
                void nav.handleLogout();
                setIsMenuOpen(false);
              }}
              className="mt-4 flex w-full items-center gap-3 rounded-[10px] border border-[var(--border-light)] px-3 py-3 text-left text-sm font-medium text-[var(--text-3)] hover:bg-[var(--bg-hover)]"
            >
              <LogOut size={17} />
              {nav.t('nav.logOut')}
            </button>
          </div>
        </>
      ) : null}

      {isLanguageOpen ? (
        <LanguageModal
          currentLanguage={nav.nativeLanguage}
          onSelect={nav.setNativeLanguage}
          onClose={() => setIsLanguageOpen(false)}
        />
      ) : null}
    </>
  );
}
