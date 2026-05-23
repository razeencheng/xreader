import { CalendarDays, List, RadioTower, Star, type LucideIcon } from 'lucide-react';
import type { ViewTab } from '@/stores/useUIStore';

export const PRIMARY_NAV_ITEMS: {
  id: ViewTab;
  icon: LucideIcon;
  title: string;
  label: string;
  shortLabel: string;
}[] = [
  { id: 'today', icon: CalendarDays, title: 'Today', label: 'Today', shortLabel: '今日' },
  { id: 'all', icon: List, title: 'All Articles', label: 'All', shortLabel: '全部' },
  { id: 'starred', icon: Star, title: 'Starred', label: 'Starred', shortLabel: '收藏' },
  { id: 'sources', icon: RadioTower, title: 'Sources', label: 'Sources', shortLabel: '源' },
];

export const LANGUAGE_OPTIONS = [
  { code: 'zh-CN', label: '中文', name: 'Chinese', short: 'ZH' },
  { code: 'zh-TW', label: '繁中', name: 'Traditional Chinese', short: 'ZH' },
  { code: 'en-US', label: 'EN', name: 'English', short: 'EN' },
  { code: 'ja-JP', label: '日本語', name: 'Japanese', short: 'JA' },
  { code: 'es-ES', label: 'ES', name: 'Spanish', short: 'ES' },
  { code: 'fr-FR', label: 'FR', name: 'French', short: 'FR' },
  { code: 'de-DE', label: 'DE', name: 'German', short: 'DE' },
  { code: 'ko-KR', label: '한국어', name: 'Korean', short: 'KO' },
  { code: 'pt-PT', label: 'PT', name: 'Portuguese', short: 'PT' },
] as const;

function normalizeOptionCode(code: string) {
  const normalized = code.toLowerCase();
  if (normalized === 'zh-tw' || normalized === 'zh-hk' || normalized === 'zh-mo') return 'zh-TW';
  if (normalized.startsWith('zh')) return 'zh-CN';
  if (normalized.startsWith('ja')) return 'ja';
  if (normalized.startsWith('ko')) return 'ko';
  if (normalized.startsWith('es')) return 'es';
  if (normalized.startsWith('fr')) return 'fr';
  if (normalized.startsWith('de')) return 'de';
  if (normalized.startsWith('pt')) return 'pt';
  return 'en';
}

export function getLanguageOption(language: string) {
  return (
    LANGUAGE_OPTIONS.find((option) => option.code === language) ??
    LANGUAGE_OPTIONS.find((option) => normalizeOptionCode(option.code) === normalizeOptionCode(language)) ??
    LANGUAGE_OPTIONS[0]
  );
}

export function isLanguageOptionActive(currentLanguage: string, optionCode: string) {
  const hasExactMatch = LANGUAGE_OPTIONS.some((option) => option.code === currentLanguage);
  if (hasExactMatch) {
    return currentLanguage === optionCode;
  }

  return normalizeOptionCode(currentLanguage) === normalizeOptionCode(optionCode);
}
