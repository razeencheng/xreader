'use client';

import { useI18n } from '@/lib/i18n';
import type { ReaderGestureHint as ReaderGestureHintType } from '@/hooks/useReaderGestures';

const HINT_KEYS: Record<Exclude<ReaderGestureHintType, null>, string> = {
  next: 'reader.gestureNext',
  previous: 'reader.gesturePrevious',
  back: 'reader.gestureBack',
  nextUnavailable: 'reader.gestureNoNext',
  previousUnavailable: 'reader.gestureNoPrevious',
};

export function ReaderGestureHint({ hint }: { hint: ReaderGestureHintType }) {
  const { t } = useI18n();

  if (!hint) return null;

  return (
    <div className="pointer-events-none absolute inset-x-0 top-[76px] z-[90] flex justify-center px-5 md:hidden">
      <div className="rounded-[10px] border border-[var(--accent-border)] bg-[var(--accent-soft)] px-4 py-2 font-[system-ui] text-xs font-semibold text-[var(--text-accent)] shadow-[0_14px_40px_rgba(65,52,35,0.14)]">
        {t(HINT_KEYS[hint])}
      </div>
    </div>
  );
}
