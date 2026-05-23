'use client';

import { useEffect, useRef, useState } from 'react';
import { AnimatePresence, motion } from 'framer-motion';
import { useI18n } from '@/lib/i18n';
import { useUIStore, type AccentColor, type Density, type Layout, type Theme } from '@/stores/useUIStore';
import { applyReaderLayoutSelection, getActiveReaderLayout } from '@/lib/reader-layout';

const CHIP_BASE = 'rounded-md px-[11px] py-1 text-[12px]';

const FONT_SIZES = [14, 16, 17, 19, 21] as const;
const THEME_OPTIONS: Array<{ id: Theme; labelKey: string }> = [
  { id: 'light', labelKey: 'settings.themeLight' },
  { id: 'dark', labelKey: 'settings.themeDark' },
  { id: 'system', labelKey: 'settings.themeSystem' },
];
const ACCENTS: Array<{ id: AccentColor; color: string }> = [
  { id: 'blue', color: 'oklch(50% 0.16 255)' },
  { id: 'sage', color: 'oklch(52% 0.12 160)' },
  { id: 'ember', color: 'oklch(52% 0.15 32)' },
  { id: 'rose', color: 'oklch(52% 0.16 5)' },
];

function Section({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <section className="mb-[14px] last:mb-0">
      <div className="mb-[7px] text-[10px] font-medium uppercase tracking-[0.07em] text-[var(--text-3)]">
        {label}
      </div>
      {children}
    </section>
  );
}

function Chip<T extends string | number>({
  label,
  value,
  active,
  onSelect,
}: {
  label: string;
  value: T;
  active: boolean;
  onSelect: (value: T) => void;
}) {
  return (
    <button
      type="button"
      onClick={() => onSelect(value)}
      className={`${active ? 'ui-pill-active' : 'ui-pill-neutral'} ${CHIP_BASE}`}
    >
      {label}
    </button>
  );
}

interface TweaksPanelProps {
  externalOpen?: boolean;
  onExternalClose?: () => void;
}

export function TweaksPanel({ externalOpen, onExternalClose }: TweaksPanelProps = {}) {
  const { t } = useI18n();
  const [internalOpen, setInternalOpen] = useState(false);
  const isOpen = externalOpen || internalOpen;
  const panelRef = useRef<HTMLDivElement>(null);
  const accentColor = useUIStore((state) => state.accentColor);
  const setAccentColor = useUIStore((state) => state.setAccentColor);
  const fontSize = useUIStore((state) => state.fontSize);
  const setFontSize = useUIStore((state) => state.setFontSize);
  const density = useUIStore((state) => state.density);
  const setDensity = useUIStore((state) => state.setDensity);
  const theme = useUIStore((state) => state.theme);
  const setTheme = useUIStore((state) => state.setTheme);
  const layout = useUIStore((state) => state.layout);
  const setLayout = useUIStore((state) => state.setLayout);
  const focusMode = useUIStore((state) => state.focusMode);
  const setFocusMode = useUIStore((state) => state.setFocusMode);

  const closePanel = () => {
    setInternalOpen(false);
    onExternalClose?.();
  };

  useEffect(() => {
    if (!isOpen) return;
    const handleClick = (event: MouseEvent) => {
      if (panelRef.current && !panelRef.current.contains(event.target as Node)) {
        closePanel();
      }
    };
    document.addEventListener('mousedown', handleClick);
    return () => document.removeEventListener('mousedown', handleClick);
  }, [isOpen]);

  const activeLayout = getActiveReaderLayout(layout, focusMode);

  const handleSelectLayout = (value: Layout) => {
    applyReaderLayoutSelection(value, setLayout, setFocusMode);
  };

  if (!isOpen) return null;

  return (
    <div ref={panelRef} className="absolute bottom-5 right-5 z-[100]">
      <AnimatePresence>
        <motion.div
          initial={{ opacity: 0, y: 12, scale: 0.96 }}
          animate={{ opacity: 1, y: 0, scale: 1 }}
          exit={{ opacity: 0, y: 12, scale: 0.96 }}
          transition={{ duration: 0.16, ease: 'easeOut' }}
          className="min-w-[240px] rounded-[14px] border border-[var(--border)] bg-[var(--bg)] px-5 py-[18px] shadow-[0_8px_32px_rgba(0,0,0,0.14)]"
        >
          <div className="mb-4 text-[13.5px] font-semibold text-[var(--text)]">{t('tweaks.title')}</div>

          <Section label={t('tweaks.layout')}>
            <div className="flex flex-wrap gap-[6px]">
              <Chip label={t('tweaks.layoutClassic')} value="classic" active={activeLayout === 'classic'} onSelect={handleSelectLayout} />
              <Chip label={t('tweaks.layoutFocus')} value="focus" active={activeLayout === 'focus'} onSelect={handleSelectLayout} />
              <Chip label={t('tweaks.layoutWide')} value="wide" active={activeLayout === 'wide'} onSelect={handleSelectLayout} />
            </div>
          </Section>

          <Section label={t('tweaks.density')}>
            <div className="flex flex-wrap gap-[6px]">
              {(['comfortable', 'compact'] as const).map((value) => (
                <Chip
                  key={value}
                  label={value === 'comfortable' ? t('settings.densityComfortable') : t('settings.densityCompact')}
                  value={value satisfies Density}
                  active={density === value}
                  onSelect={setDensity}
                />
              ))}
            </div>
          </Section>

          <Section label={t('tweaks.fontSize')}>
            <div className="flex flex-wrap gap-[6px]">
              {FONT_SIZES.map((value) => (
                <Chip key={value} label={`${value}`} value={value} active={fontSize === value} onSelect={setFontSize} />
              ))}
            </div>
          </Section>

          <Section label={t('tweaks.theme')}>
            <div className="flex flex-wrap gap-[6px]">
              {THEME_OPTIONS.map((option) => (
                <Chip
                  key={option.id}
                  label={t(option.labelKey)}
                  value={option.id}
                  active={theme === option.id}
                  onSelect={setTheme}
                />
              ))}
            </div>
          </Section>

          <Section label={t('tweaks.accent')}>
            <div className="flex items-center gap-[9px]">
              {ACCENTS.map((accent) => {
                const active = accent.id === accentColor;

                return (
                  <button
                    key={accent.id}
                    type="button"
                    aria-label={t(`tweaks.accent${accent.id[0].toUpperCase()}${accent.id.slice(1)}`)}
                    onClick={() => setAccentColor(accent.id)}
                    className={`flex h-[24px] w-[24px] items-center justify-center rounded-full border transition-transform hover:scale-105 ${
                      active
                        ? 'border-[var(--accent-border)] bg-[var(--accent-soft)]'
                        : 'border-[var(--border)] bg-[var(--bg-elevated)] hover:bg-[var(--bg-hover)]'
                    }`}
                  >
                    <span
                      className="block h-[14px] w-[14px] rounded-full"
                      style={{
                        background: accent.color,
                        boxShadow: active ? '0 0 0 2px var(--accent-ring)' : 'none',
                      }}
                    />
                  </button>
                );
              })}
            </div>
          </Section>
        </motion.div>
      </AnimatePresence>
    </div>
  );
}
