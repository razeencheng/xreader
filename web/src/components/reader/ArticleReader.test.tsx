import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import { ApiError, apiFetch } from '@/lib/api-client';
import { ArticleReader } from './ArticleReader';
import { useUIStore } from '@/stores/useUIStore';

vi.mock('@/lib/api-client', () => {
  class MockApiError extends Error {
    code: string;
    status: number;

    constructor(status: number, code: string, message: string) {
      super(message);
      this.name = 'ApiError';
      this.status = status;
      this.code = code;
    }
  }

  return {
    ApiError: MockApiError,
    apiFetch: vi.fn(),
  };
});

function wrapper({ children }: { children: React.ReactNode }) {
  const qc = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });

  return <QueryClientProvider client={qc}>{children}</QueryClientProvider>;
}

beforeEach(() => {
  vi.mocked(apiFetch).mockReset();
  useUIStore.setState({
    nativeLanguage: 'en-US',
    fontSize: 17,
    layout: 'classic',
    focusMode: false,
  });
});

test('notifies the parent when the selected article no longer exists', async () => {
  const onNotFound = vi.fn();
  vi.mocked(apiFetch).mockRejectedValue(new ApiError(404, 'UNKNOWN', 'article not found'));

  render(<ArticleReader id="149" onNotFound={onNotFound} />, { wrapper });

  await waitFor(() => {
    expect(onNotFound).toHaveBeenCalledTimes(1);
  });
});

test('wide layout keeps article text within a readable max width', async () => {
  useUIStore.setState({ layout: 'wide', nativeLanguage: 'en-US' });
  vi.mocked(apiFetch).mockImplementation(async (url: string) => {
    if (url === '/api/articles/88') {
      return {
        id: 88,
        source_id: 1,
        title: 'A readable wide article',
        link: 'https://example.com/readable',
        language: 'en',
        source_title: 'Example',
        content_text: 'Long text for a wide reading layout.',
      };
    }
    return null;
  });

  const { container } = render(<ArticleReader id="88" />, { wrapper });

  await screen.findByRole('heading', { name: 'A readable wide article' });

  expect(container.querySelector('article')).toHaveClass('mx-auto', 'max-w-[960px]');
});
