import { act, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import FeedPage from './page';
import { useUIStore } from '@/stores/useUIStore';
import type { ArticleItem } from '@/lib/types';

const push = vi.fn();
const replace = vi.fn();
const searchParamsMock = vi.fn(() => new URLSearchParams());
const articleItemsMock = vi.hoisted(() => ({
  items: [] as ArticleItem[],
}));

vi.mock('next/navigation', () => ({
  useRouter: () => ({ push, replace }),
  useSearchParams: () => searchParamsMock(),
}));

vi.mock('@/components/feed/FeedList', () => ({
  FeedList: ({ onOpenArticle, selectedArticleId }: { onOpenArticle?: (article: ArticleItem) => void; selectedArticleId?: number | null }) => (
    <div data-testid="feed-list" data-selected-id={selectedArticleId ?? ''}>
      <button
        type="button"
        onClick={() =>
          onOpenArticle?.({
            id: 7,
            source_id: 1,
            title: 'Article 7',
            link: 'https://example.com/7',
            language: 'en',
            is_read: false,
          })
        }
      >
        Open Article 7
      </button>
    </div>
  ),
}));

vi.mock('@/components/layout/SourceBrowser', () => ({
  SourceBrowser: () => <div data-testid="source-browser" />,
}));

vi.mock('@/components/reader/ArticleView', () => ({
  ArticleView: ({ id, onNotFound }: { id: string; onNotFound?: () => void }) => (
    <div data-testid="article-view">
      Article {id}
      <button type="button" onClick={onNotFound}>Report missing article</button>
    </div>
  ),
}));

vi.mock('@/hooks/useArticleNavigation', () => ({
  useArticleNavigation: vi.fn(),
}));

vi.mock('@/lib/queries/articles', () => ({
  useArticles: () => ({
    data: {
      pages: [
        {
          items: articleItemsMock.items,
          next_cursor: null,
        },
      ],
      pageParams: [undefined],
    },
  }),
}));

beforeEach(() => {
  push.mockReset();
  replace.mockReset();
  searchParamsMock.mockReset();
  searchParamsMock.mockReturnValue(new URLSearchParams());
  articleItemsMock.items = [
    { id: 2, source_id: 1, title: 'Article 2', link: 'https://example.com/2', language: 'en', is_read: false },
    { id: 7, source_id: 1, title: 'Article 7', link: 'https://example.com/7', language: 'en', is_read: false },
  ];
  useUIStore.setState({
    currentView: 'today',
    selectedSourceId: null,
    readFilter: 'unread',
    focusMode: false,
  });
});

function wrapper({ children }: { children: React.ReactNode }) {
  const qc = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });

  return <QueryClientProvider client={qc}>{children}</QueryClientProvider>;
}

test('stretches the feed transition pane across single-column layouts', async () => {
  render(<FeedPage />, { wrapper });

  const transitionPane = (await screen.findByTestId('feed-list')).parentElement;

  expect(transitionPane).toHaveClass('w-full');
});

test('stretches the source browser transition pane across single-column layouts', async () => {
  useUIStore.setState({ currentView: 'sources', selectedSourceId: null });

  render(<FeedPage />, { wrapper });

  const transitionPane = (await screen.findByTestId('source-browser')).parentElement;

  expect(transitionPane).toHaveClass('w-full');
});

test('unmounts the article list immediately when switching to source browser', async () => {
  useUIStore.setState({ currentView: 'starred', selectedSourceId: null });

  render(<FeedPage />, { wrapper });
  expect(await screen.findByTestId('feed-list')).toBeInTheDocument();

  act(() => {
    useUIStore.getState().setCurrentView('sources', null);
  });

  await screen.findByTestId('source-browser');
  await waitFor(() => {
    expect(screen.queryByTestId('feed-list')).not.toBeInTheDocument();
  });
});

test('restores the selected article from the URL after a hard refresh', async () => {
  searchParamsMock.mockReturnValue(new URLSearchParams('article=2&ctx=today'));

  render(<FeedPage />, { wrapper });

  expect(await screen.findByTestId('article-view')).toHaveTextContent('Article 2');
  expect(screen.getByTestId('feed-list')).toHaveAttribute('data-selected-id', '2');
});

test('writes the opened article into the URL so refresh keeps reading context', async () => {
  const user = userEvent.setup();

  render(<FeedPage />, { wrapper });
  await user.click(screen.getByRole('button', { name: 'Open Article 7' }));

  expect(push).toHaveBeenCalledWith('/?article=7&ctx=today');
});

test('replaces a missing selected article with the first visible article', async () => {
  const user = userEvent.setup();
  searchParamsMock.mockReturnValue(new URLSearchParams('article=149&ctx=today'));

  render(<FeedPage />, { wrapper });

  expect(await screen.findByTestId('article-view')).toHaveTextContent('Article 149');
  await user.click(screen.getByRole('button', { name: 'Report missing article' }));

  expect(replace).toHaveBeenCalledWith('/?article=2&ctx=today', { scroll: false });
});

test('skips a missing selected article that is still present in the cached list', async () => {
  const user = userEvent.setup();
  searchParamsMock.mockReturnValue(new URLSearchParams('article=149&ctx=today'));
  articleItemsMock.items = [
    { id: 149, source_id: 1, title: 'Missing Article', link: 'https://example.com/149', language: 'en', is_read: false },
    { id: 2, source_id: 1, title: 'Article 2', link: 'https://example.com/2', language: 'en', is_read: false },
  ];

  render(<FeedPage />, { wrapper });

  expect(await screen.findByTestId('article-view')).toHaveTextContent('Article 149');
  await user.click(screen.getByRole('button', { name: 'Report missing article' }));

  expect(replace).toHaveBeenCalledWith('/?article=2&ctx=today', { scroll: false });
});
