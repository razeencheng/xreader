import { render, screen } from '@testing-library/react';
import { HighlightsList } from './HighlightsList';
import { apiFetch } from '@/lib/api-client';

vi.mock('@/lib/api-client', () => ({
  apiFetch: vi.fn(),
}));

beforeEach(() => {
  vi.mocked(apiFetch).mockReset();
});

test('renders highlights from the API items envelope and links back to the article mark', async () => {
  vi.mocked(apiFetch).mockResolvedValue({
    items: [
      {
        id: 12,
        article_id: 7,
        quoted_text: 'important quote',
        note: 'review later',
        paragraph_index: 0,
        created_at: '2026-04-26T00:00:00Z',
        article_title: 'Original article',
        article_link: 'https://example.com/original',
      },
    ],
  });

  render(<HighlightsList />);

  const link = await screen.findByRole('link', { name: /Original article/i });
  expect(link).toHaveAttribute('href', '/?article=7#highlight-12');
  expect(screen.getByText(/important quote/)).toBeInTheDocument();
  expect(screen.getByText(/review later/)).toBeInTheDocument();
});
