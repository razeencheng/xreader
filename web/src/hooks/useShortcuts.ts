'use client';

import { useEffect } from 'react';
import { registerShortcut, unregisterShortcut } from '@/lib/keyboard';

export function useShortcuts(shortcuts: Record<string, () => void>) {
  useEffect(() => {
    const entries = Object.entries(shortcuts);

    for (const [key, handler] of entries) {
      registerShortcut(key, handler);
    }

    return () => {
      for (const [key] of entries) {
        unregisterShortcut(key);
      }
    };
  }, [shortcuts]);
}
