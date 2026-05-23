'use client';

import { Keyboard, X } from 'lucide-react';
import { useI18n } from '@/lib/i18n';
import { useFocusTrap } from '@/hooks/useFocusTrap';

const SHORTCUT_GROUPS = [
  {
    labelKey: 'shortcuts.navigation',
    items: [
      ['l', 'shortcuts.nextArticle'],
      ['h', 'shortcuts.previousArticle'],
    ],
  },
  {
    labelKey: 'shortcuts.article',
    items: [
      ['j', 'shortcuts.scrollDown'],
      ['k', 'shortcuts.scrollUp'],
      ['s', 'shortcuts.starArticle'],
      ['r', 'shortcuts.markRead'],
    ],
  },
  {
    labelKey: 'shortcuts.view',
    items: [
      ['f', 'shortcuts.toggleFocus'],
      ['?', 'shortcuts.showShortcuts'],
      ['Esc', 'shortcuts.close'],
    ],
  },
] as const;

function Kbd({ children }: { children: React.ReactNode }) {
  return (
    <kbd className="inline-flex min-w-8 items-center justify-center rounded-[8px] border border-[var(--border)] bg-[var(--bg-panel)] px-2 py-[3px] text-[11px] font-semibold text-[var(--text-2)] shadow-[inset_0_-1px_0_rgba(65,52,35,0.05)]">
      {children}
    </kbd>
  );
}

export function KeyboardShortcutsButton({
  onClick,
  className = '',
}: {
  onClick: () => void;
  className?: string;
}) {
  const { t } = useI18n();

  return (
    <button
      type="button"
      onClick={onClick}
      title={`${t('shortcuts.open')} (?)`}
      aria-label={t('shortcuts.openAria')}
      className={`fixed bottom-[max(16px,calc(env(safe-area-inset-bottom)+8px))] left-4 z-[90] inline-flex items-center gap-1.5 rounded-[10px] border border-[var(--border)] bg-[color-mix(in_srgb,var(--bg-body)_92%,transparent)] px-3 py-2 text-[11px] text-[var(--text-3)] shadow-[0_16px_40px_rgba(65,52,35,0.12)] backdrop-blur transition-colors hover:bg-[var(--bg)] hover:text-[var(--text-2)] md:left-[68px] ${className}`}
    >
      <Keyboard size={13} />
      <span className="font-medium">{t('shortcuts.open')}</span>
      <Kbd>?</Kbd>
    </button>
  );
}

export function KeyboardShortcutsModal({
  open,
  onClose,
}: {
  open: boolean;
  onClose: () => void;
}) {
  const { t } = useI18n();
  const trapRef = useFocusTrap<HTMLDivElement>({ onEscape: onClose });

  if (!open) {
    return null;
  }

  return (
    <div
      className="fixed inset-0 z-[140] flex items-center justify-center bg-black/35 px-4 backdrop-blur-[2px]"
      onClick={onClose}
    >
      <div
        ref={trapRef}
        role="dialog"
        aria-modal="true"
        aria-label={t('shortcuts.title')}
        className="w-full max-w-[380px] rounded-2xl border border-[var(--border)] bg-[var(--bg)] p-6 shadow-[0_24px_64px_rgba(0,0,0,0.25)]"
        onClick={(event) => event.stopPropagation()}
      >
        <div className="mb-5 flex items-center justify-between">
          <div>
            <h2 className="text-[15px] font-semibold text-[var(--text)]">{t('shortcuts.title')}</h2>
            <p className="mt-1 text-[12px] text-[var(--text-3)]">{t('shortcuts.subtitle')}</p>
          </div>
          <button
            type="button"
            onClick={onClose}
            aria-label={t('shortcuts.closeAria')}
            className="inline-flex min-h-11 min-w-11 items-center justify-center rounded-[9px] p-2 text-[var(--text-3)] transition-colors hover:bg-[var(--bg-hover)] hover:text-[var(--text-2)]"
          >
            <X size={16} />
          </button>
        </div>

        <div className="space-y-4">
          {SHORTCUT_GROUPS.map((group) => (
            <section key={group.labelKey}>
              <div className="mb-2 text-[10.5px] font-medium uppercase tracking-[0.08em] text-[var(--text-3)]">
                {t(group.labelKey)}
              </div>
              <div className="overflow-hidden rounded-[12px] border border-[var(--border-light)]">
                {group.items.map(([key, labelKey], index) => (
                  <div
                    key={key}
                    className={`flex items-center justify-between gap-4 bg-[var(--bg-panel)] px-3 py-2.5 ${
                      index > 0 ? 'border-t border-[var(--border-light)]' : ''
                    }`}
                  >
                    <span className="text-[13px] text-[var(--text-2)]">{t(labelKey)}</span>
                    <Kbd>{key.toUpperCase()}</Kbd>
                  </div>
                ))}
              </div>
            </section>
          ))}
        </div>
      </div>
    </div>
  );
}
