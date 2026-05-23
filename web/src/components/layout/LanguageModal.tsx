'use client';

import { isLanguageOptionActive, LANGUAGE_OPTIONS } from '@/components/layout/navigationConfig';
import { useI18n } from '@/lib/i18n';
import { useFocusTrap } from '@/hooks/useFocusTrap';

export function LanguageModal({
  currentLanguage,
  onSelect,
  onClose,
}: {
  currentLanguage: string;
  onSelect: (language: string) => void;
  onClose: () => void;
}) {
  const { t } = useI18n();
  const trapRef = useFocusTrap<HTMLDivElement>({ onEscape: onClose });

  return (
    <div
      className="fixed inset-0 z-[120] flex items-center justify-center bg-black/30 px-4 backdrop-blur-[3px]"
      onClick={onClose}
    >
      <div
        ref={trapRef}
        role="dialog"
        aria-modal="true"
        aria-label={t('language.title')}
        className="w-full max-w-[320px] rounded-2xl border border-[var(--border)] bg-[var(--bg)] p-6 shadow-[0_20px_60px_rgba(0,0,0,0.18)]"
        onClick={(event) => event.stopPropagation()}
      >
        <h2 className="text-[14.5px] font-semibold text-[var(--text)]">{t('language.title')}</h2>
        <p className="mt-2 text-[12.5px] leading-5 text-[var(--text-3)]">
          {t('language.description')}
        </p>

        <div className="mt-4 grid grid-cols-1 gap-2 sm:grid-cols-2">
          {LANGUAGE_OPTIONS.map((language) => {
            const active = isLanguageOptionActive(currentLanguage, language.code);

            return (
              <button
                key={language.code}
                type="button"
                onClick={() => {
                  onSelect(language.code);
                  onClose();
                }}
                className={`flex min-h-11 items-center justify-between rounded-[9px] border px-3 py-[9px] text-[13px] transition-colors ${
                  active
                    ? 'border-[var(--accent)] bg-[var(--accent-bg)] text-[var(--accent)]'
                    : 'border-[var(--border)] bg-transparent text-[var(--text)] hover:bg-[var(--bg-hover)]'
                }`}
              >
                <span>
                  {language.label}{' '}
                  <span className="font-normal text-[var(--text-3)]">· {language.name}</span>
                </span>
                {active ? <span className="text-[12px]">✓</span> : null}
              </button>
            );
          })}
        </div>
      </div>
    </div>
  );
}
