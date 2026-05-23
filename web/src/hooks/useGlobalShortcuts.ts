'use client';

import { useMemo } from 'react';
import { useShortcuts } from '@/hooks/useShortcuts';
import { useUIStore } from '@/stores/useUIStore';

export function useGlobalShortcuts() {
  const isShortcutsOpen = useUIStore((state) => state.isShortcutsOpen);
  const openShortcuts = useUIStore((state) => state.openShortcuts);
  const closeShortcuts = useUIStore((state) => state.closeShortcuts);
  const focusMode = useUIStore((state) => state.focusMode);
  const setFocusMode = useUIStore((state) => state.setFocusMode);

  const shortcuts = useMemo<Record<string, () => void>>(
    () => ({
      '?': openShortcuts,
      escape: () => {
        if (isShortcutsOpen) {
          closeShortcuts();
          return;
        }

        if (focusMode) {
          setFocusMode(false);
        }
      },
    }),
    [closeShortcuts, focusMode, isShortcutsOpen, openShortcuts, setFocusMode],
  );

  useShortcuts(shortcuts);
}
