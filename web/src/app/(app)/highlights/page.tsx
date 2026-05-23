'use client';

import { Suspense } from 'react';
import { HighlightsList } from '@/components/highlights/HighlightsList';
import { useI18n } from '@/lib/i18n';

export default function HighlightsPage() {
  const { t } = useI18n();

  return (
    <div className="h-full overflow-y-auto px-4 py-6 pb-[env(safe-area-inset-bottom)] text-[var(--text-body)]">
      <div className="mx-auto max-w-3xl">
        <h1 className="mb-6 text-xl font-semibold text-[var(--text-body)]">{t('highlights.title')}</h1>
        <Suspense fallback={<div className="py-8 text-center text-sm text-[var(--text-muted)]">{t('common.loading')}</div>}>
          <HighlightsList />
        </Suspense>
      </div>
    </div>
  );
}
