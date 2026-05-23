import type { ArticleItem } from '@/lib/types';

function stripHtml(html: string) {
  return html.replace(/<[^>]+>/g, ' ');
}

function countContentBlocks(html: string) {
  const explicitBlocks = html.match(/<\/(?:p|li|blockquote|pre|h[1-6]|section|article)>/gi)?.length ?? 0;
  if (explicitBlocks > 0) return explicitBlocks;

  const textBlocks = stripHtml(html)
    .split(/\n{2,}/)
    .map((block) => block.trim())
    .filter(Boolean);

  return textBlocks.length || (stripHtml(html).trim() ? 1 : 0);
}

export function formatRelativeTime(value?: string) {
  if (!value) return '';

  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '';

  const seconds = Math.max(1, Math.floor((Date.now() - date.getTime()) / 1000));
  const units: Array<[label: string, size: number]> = [
    ['y', 31_536_000],
    ['mo', 2_592_000],
    ['d', 86_400],
    ['h', 3_600],
    ['m', 60],
  ];

  for (const [label, size] of units) {
    if (seconds >= size) {
      return `${Math.floor(seconds / size)}${label}`;
    }
  }

  return `${seconds}s`;
}

export function estimateReadMinutes(
  article: Pick<ArticleItem, 'title' | 'title_translated' | 'summary' | 'word_count'> & {
    content_text?: string;
    content_html?: string;
  },
) {
  if (article.word_count && article.word_count > 0) {
    return Math.max(1, Math.round(article.word_count / 300));
  }

  const content = [
    article.title_translated,
    article.title,
    article.summary,
    article.content_text,
    article.content_html ? stripHtml(article.content_html) : '',
  ]
    .filter(Boolean)
    .join(' ')
    .trim();

  if (!content) return null;

  const words = content.split(/\s+/).filter(Boolean).length;
  const characters = content.replace(/\s+/g, '').length;
  const estimate = Math.max(words / 220, characters / 420);

  return Math.max(1, Math.round(estimate));
}

export function isLikelySummaryOnly(
  article: Pick<ArticleItem, 'link'> & {
    content_text?: string;
    content_html?: string;
  },
) {
  if (!article.link) return false;

  const html = article.content_html?.trim() ?? '';
  const text = (article.content_text || (html ? stripHtml(html) : '')).replace(/\s+/g, ' ').trim();
  if (!text) return false;

  const blockCount = html ? countContentBlocks(html) : 1;
  const compactLength = text.replace(/\s+/g, '').length;

  return blockCount <= 1 && compactLength < 700;
}

export function getDisplayTitle(article: Pick<ArticleItem, 'title' | 'title_translated'>) {
  return article.title_translated?.trim() || article.title;
}

export function getOriginalTitle(article: Pick<ArticleItem, 'title' | 'title_translated'>) {
  const displayTitle = getDisplayTitle(article);
  if (!article.title_translated || displayTitle === article.title) {
    return null;
  }

  return article.title;
}

function normalizeLanguage(value: string) {
  return value.toLowerCase().split('-')[0];
}

export function isSameLanguage(articleLanguage: string, nativeLanguage: string) {
  return normalizeLanguage(articleLanguage) === normalizeLanguage(nativeLanguage);
}
