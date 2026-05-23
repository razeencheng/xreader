'use client';

import { useEffect, useState } from 'react';
import { useI18n } from '@/lib/i18n';

export default function LoginPage() {
  const { t } = useI18n();
  const [ready, setReady] = useState(false);

  useEffect(() => {
    fetch('/api/setup/status')
      .then((res) => res.json())
      .then((data) => {
        if (data.needs_setup) {
          window.location.href = '/setup';
        } else {
          setReady(true);
        }
      })
      .catch(() => setReady(true));
  }, []);

  if (!ready) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <div className="h-6 w-6 animate-spin rounded-full border-2 border-[var(--accent)] border-t-transparent" />
      </div>
    );
  }

  return (
    <div className="flex min-h-screen items-center justify-center">
      <div className="text-center">
        <h1 className="mb-8 text-2xl font-semibold text-[var(--text-body)]">xReader</h1>
        <a
          href="/api/auth/github"
          className="inline-flex items-center justify-center rounded-lg bg-[oklch(22%_0.01_260)] px-6 py-3 text-sm font-semibold text-white shadow-[0_12px_30px_rgba(15,23,42,0.16)] transition-colors hover:bg-[oklch(28%_0.012_260)] focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[var(--accent)]"
          style={{ color: '#fff' }}
        >
          {t('auth.signInWithGitHub')}
        </a>
      </div>
    </div>
  );
}
