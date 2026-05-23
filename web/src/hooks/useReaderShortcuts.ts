'use client';

import { useMemo } from 'react';
import { useShortcuts } from '@/hooks/useShortcuts';
import { useUIStore } from '@/stores/useUIStore';

interface ReaderShortcutHandlers {
  onNext?: () => void;
  onPrev?: () => void;
  onToggleStar?: () => void;
  onMarkRead?: () => void;
  onToggleFocus?: () => void;
}

export function useReaderShortcuts({
  onNext,
  onPrev,
  onToggleStar,
  onMarkRead,
  onToggleFocus,
}: ReaderShortcutHandlers) {
  const isShortcutsOpen = useUIStore((state) => state.isShortcutsOpen);
  const openShortcuts = useUIStore((state) => state.openShortcuts);
  const closeShortcuts = useUIStore((state) => state.closeShortcuts);

  const shortcuts = useMemo<Record<string, () => void>>(() => {
    if (isShortcutsOpen) {
      return {};
    }

    const nextShortcuts: Record<string, () => void> = {};
    nextShortcuts.l = () => onNext?.();
    nextShortcuts.h = () => onPrev?.();
    nextShortcuts.arrowright = () => onNext?.();
    nextShortcuts.arrowleft = () => onPrev?.();
    nextShortcuts.s = () => onToggleStar?.();
    nextShortcuts.r = () => onMarkRead?.();
    nextShortcuts.f = () => onToggleFocus?.();

    return nextShortcuts;
  }, [isShortcutsOpen, onMarkRead, onNext, onPrev, onToggleFocus, onToggleStar]);

  useShortcuts(shortcuts);

  return {
    isShortcutsOpen,
    openShortcuts,
    closeShortcuts,
  };
}
