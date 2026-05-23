import { useI18n } from '@/lib/i18n';
import type { ReadFilter } from '@/stores/useUIStore';

const READ_FILTERS: Array<{ id: ReadFilter; labelKey: string }> = [
  { id: 'unread', labelKey: 'feed.unread' },
  { id: 'all', labelKey: 'feed.all' },
  { id: 'read', labelKey: 'feed.read' },
];

function compactCount(n: number): string {
  if (n < 1000) return String(n);
  if (n < 10000) return `${(n / 1000).toFixed(1).replace(/\.0$/, '')}k`;
  return `${Math.round(n / 1000)}k`;
}

interface ReadFilterSegmentedControlProps {
  value: ReadFilter;
  counts: Record<ReadFilter, number>;
  onChange: (value: ReadFilter) => void;
  fullWidth?: boolean;
}

export function ReadFilterSegmentedControl({ value, counts, onChange, fullWidth }: ReadFilterSegmentedControlProps) {
  const { t } = useI18n();
  const label = `${t('feed.unread')} / ${t('feed.all')} / ${t('feed.read')}`;

  return (
    <div
      role="group"
      aria-label={label}
      className={`inline-flex min-w-0 items-center rounded-[10px] bg-[var(--bg-panel)] p-[3px] text-[13px] leading-none ring-1 ring-inset ring-[var(--border-light)] ${fullWidth ? 'w-full md:w-auto' : ''}`}
    >
      {READ_FILTERS.map(({ id, labelKey }) => {
        const active = value === id;

        return (
          <button
            key={id}
            type="button"
            aria-pressed={active}
            onClick={() => onChange(id)}
            className={`inline-flex min-h-11 items-center justify-center gap-[5px] whitespace-nowrap rounded-[8px] px-2.5 font-semibold transition-[background,color,box-shadow] duration-150 ${fullWidth ? 'flex-1 md:flex-initial' : ''} ${
              active
                ? 'read-filter-segment-active bg-[var(--bg-elevated)] text-[var(--text)] shadow-[0_1px_2px_rgba(30,24,16,0.10),0_5px_14px_rgba(30,24,16,0.06)]'
                : 'text-[var(--text-3)] hover:text-[var(--text-2)]'
            }`}
          >
            <span>{t(labelKey)}</span>
            <span className={active ? 'text-[var(--text-3)]' : 'text-[var(--text-faint)]'}>{compactCount(counts[id])}</span>
          </button>
        );
      })}
    </div>
  );
}
