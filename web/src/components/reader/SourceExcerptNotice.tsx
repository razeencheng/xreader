import { useI18n } from '@/lib/i18n';

interface SourceExcerptNoticeProps {
  error?: string | null;
  isLoading?: boolean;
  onLoadOriginal: () => void;
}

export function SourceExcerptNotice({ error, isLoading = false, onLoadOriginal }: SourceExcerptNoticeProps) {
  const { t } = useI18n();

  return (
    <aside className="mb-7 rounded-2xl border border-[var(--border-strong)] bg-[var(--bg-callout)] px-5 py-4 font-[system-ui] text-sm leading-6 text-[var(--text-secondary)]">
      <div className="font-medium text-[var(--text-body)]">{t('reader.summaryOnlyTitle')}</div>
      <div className="mt-1 text-[13px]">
        {t('reader.summaryOnlyDescription')}
      </div>
      <button
        type="button"
        onClick={onLoadOriginal}
        disabled={isLoading}
        className="mt-3 w-full rounded-[10px] bg-[var(--accent)] py-2.5 text-[13px] font-medium text-white transition-opacity hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-60"
      >
        {isLoading ? (
          <span className="inline-flex items-center gap-1.5">
            <span className="h-1.5 w-1.5 animate-pulse rounded-full bg-current" />
            {t('reader.loading')}
          </span>
        ) : (
          t('reader.loadOriginal')
        )}
      </button>
      {error ? <div className="mt-3 text-xs text-[var(--text-error)]">{error}</div> : null}
    </aside>
  );
}
