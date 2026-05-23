import { act, fireEvent, render, screen } from '@testing-library/react';
import {
  startBodyWarmup,
  getBodyWarmup,
  __resetBodyWarmupForTests,
} from '@/lib/body-translation-warmup';

const sse = vi.hoisted(() => {
  type ClientRecord = {
    url: string;
    paragraphHandler: ((paragraph: { index: number; translation: string }) => void) | null;
    doneHandler: (() => void) | null;
    close: ReturnType<typeof vi.fn>;
  };

  const clients: ClientRecord[] = [];
  const createSSEClient = vi.fn((url: string) => {
    const client: ClientRecord = {
      url,
      paragraphHandler: null,
      doneHandler: null,
      close: vi.fn(),
    };
    clients.push(client);
    return {
      onParagraph: (callback: NonNullable<ClientRecord['paragraphHandler']>) => {
        client.paragraphHandler = callback;
      },
      onDone: (callback: NonNullable<ClientRecord['doneHandler']>) => {
        client.doneHandler = callback;
      },
      onError: vi.fn(),
      close: client.close,
    };
  });

  return {
    createSSEClient,
    get clients() {
      return clients;
    },
    pushParagraph: (clientIndex: number, paragraph: { index: number; translation: string }) => {
      clients[clientIndex]?.paragraphHandler?.(paragraph);
    },
    pushDone: (clientIndex: number) => clients[clientIndex]?.doneHandler?.(),
    reset: () => {
      clients.splice(0, clients.length);
      createSSEClient.mockClear();
    },
  };
});

vi.mock('@/lib/sse-client', () => ({
  createSSEClient: sse.createSSEClient,
}));

import { BilingualBody } from './BilingualBody';
import { useUIStore } from '@/stores/useUIStore';

const contentHtml = '<p>First paragraph</p><p>Second paragraph</p>';
let intersectionCallback:
  | ((entries: Array<Pick<IntersectionObserverEntry, 'isIntersecting' | 'target'>>) => void)
  | null = null;

class FakeIntersectionObserver {
  observe = vi.fn();
  unobserve = vi.fn();
  disconnect = vi.fn();

  constructor(callback: typeof intersectionCallback) {
    intersectionCallback = callback;
  }
}

beforeEach(() => {
  sse.reset();
  intersectionCallback = null;
  vi.stubGlobal('IntersectionObserver', FakeIntersectionObserver);
  useUIStore.setState({ nativeLanguage: 'zh-CN' });
});

afterEach(() => {
  vi.unstubAllGlobals();
});

function enterParagraph(container: HTMLElement, index: number) {
  const target = container.querySelector(`[data-observe-index="${index}"]`);
  expect(target).toBeInTheDocument();
  act(() => {
    intersectionCallback?.([{ isIntersecting: true, target: target as Element }]);
  });
}

test('renders original paragraphs without translation controls in same-language mode', () => {
  render(
    <BilingualBody articleId={1} contentHtml={contentHtml} language="zh-CN" nativeLanguage="zh" />,
  );

  expect(screen.getByText('First paragraph')).toBeInTheDocument();
  expect(screen.getByText('Second paragraph')).toBeInTheDocument();
  expect(screen.queryByRole('button', { name: /翻译段落/i })).not.toBeInTheDocument();
  expect(sse.createSSEClient).not.toHaveBeenCalled();
});

test('opens an SSE stream for the visible paragraph range and renders translations as they arrive', () => {
  const { container } = render(
    <BilingualBody articleId={1} contentHtml={contentHtml} language="en" nativeLanguage="zh-CN" />,
  );

  expect(screen.getByText('First paragraph')).toBeInTheDocument();
  expect(screen.getByText('Second paragraph')).toBeInTheDocument();
  expect(sse.createSSEClient).not.toHaveBeenCalled();

  enterParagraph(container, 0);

  expect(sse.createSSEClient).toHaveBeenCalledWith('/api/articles/1/body-translation?start=0&count=2');
  expect(screen.queryByRole('button', { name: /翻译段落/i })).not.toBeInTheDocument();
  expect(screen.getAllByTestId('translation-loading')).toHaveLength(1);

  act(() => {
    sse.pushParagraph(0, { index: 0, translation: '第一段翻译' });
  });
  expect(screen.getByText('第一段翻译')).toBeInTheDocument();
  expect(screen.queryByTestId('translation-loading')).not.toBeInTheDocument();

  act(() => {
    sse.pushParagraph(0, { index: 1, translation: '第二段翻译' });
  });
  expect(screen.getByText('第二段翻译')).toBeInTheDocument();
  expect(screen.queryByTestId('translation-loading')).not.toBeInTheDocument();
});

test('renders translated paragraphs without decorative left rule', () => {
  const { container } = render(
    <BilingualBody articleId={1} contentHtml="<p>Hello world</p>" language="en" nativeLanguage="zh-CN" />,
  );

  enterParagraph(container, 0);
  act(() => {
    sse.pushParagraph(0, { index: 0, translation: '你好，世界' });
  });

  const translation = screen.getByText('你好，世界');
  const translationLayer = translation.closest('[data-layer="translation"]');
  expect(translationLayer).toHaveAttribute('data-layer', 'translation');
  expect(translationLayer?.className).not.toContain('border-l');
  expect(translationLayer?.className).not.toContain('pl-4');
});

test('does not render loading placeholders for prefetched paragraphs', () => {
  const longContent = Array.from({ length: 6 }, (_, index) => `<p>Paragraph ${index}</p>`).join('');
  const { container } = render(
    <BilingualBody articleId={1} contentHtml={longContent} language="en" nativeLanguage="zh-CN" />,
  );

  enterParagraph(container, 1);

  expect(sse.createSSEClient).toHaveBeenCalledWith('/api/articles/1/body-translation?start=1&count=5');
  expect(screen.getAllByTestId('translation-loading')).toHaveLength(1);
  const loadingParent = screen.getByTestId('translation-loading').closest('[data-observe-index]');
  expect(loadingParent).toHaveAttribute('data-observe-index', '1');
});

test('prefetches the current paragraph and the following four paragraphs', () => {
  const longContent = Array.from({ length: 6 }, (_, index) => `<p>Paragraph ${index}</p>`).join('');
  const { container } = render(
    <BilingualBody articleId={1} contentHtml={longContent} language="en" nativeLanguage="zh-CN" />,
  );

  enterParagraph(container, 2);

  expect(sse.createSSEClient).toHaveBeenCalledWith('/api/articles/1/body-translation?start=2&count=4');
});

test('proxies external reader images and reserves their aspect ratio', () => {
  const { container } = render(
    <BilingualBody
      articleId={1}
      contentHtml='<p>如下图：<img width="640" height="417" src="https://st.deepzz.cn/blog/img/single-open-double-charged.jpg" alt="single-open-double-charged"/></p>'
      language="zh-CN"
      nativeLanguage="zh"
    />,
  );

  const image = container.querySelector('img');
  const frame = container.querySelector('[data-reader-image-frame]');
  expect(frame).toBeInTheDocument();
  expect(frame?.getAttribute('style')).toContain('aspect-ratio: 640 / 417');
  expect(frame?.getAttribute('style')).toContain('max-width: 640px');
  expect(image).toBeInTheDocument();
  expect(frame).toContainElement(image);
  expect(image?.getAttribute('src')).toBe(
    '/api/images/proxy?url=https%3A%2F%2Fst.deepzz.cn%2Fblog%2Fimg%2Fsingle-open-double-charged.jpg',
  );
  expect(image?.getAttribute('data-original-src')).toBe('https://st.deepzz.cn/blog/img/single-open-double-charged.jpg');
  expect(image).toHaveAttribute('data-reader-image', 'true');
  expect(image).toHaveAttribute('loading', 'eager');
  expect(image).toHaveAttribute('decoding', 'async');
});

test('keeps failed reader images in a stable frame with an explicit error state', () => {
  const { container } = render(
    <BilingualBody
      articleId={1}
      contentHtml='<p><img src="https://static.example.com/missing.jpg" alt="Article visual"/></p>'
      language="zh-CN"
      nativeLanguage="zh"
    />,
  );

  const image = container.querySelector('img');
  const frame = container.querySelector('[data-reader-image-frame]');
  expect(frame).toBeInTheDocument();
  expect(frame?.getAttribute('style')).toContain('aspect-ratio: 16 / 9');

  fireEvent.error(image as HTMLImageElement);

  expect(frame).toHaveAttribute('data-image-state', 'error');
  expect(image).toHaveAttribute('aria-hidden', 'true');
});

test('marks semantic article blocks so the reader stylesheet can preserve hierarchy', () => {
  const { container } = render(
    <BilingualBody
      articleId={1}
      contentHtml="<details><summary>目录</summary><ul><li>第一节</li></ul></details><h3>章节标题</h3><p>正文段落</p>"
      language="zh-CN"
      nativeLanguage="zh"
    />,
  );

  expect(container.querySelector('.reader-content')).toBeInTheDocument();
  expect(container.querySelector('[data-block-tag="details"] details')).toBeInTheDocument();
  expect(container.querySelector('[data-block-tag="h3"] h3')).toHaveTextContent('章节标题');
  expect(container.querySelector('[data-block-tag="p"] p')).toHaveTextContent('正文段落');
});

test('keeps paragraphs with inline code in normal text flow', () => {
  const { container } = render(
    <BilingualBody
      articleId={1}
      contentHtml="<p>老习惯，最后贴一下全部的配置。<code>.gitlab-ci.yml</code>。加了一个<code>resource_group</code>。</p><pre><code>image: docker:latest</code></pre>"
      language="zh-CN"
      nativeLanguage="zh"
    />,
  );

  const paragraphBlock = container.querySelector('[data-block-tag="p"]');
  expect(paragraphBlock).toHaveClass('paragraph-container');
  expect(paragraphBlock).toHaveTextContent('老习惯，最后贴一下全部的配置。');
  expect(paragraphBlock).not.toHaveClass('font-mono');
  expect(container.querySelector('[data-block-tag="pre"]')).toHaveClass('font-mono');
});

test('keeps translations attached to their original text after empty article blocks', () => {
  const { container } = render(
    <BilingualBody
      articleId={1}
      contentHtml="<h2>How it works</h2><p> </p><p>Install the Stripe CLI</p><pre><code>stripe projects init</code></pre>"
      language="en"
      nativeLanguage="zh-CN"
    />,
  );

  enterParagraph(container, 0);

  act(() => {
    sse.pushParagraph(0, { index: 0, translation: '工作原理' });
    sse.pushParagraph(0, { index: 1, translation: '安装 Stripe CLI' });
    sse.pushParagraph(0, { index: 2, translation: '条纹项目初始化' });
  });

  const text = container.textContent ?? '';
  expect(text.indexOf('Install the Stripe CLI')).toBeLessThan(text.indexOf('安装 Stripe CLI'));
  expect(text.indexOf('安装 Stripe CLI')).toBeLessThan(text.indexOf('stripe projects init'));
  expect(text.indexOf('条纹项目初始化')).toBe(-1);
});

test('renders heading translations with the original heading level', () => {
  const { container } = render(
    <BilingualBody
      articleId={1}
      contentHtml="<h2>How it works: zero to production</h2>"
      language="en"
      nativeLanguage="zh-CN"
    />,
  );

  enterParagraph(container, 0);

  act(() => {
    sse.pushParagraph(0, { index: 0, translation: '其工作原理：从零到生产' });
  });

  const translatedHeading = screen.getByRole('heading', { name: '其工作原理：从零到生产', level: 2 });
  expect(translatedHeading.closest('[data-layer="translation"]')).toHaveAttribute('data-layer', 'translation');
});

test('renders list item translations directly under each original item', () => {
  const { container } = render(
    <BilingualBody
      articleId={1}
      contentHtml="<p>The agent has gone from zero to having:</p><ul><li>Provisioned a new Cloudflare account</li><li>Obtained an API token</li><li>Purchased a domain</li><li>Deployed an app to production</li></ul>"
      language="en"
      nativeLanguage="zh-CN"
    />,
  );

  enterParagraph(container, 0);

  act(() => {
    sse.pushParagraph(0, { index: 0, translation: '该代理已经从零开始发展到拥有：' });
    sse.pushParagraph(0, { index: 1, translation: '配置了新的 Cloudflare 账户' });
    sse.pushParagraph(0, { index: 2, translation: '获取了 API 令牌' });
    sse.pushParagraph(0, { index: 3, translation: '购买了一个域名' });
    sse.pushParagraph(0, { index: 4, translation: '将应用程序部署到生产环境' });
  });

  const text = container.textContent ?? '';
  expect(text.indexOf('Provisioned a new Cloudflare account')).toBeLessThan(text.indexOf('配置了新的 Cloudflare 账户'));
  expect(text.indexOf('配置了新的 Cloudflare 账户')).toBeLessThan(text.indexOf('Obtained an API token'));
  expect(text.indexOf('Obtained an API token')).toBeLessThan(text.indexOf('获取了 API 令牌'));
  expect(text.indexOf('获取了 API 令牌')).toBeLessThan(text.indexOf('Purchased a domain'));
  expect(text.indexOf('Purchased a domain')).toBeLessThan(text.indexOf('购买了一个域名'));
  expect(text.indexOf('购买了一个域名')).toBeLessThan(text.indexOf('Deployed an app to production'));
  expect(text.indexOf('Deployed an app to production')).toBeLessThan(text.indexOf('将应用程序部署到生产环境'));

  const translatedItems = Array.from(container.querySelectorAll('[data-layer="translation"] li'));
  expect(translatedItems.map((item) => item.textContent)).toEqual([
    '配置了新的 Cloudflare 账户',
    '获取了 API 令牌',
    '购买了一个域名',
    '将应用程序部署到生产环境',
  ]);
});

test('removes hidden heading anchors from sanitized article html', () => {
  render(
    <BilingualBody
      articleId={1}
      contentHtml='<h3>详细配置<a hidden class="anchor" aria-hidden="true" href="#详细配置">#</a></h3><h3>总结#</h3>'
      language="zh-CN"
      nativeLanguage="zh"
    />,
  );

  expect(screen.getByRole('heading', { name: '详细配置' })).toBeInTheDocument();
  expect(screen.getByRole('heading', { name: '总结' })).toBeInTheDocument();
  expect(screen.queryByRole('heading', { name: '详细配置#' })).not.toBeInTheDocument();
  expect(screen.queryByRole('heading', { name: '总结#' })).not.toBeInTheDocument();
});

test('stabilizes external images without dimensions and suppresses filename-like fallback alt text', () => {
  const { container } = render(
    <BilingualBody
      articleId={1}
      contentHtml='<p><img src="https://st.razeen.me/img/2025/image-20250412165248283.webp" alt="image-20250412165248283"/></p>'
      language="zh-CN"
      nativeLanguage="zh"
    />,
  );

  const image = container.querySelector('img');
  const frame = container.querySelector('[data-reader-image-frame]');
  expect(image).toBeInTheDocument();
  expect(frame).toContainElement(image);
  expect(image?.getAttribute('src')).toBe(
    '/api/images/proxy?url=https%3A%2F%2Fst.razeen.me%2Fimg%2F2025%2Fimage-20250412165248283.webp',
  );
  expect(image).toHaveAttribute('data-original-alt', 'image-20250412165248283');
  expect(image).toHaveAttribute('alt', '');
  expect(frame?.getAttribute('style')).toContain('aspect-ratio: 16 / 9');
});

describe('BilingualBody warm-up adoption', () => {
  afterEach(() => {
    __resetBodyWarmupForTests();
    sse.reset();
  });

  test('seeds already-warmed paragraphs on mount and does not open a new SSE for them', () => {
    startBodyWarmup(1, 'zh-CN', 5);
    act(() => {
      sse.pushParagraph(0, { index: 0, translation: '预热第一段' });
      sse.pushParagraph(0, { index: 1, translation: '预热第二段' });
      sse.pushDone(0);
    });
    const callsBeforeMount = sse.createSSEClient.mock.calls.length;

    const { container } = render(
      <BilingualBody articleId={1} contentHtml={contentHtml} language="en" nativeLanguage="zh-CN" />,
    );

    expect(screen.getByText('预热第一段')).toBeInTheDocument();
    expect(screen.getByText('预热第二段')).toBeInTheDocument();
    enterParagraph(container, 0);
    expect(sse.createSSEClient.mock.calls.length).toBe(callsBeforeMount);
  });

  test('adopts an in-flight warm-up: paragraphs arriving after mount still render', () => {
    startBodyWarmup(1, 'zh-CN', 5);
    render(
      <BilingualBody articleId={1} contentHtml={contentHtml} language="en" nativeLanguage="zh-CN" />,
    );
    const callsAfterMount = sse.createSSEClient.mock.calls.length;
    act(() => {
      sse.pushParagraph(0, { index: 0, translation: '流式第一段' });
    });
    expect(screen.getByText('流式第一段')).toBeInTheDocument();
    expect(sse.createSSEClient.mock.calls.length).toBe(callsAfterMount);
  });

  test('releases the warm entry on unmount so the store stays bounded', () => {
    startBodyWarmup(1, 'zh-CN', 5);
    const { unmount } = render(
      <BilingualBody articleId={1} contentHtml={contentHtml} language="en" nativeLanguage="zh-CN" />,
    );
    expect(getBodyWarmup(1, 'zh-CN')).toBeDefined();
    unmount();
    expect(getBodyWarmup(1, 'zh-CN')).toBeUndefined();
  });

  test('releases the entry on a resetKey change (content swap) and does not re-seed', () => {
    startBodyWarmup(1, 'zh-CN', 5);
    act(() => {
      sse.pushParagraph(0, { index: 0, translation: '预热第一段' });
      sse.pushParagraph(0, { index: 1, translation: '预热第二段' });
      sse.pushDone(0);
    });

    const { rerender } = render(
      <BilingualBody articleId={1} contentHtml={contentHtml} language="en" nativeLanguage="zh-CN" />,
    );
    expect(screen.getByText('预热第一段')).toBeInTheDocument();
    expect(getBodyWarmup(1, 'zh-CN')).toBeDefined();

    // Simulate "load original content": same article, swapped contentHtml ->
    // resetKey changes -> the warm effect cleanup must release the entry, and
    // the re-run must NOT re-seed the stale (summary-indexed) translations.
    rerender(
      <BilingualBody
        articleId={1}
        contentHtml={'<p>Original first</p><p>Original second</p>'}
        language="en"
        nativeLanguage="zh-CN"
      />,
    );

    expect(getBodyWarmup(1, 'zh-CN')).toBeUndefined();
    expect(screen.queryByText('预热第一段')).not.toBeInTheDocument();
    expect(screen.getByText('Original first')).toBeInTheDocument();
  });
});
