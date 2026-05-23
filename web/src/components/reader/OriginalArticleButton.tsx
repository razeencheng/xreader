'use client';

import { useI18n } from '@/lib/i18n';

interface Props {
  href: string;
  onOpen?: () => void;
}

export function OriginalArticleButton({ href, onOpen }: Props) {
  const { t } = useI18n();

  const handleOpen = () => {
    if (onOpen) {
      onOpen();
      return;
    }
    window.open(href, '_blank', 'noopener,noreferrer');
  };

  return (
    <button
      type="button"
      onClick={handleOpen}
      className="inline-flex min-h-11 items-center rounded-[5px] text-[12.5px] font-medium text-[var(--accent)] transition-colors hover:text-[var(--accent-strong)] focus:outline-none focus:ring-2 focus:ring-[var(--accent)]/25 md:min-h-0"
    >
      {t('reader.readOriginal')}
    </button>
  );
}
