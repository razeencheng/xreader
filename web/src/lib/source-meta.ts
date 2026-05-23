import type { Source } from '@/lib/types';

const GROUP_ORDER = ['Technology', 'Finance', 'Infrastructure'] as const;
const FALLBACK_COLORS = ['#4f6bed', '#f48122', '#e03030', '#198754', '#8f63d7', '#9b6b42'];
const SOURCE_MATCHERS: Array<{ pattern: RegExp; color: string }> = [
  { pattern: /hacker\s*news|\bhn\b/i, color: '#f66026' },
  { pattern: /bloomberg|\bbb\b/i, color: '#1355f5' },
  { pattern: /the\s*verge|\bverge\b|\bvg\b/i, color: '#e03030' },
  { pattern: /ars\s*technica|\bat\b/i, color: '#d4481a' },
  { pattern: /wired|\bwd\b/i, color: '#555555' },
  { pattern: /cloudflare|\bcf\b/i, color: '#f48122' },
  { pattern: /vercel/i, color: '#111111' },
  { pattern: /v2ex/i, color: '#1f5feb' },
];

function hashText(value: string) {
  let hash = 0;
  for (let index = 0; index < value.length; index += 1) {
    hash = (hash << 5) - hash + value.charCodeAt(index);
    hash |= 0;
  }
  return Math.abs(hash);
}

export function normalizeSourceCategory(category?: string | null) {
  const trimmed = category?.trim();
  if (!trimmed) return 'General';
  return trimmed.replace(/\s+/g, ' ');
}

export function getSourceColor(source?: Pick<Source, 'title' | 'category'> | string | null) {
  const title = typeof source === 'string' ? source : source?.title;
  const normalized = title?.trim();

  if (normalized) {
    const matched = SOURCE_MATCHERS.find(({ pattern }) => pattern.test(normalized));
    if (matched) return matched.color;
    return FALLBACK_COLORS[hashText(normalized.toLowerCase()) % FALLBACK_COLORS.length];
  }

  const category = typeof source === 'string' ? '' : normalizeSourceCategory(source?.category);
  return FALLBACK_COLORS[hashText(category.toLowerCase()) % FALLBACK_COLORS.length];
}

export function orderSourceGroups<T extends Pick<Source, 'category'>>(sources: T[]) {
  const grouped = new Map<string, T[]>();

  for (const source of sources) {
    const category = normalizeSourceCategory(source.category);
    grouped.set(category, [...(grouped.get(category) ?? []), source]);
  }

  const orderedLabels = [
    ...GROUP_ORDER.filter((label) => grouped.has(label)),
    ...[...grouped.keys()]
      .filter((label) => !GROUP_ORDER.includes(label as (typeof GROUP_ORDER)[number]))
      .sort((left, right) => left.localeCompare(right)),
  ];

  return orderedLabels.map((label) => [label, grouped.get(label) ?? []] as const);
}
