'use client';

import { useEffect, useSyncExternalStore } from 'react';
import { useUIStore } from '@/stores/useUIStore';

function subscribeToMediaQuery(callback: () => void) {
  const mq = window.matchMedia('(prefers-color-scheme: dark)');
  mq.addEventListener('change', callback);
  return () => mq.removeEventListener('change', callback);
}

function getSystemThemeSnapshot(): 'light' | 'dark' {
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
}

function getServerSnapshot(): 'light' | 'dark' {
  return 'light';
}

export function ThemeProvider({ children }: { children: React.ReactNode }) {
  const theme = useUIStore((s) => s.theme);
  const accent = useUIStore((s) => s.accentColor);
  const fontSize = useUIStore((s) => s.fontSize);

  const systemTheme = useSyncExternalStore(
    subscribeToMediaQuery,
    getSystemThemeSnapshot,
    getServerSnapshot,
  );

  useEffect(() => {
    const el = document.documentElement;
    el.classList.remove('theme-light', 'theme-dark');
    el.setAttribute('data-accent', accent);
    el.style.setProperty('--font-ui-size', `${fontSize}px`);

    const resolvedTheme = theme === 'system' ? systemTheme : theme;

    if (resolvedTheme === 'light') {
      el.classList.add('theme-light');
    } else {
      el.classList.add('theme-dark');
    }
  }, [theme, accent, fontSize, systemTheme]);

  return <>{children}</>;
}
