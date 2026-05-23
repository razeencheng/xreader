'use client';

import { useCallback, useEffect, useMemo, useRef, useState, type SyntheticEvent } from 'react';
import DOMPurify from 'dompurify';
import { createSSEClient, type SSEClient } from '@/lib/sse-client';
import {
  getBodyWarmup,
  subscribeBodyWarmup,
  claimBodyWarmup,
  releaseBodyWarmup,
} from '@/lib/body-translation-warmup';
import { fontForLang } from '@/lib/langFonts';
import { isSameLanguage } from '@/lib/article-meta';
import { useI18n } from '@/lib/i18n';

interface Props {
  articleId: number;
  contentHtml: string;
  language: string;
  nativeLanguage: string;
}

const BLOCK_TAGS = new Set([
  'article',
  'aside',
  'blockquote',
  'div',
  'details',
  'figure',
  'footer',
  'h1',
  'h2',
  'h3',
  'h4',
  'h5',
  'h6',
  'header',
  'li',
  'main',
  'ol',
  'p',
  'pre',
  'section',
  'table',
  'ul',
]);

const WRAPPER_TAGS = new Set(['article', 'aside', 'div', 'footer', 'header', 'main', 'ol', 'section', 'ul']);
const SKIP_EMPTY_TAGS = new Set(['br', 'ins']);
const TRANSLATION_PREFETCH_COUNT = 5;

type ReaderBlock = {
  html: string;
  blockTag?: string;
  translationIndex: number | null;
};

function escapeHtml(text: string) {
  return text
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&#39;');
}

function primaryTagFromHtml(html: string) {
  const parser = new DOMParser();
  const doc = parser.parseFromString(`<body>${html}</body>`, 'text/html');
  const firstElement = doc.body.firstElementChild;
  return firstElement?.tagName.toLowerCase() ?? 'p';
}

function isFilenameLikeAlt(text: string, src: string) {
  const normalized = text.trim().toLowerCase();
  if (!normalized) {
    return false;
  }
  const basename = src.split('/').pop()?.split('?')[0]?.replace(/\.[a-z0-9]+$/i, '').toLowerCase();
  return normalized === basename || /^image[-_]\d+$/i.test(normalized);
}

function removeHiddenHeadingAnchors(root: HTMLElement) {
  root.querySelectorAll('h1, h2, h3, h4, h5, h6').forEach((heading) => {
    heading.querySelectorAll('a[hidden], a.anchor[aria-hidden="true"]').forEach((anchor) => {
      anchor.remove();
    });

    const text = heading.textContent?.trim() ?? '';
    if (text.length > 2 && text.endsWith('#') && !text.endsWith('C#') && !text.endsWith('F#')) {
      heading.textContent = text.slice(0, -1).trim();
    }
  });
}

function normalizeReaderImages(root: HTMLElement) {
  root.querySelectorAll('img').forEach((image) => {
    const src = image.getAttribute('src')?.trim();
    if (src && /^https?:\/\//i.test(src) && !src.startsWith('/api/images/proxy')) {
      image.setAttribute('data-original-src', src);
      image.setAttribute('src', `/api/images/proxy?url=${encodeURIComponent(src)}`);
    }

    image.setAttribute('data-reader-image', 'true');
    image.setAttribute('loading', 'eager');
    image.setAttribute('decoding', 'async');

    const width = Number.parseInt(image.getAttribute('width') ?? '', 10);
    const height = Number.parseInt(image.getAttribute('height') ?? '', 10);
    const frame = image.ownerDocument.createElement('span');
    const frameStyles = width > 0 && height > 0
      ? [`aspect-ratio: ${width} / ${height}`, `max-width: ${width}px`]
      : ['aspect-ratio: 16 / 9'];
    frame.setAttribute('data-reader-image-frame', 'true');
    frame.setAttribute('data-image-state', 'idle');
    frame.setAttribute('style', frameStyles.join('; '));

    image.removeAttribute('style');
    image.parentNode?.insertBefore(frame, image);
    frame.appendChild(image);

    if (width > 0 && height > 0) {
      image.setAttribute('width', String(width));
      image.setAttribute('height', String(height));
    } else {
      image.removeAttribute('width');
      image.removeAttribute('height');
    }

    const alt = image.getAttribute('alt')?.trim() ?? '';
    if (src && isFilenameLikeAlt(alt, src)) {
      image.setAttribute('data-original-alt', alt);
      image.setAttribute('alt', '');
    }
  });
}

function normalizeBlockText(text: string) {
  return text.trim().split(/\s+/).filter(Boolean).join(' ');
}

function hasStandaloneMedia(element: HTMLElement) {
  return element.querySelector('img, picture, video, audio, canvas, svg, table') !== null;
}

function isHeadingTag(tag: string) {
  return /^h[1-6]$/.test(tag);
}

function listTagFromHtml(html: string) {
  return html.trim().toLowerCase().startsWith('<ol') ? 'ol' : 'ul';
}

function translationHtmlForBlock(block: ReaderBlock, blockTag: string, translation: string) {
  const escaped = escapeHtml(translation).replaceAll('\n', '<br />');

  if (isHeadingTag(blockTag)) {
    return `<${blockTag}>${escaped}</${blockTag}>`;
  }

  if (blockTag === 'li') {
    const listTag = listTagFromHtml(block.html);
    return `<${listTag}><li>${escaped}</li></${listTag}>`;
  }

  if (blockTag === 'blockquote') {
    return `<blockquote>${escaped}</blockquote>`;
  }

  return `<p>${escaped}</p>`;
}

function translationClassName(blockTag: string) {
  if (isHeadingTag(blockTag) || blockTag === 'li') {
    return 'mt-1 text-[var(--text-translation)]';
  }
  return 'mt-1 text-[0.92em] leading-[1.85] text-[var(--text-translation)]';
}

function sanitizeHtml(html: string): string {
  return DOMPurify.sanitize(html, {
    USE_PROFILES: { html: true },
    FORBID_TAGS: ['style', 'script', 'iframe', 'form', 'object', 'embed'],
    FORBID_ATTR: ['onerror', 'onclick', 'onload', 'onmouseover'],
  });
}

function splitContentHtml(contentHtml: string) {
  const sanitized = sanitizeHtml(contentHtml);
  const parser = new DOMParser();
  const doc = parser.parseFromString(`<body>${sanitized}</body>`, 'text/html');
  removeHiddenHeadingAnchors(doc.body);
  normalizeReaderImages(doc.body);
  const blocks: ReaderBlock[] = [];
  let translationIndex = 0;

  const pushBlock = (html: string, translatable: boolean, blockTag?: string) => {
    blocks.push({ html, blockTag, translationIndex: translatable ? translationIndex : null });
    if (translatable) {
      translationIndex += 1;
    }
  };

  const pushText = (text: string) => {
    const trimmed = normalizeBlockText(text);
    if (trimmed) {
      pushBlock(`<p>${escapeHtml(trimmed)}</p>`, true);
    }
  };

  const traverseNodes = (node: ChildNode) => {
    if (node.nodeType === Node.TEXT_NODE) {
      pushText(node.textContent ?? '');
      return;
    }

    if (node.nodeType !== Node.ELEMENT_NODE) {
      return;
    }

    const element = node as HTMLElement;
    const tag = element.tagName.toLowerCase();

    if (SKIP_EMPTY_TAGS.has(tag) && !element.textContent?.trim()) {
      return;
    }

    if (WRAPPER_TAGS.has(tag)) {
      Array.from(element.childNodes).forEach(traverseNodes);
      return;
    }

    if (tag === 'li') {
      const hasText = normalizeBlockText(element.textContent ?? '') !== '';
      const parentTag = element.parentElement?.tagName.toLowerCase() === 'ol' ? 'ol' : 'ul';
      if (hasText || hasStandaloneMedia(element)) {
        pushBlock(`<${parentTag}>${element.outerHTML}</${parentTag}>`, hasText, 'li');
      }
      return;
    }

    if (BLOCK_TAGS.has(tag)) {
      const hasText = normalizeBlockText(element.textContent ?? '') !== '';
      if (hasText || hasStandaloneMedia(element)) {
        pushBlock(element.outerHTML, hasText);
      }
      return;
    }

    const hasText = normalizeBlockText(element.textContent ?? '') !== '';
    if (hasText || hasStandaloneMedia(element)) {
      pushBlock(`<p>${element.outerHTML}</p>`, hasText);
    }
  };

  Array.from(doc.body.childNodes).forEach(traverseNodes);
  return blocks;
}

export function BilingualBody({ articleId, contentHtml, language, nativeLanguage }: Props) {
  const { t } = useI18n();
  const blocks = useMemo(() => splitContentHtml(contentHtml), [contentHtml]);
  const translatableCount = useMemo(
    () => blocks.reduce((count, block) => (block.translationIndex === null ? count : count + 1), 0),
    [blocks],
  );
  const sameLanguage = isSameLanguage(language, nativeLanguage);
  const originalFont = fontForLang(language);
  const translationFont = fontForLang(nativeLanguage);
  const resetKey = `${articleId}:${language}:${nativeLanguage}:${contentHtml}`;
  const paragraphRefs = useRef<Array<HTMLDivElement | null>>([]);
  const activeClientsRef = useRef<SSEClient[]>([]);
  const requestedIndicesRef = useRef<Set<number>>(new Set());
  const translationsRef = useRef<Map<number, string>>(new Map());
  const [translationState, setTranslationState] = useState<{
    key: string;
    translations: Map<number, string>;
    pending: Set<number>;
  }>({
    key: '',
    translations: new Map(),
    pending: new Set(),
  });

  const shouldTranslate = !sameLanguage && translatableCount > 0;
  const translations = useMemo(
    () => (translationState.key === resetKey ? translationState.translations : new Map<number, string>()),
    [resetKey, translationState.key, translationState.translations],
  );
  const pendingTranslations = useMemo(
    () => (translationState.key === resetKey ? translationState.pending : new Set<number>()),
    [resetKey, translationState.key, translationState.pending],
  );

  useEffect(() => {
    translationsRef.current = translations;
  }, [translations]);

  useEffect(() => {
    requestedIndicesRef.current.clear();
    activeClientsRef.current.forEach((client) => client.close());
    activeClientsRef.current = [];
    return () => {
      activeClientsRef.current.forEach((client) => client.close());
      activeClientsRef.current = [];
    };
  }, [resetKey]);

  // #3 prefetch: seed + adopt any warm-up started for this (article, native
  // language) so an opened-from-prefetch article shows its first paragraphs
  // immediately and never re-requests the warmed range (dedupe). Released on
  // cleanup so the store stays bounded and a resetKey change (e.g. "load
  // original content" swaps contentHtml) drops the now-mismatched entry
  // instead of re-seeding stale paragraph indices.
  useEffect(() => {
    if (!shouldTranslate) {
      return;
    }

    const applyParagraph = (index: number, translation: string) => {
      requestedIndicesRef.current.add(index);
      setTranslationState((previous) => {
        const nextTranslations =
          previous.key === resetKey ? new Map(previous.translations) : new Map<number, string>();
        const nextPending = previous.key === resetKey ? new Set(previous.pending) : new Set<number>();
        nextTranslations.set(index, translation);
        nextPending.delete(index);
        return { key: resetKey, translations: nextTranslations, pending: nextPending };
      });
    };

    const snapshot = getBodyWarmup(articleId, nativeLanguage);
    if (!snapshot) {
      return;
    }
    claimBodyWarmup(articleId, nativeLanguage);
    snapshot.translations.forEach((translation, index) => applyParagraph(index, translation));
    let unsubscribe = () => {};
    if (!snapshot.done) {
      unsubscribe = subscribeBodyWarmup(articleId, nativeLanguage, (paragraph) => {
        applyParagraph(paragraph.index, paragraph.translation);
      });
    }
    return () => {
      unsubscribe();
      releaseBodyWarmup(articleId, nativeLanguage);
    };
  }, [articleId, nativeLanguage, resetKey, shouldTranslate]);

  const requestTranslationRange = useCallback((visibleIndex: number) => {
    if (!shouldTranslate || visibleIndex < 0 || visibleIndex >= translatableCount) {
      return;
    }

    const rangeEnd = Math.min(translatableCount, visibleIndex + TRANSLATION_PREFETCH_COUNT);
    const rangeIndices = Array.from(
      { length: rangeEnd - visibleIndex },
      (_, offset) => visibleIndex + offset,
    );
    const firstMissingIndex = rangeIndices.find(
      (index) => !translationsRef.current.has(index) && !requestedIndicesRef.current.has(index),
    );
    if (firstMissingIndex === undefined) {
      return;
    }

    const requestIndices = Array.from(
      { length: rangeEnd - firstMissingIndex },
      (_, offset) => firstMissingIndex + offset,
    );
    const loadingIndex = firstMissingIndex === visibleIndex ? visibleIndex : null;
    requestIndices.forEach((index) => requestedIndicesRef.current.add(index));
    setTranslationState((previous) => {
      const nextTranslations = previous.key === resetKey ? previous.translations : new Map<number, string>();
      const nextPending = previous.key === resetKey ? new Set(previous.pending) : new Set<number>();
      if (loadingIndex !== null && !nextTranslations.has(loadingIndex)) {
        nextPending.add(loadingIndex);
      }
      return { key: resetKey, translations: nextTranslations, pending: nextPending };
    });

    const params = new URLSearchParams({
      start: String(firstMissingIndex),
      count: String(rangeEnd - firstMissingIndex),
    });
    const client = createSSEClient(`/api/articles/${articleId}/body-translation?${params.toString()}`);
    activeClientsRef.current.push(client);

    const removeClient = () => {
      activeClientsRef.current = activeClientsRef.current.filter((item) => item !== client);
    };
    const clearPending = () => {
      setTranslationState((previous) => {
        if (previous.key !== resetKey) {
          return previous;
        }
        const nextPending = new Set(previous.pending);
        if (loadingIndex !== null) {
          nextPending.delete(loadingIndex);
        }
        return { ...previous, pending: nextPending };
      });
    };

    client.onParagraph((paragraph) => {
      setTranslationState((previous) => {
        const nextTranslations = previous.key === resetKey ? new Map(previous.translations) : new Map<number, string>();
        const nextPending = previous.key === resetKey ? new Set(previous.pending) : new Set<number>();
        nextTranslations.set(paragraph.index, paragraph.translation);
        nextPending.delete(paragraph.index);
        return { key: resetKey, translations: nextTranslations, pending: nextPending };
      });
    });
    client.onDone(() => {
      clearPending();
      removeClient();
    });
    client.onError(() => {
      requestIndices.forEach((index) => requestedIndicesRef.current.delete(index));
      clearPending();
      removeClient();
    });
  }, [articleId, resetKey, shouldTranslate, translatableCount]);

  useEffect(() => {
    if (!shouldTranslate) {
      return;
    }

    const observer = new IntersectionObserver((entries) => {
      entries.forEach((entry) => {
        if (!entry.isIntersecting) {
          return;
        }
        const index = Number.parseInt((entry.target as HTMLElement).dataset.observeIndex ?? '', 10);
        if (Number.isFinite(index)) {
          requestTranslationRange(index);
        }
      });
    }, { rootMargin: '320px 0px 520px 0px', threshold: 0.01 });

    paragraphRefs.current.slice(0, translatableCount).forEach((node) => {
      if (node) {
        observer.observe(node);
      }
    });

    return () => {
      observer.disconnect();
    };
  }, [requestTranslationRange, shouldTranslate, translatableCount]);

  const updateReaderImageState = useCallback((event: SyntheticEvent<HTMLDivElement>) => {
    const image = event.target;
    if (!(image instanceof HTMLImageElement) || image.dataset.readerImage !== 'true') {
      return;
    }

    const frame = image.closest('[data-reader-image-frame]');
    if (!(frame instanceof HTMLElement)) {
      return;
    }

    if (event.type === 'error') {
      frame.dataset.imageState = 'error';
      image.setAttribute('aria-hidden', 'true');
      return;
    }

    frame.dataset.imageState = 'loaded';
    image.removeAttribute('aria-hidden');
  }, []);

  return (
    <div className="reader-content" onLoadCapture={updateReaderImageState} onErrorCapture={updateReaderImageState}>
      {blocks.map((block, index) => {
        const blockTag = block.blockTag ?? primaryTagFromHtml(block.html);
        const isCode = blockTag === 'pre';
        const translationIndex = block.translationIndex;
        const translation = translationIndex === null ? undefined : translations.get(translationIndex);
        const isLoading = translationIndex !== null && pendingTranslations.has(translationIndex) && !translation;
        const translationHtml = translation ? translationHtmlForBlock(block, blockTag, translation) : '';

        if (isCode) {
          return (
            <div
              key={index}
              ref={(node) => {
                if (translationIndex !== null) {
                  paragraphRefs.current[translationIndex] = node;
                }
              }}
              data-observe-index={translationIndex ?? undefined}
              data-block-tag={blockTag}
              className="overflow-x-auto rounded-lg border border-[var(--border-light)] bg-[var(--bg-surface)] p-4 font-mono text-[13.5px] leading-[1.6] text-[var(--text-primary)]"
              dangerouslySetInnerHTML={{ __html: block.html }}
            />
          );
        }

        return (
          <div
            key={index}
            ref={(node) => {
              if (translationIndex !== null) {
                paragraphRefs.current[translationIndex] = node;
              }
            }}
            data-observe-index={translationIndex ?? undefined}
            data-block-tag={blockTag}
            className="paragraph-container"
          >
            <div
              data-layer="original"
              data-paragraph-index={translationIndex ?? undefined}
              style={{ fontFamily: originalFont }}
            >
              <div dangerouslySetInnerHTML={{ __html: block.html }} />
            </div>

            {translation ? (
              <div
                data-layer="translation"
                data-paragraph-index={translationIndex ?? undefined}
                className={translationClassName(blockTag)}
                style={{ fontFamily: translationFont }}
                dangerouslySetInnerHTML={{ __html: translationHtml }}
              />
            ) : isLoading ? (
              <div
                data-testid="translation-loading"
                aria-label={t('reader.translatingParagraph')}
                className="mt-1 flex h-5 items-center gap-1 pl-4"
              >
                <span className="h-1.5 w-1.5 animate-pulse rounded-full bg-[var(--text-3)]" />
                <span className="h-1.5 w-1.5 animate-pulse rounded-full bg-[var(--text-3)] [animation-delay:120ms]" />
                <span className="h-1.5 w-1.5 animate-pulse rounded-full bg-[var(--text-3)] [animation-delay:240ms]" />
              </div>
            ) : null}
          </div>
        );
      })}
    </div>
  );
}
