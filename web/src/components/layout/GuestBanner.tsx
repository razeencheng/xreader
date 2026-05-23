'use client';

import { useState } from 'react';
import { useIsGuest } from '@/stores/useAuthStore';
import { useI18n } from '@/lib/i18n';

export function GuestBanner() {
  const isGuest = useIsGuest();
  const { t } = useI18n();
  const [dismissed, setDismissed] = useState(() => {
    if (typeof window === 'undefined') return false;
    return localStorage.getItem('guest_banner_dismissed') === 'true';
  });

  if (!isGuest || dismissed) return null;

  return (
    <div className="flex items-center justify-between gap-2 bg-[var(--bg-elevated)] px-4 py-2 text-xs text-[var(--text-muted)] border-b border-[var(--border)]">
      <span>
        {t('guest.banner')}{' · '}
        <a href="/login" className="underline hover:text-[var(--text-body)]">
          {t('guest.signIn')}
        </a>
      </span>
      <button
        onClick={() => {
          setDismissed(true);
          localStorage.setItem('guest_banner_dismissed', 'true');
        }}
        className="text-[var(--text-muted)] hover:text-[var(--text-body)]"
        aria-label="Dismiss"
      >
        ✕
      </button>
    </div>
  );
}
