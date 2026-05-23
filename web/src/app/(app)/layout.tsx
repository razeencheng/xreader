'use client';

import { useEffect } from 'react';
import { useRouter } from 'next/navigation';
import { motion } from 'framer-motion';
import { useBroadcastSync } from '@/hooks/useBroadcastSync';
import { useCrossDevicePoll } from '@/hooks/useCrossDevicePoll';
import { useGlobalShortcuts } from '@/hooks/useGlobalShortcuts';
import { useI18n } from '@/lib/i18n';
import { useAuthStore } from '@/stores/useAuthStore';
import { useUIStore } from '@/stores/useUIStore';

import { GuestBanner } from '@/components/layout/GuestBanner';
import { KeyboardShortcutsModal } from '@/components/layout/KeyboardShortcutsModal';
import { Sidebar } from '@/components/layout/Sidebar';
import { MobileTopBar, TabletTopNav } from '@/components/layout/ResponsiveAppNav';
import { SourceImportStatus } from '@/components/layout/SourceImportStatus';

export default function AppLayout({ children }: { children: React.ReactNode }) {
  const { t } = useI18n();
  const router = useRouter();
  const { user, isLoading, fetchMe } = useAuthStore();
  const hydrate = useUIStore((state) => state.hydrate);
  const hydrateFromLocalStorage = useUIStore((state) => state.hydrateFromLocalStorage);
  const focusMode = useUIStore((state) => state.focusMode);
  const isShortcutsOpen = useUIStore((state) => state.isShortcutsOpen);
  const closeShortcuts = useUIStore((state) => state.closeShortcuts);

  useGlobalShortcuts();

  // Hydrate UI state from localStorage on mount (avoids SSR mismatch)
  useEffect(() => {
    hydrateFromLocalStorage();
  }, [hydrateFromLocalStorage]);

  useEffect(() => {
    void fetchMe();
  }, [fetchMe]);

  useEffect(() => {
    if (!user) return;
    hydrate({
      density_pref: user.density_pref,
      theme_pref: user.theme_pref,
      native_language: user.native_language,
    });
  }, [user, hydrate]);

  useEffect(() => {
    if (!isLoading && !user) {
      router.replace('/login');
    }
  }, [isLoading, router, user]);

  useCrossDevicePoll(!!user);
  useBroadcastSync();

  if (isLoading || !user) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-[var(--bg-body)]">
        <div className="flex flex-col items-center gap-4">
          <div className="h-8 w-8 animate-spin rounded-full border-2 border-[var(--accent)] border-t-transparent" />
          <p className="text-sm font-medium tracking-wider text-[var(--text-muted)]">{t('common.loading')}</p>
        </div>
      </div>
    );
  }

  return (
    <div className="flex h-dvh flex-col overflow-hidden bg-[var(--bg-body)] text-[var(--text-body)]">
      <GuestBanner />
      <TabletTopNav focusMode={focusMode} />
      <MobileTopBar focusMode={focusMode} />

      <div className="flex min-h-0 flex-1 overflow-hidden">
        <motion.div
          animate={{ width: focusMode ? 0 : 52, opacity: focusMode ? 0 : 1 }}
          transition={{ duration: 0.28, ease: [0.32, 0.72, 0, 1] }}
          className="hidden shrink-0 overflow-hidden lg:flex"
          style={{ pointerEvents: focusMode ? 'none' : 'auto' }}
        >
          <Sidebar className="shrink-0" />
        </motion.div>

        <div className="min-w-0 flex-1 overflow-hidden">
          <main className="h-full overflow-hidden">{children}</main>
        </div>
      </div>

      <KeyboardShortcutsModal open={isShortcutsOpen} onClose={closeShortcuts} />
      <SourceImportStatus />
    </div>
  );
}
