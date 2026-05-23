import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { NextUpCard } from './NextUpCard';
import type { ArticleItem } from '@/lib/types';

const push = vi.fn();
const useSearchParams = vi.fn();

vi.mock('next/navigation', () => ({
  useRouter: () => ({ push }),
  useSearchParams: () => useSearchParams(),
}));

const next: ArticleItem = {
  id: 3,
  source_id: 1,
  title: 'Original headline',
  title_translated: '翻译后的标题',
  source_title: 'Cloudflare',
  published_at: new Date(Date.now() - 3 * 3_600_000).toISOString(),
  link: '',
  language: 'en',
  summary: 'Summary text',
};

beforeEach(() => {
  push.mockReset();
  useSearchParams.mockReturnValue(new URLSearchParams('ctx=today'));
});

test('renders original subtitle when translated title exists', () => {
  render(<NextUpCard next={next} currentId={2} markRead={vi.fn()} />);

  expect(screen.getByText('翻译后的标题')).toBeInTheDocument();
  expect(screen.getByText('Original headline')).toBeInTheDocument();
  expect(screen.getByText('Cloudflare')).toBeInTheDocument();
});

test('clicking card marks current article read before navigation', async () => {
  const markRead = vi.fn();
  render(<NextUpCard next={next} currentId={2} markRead={markRead} />);

  await userEvent.click(screen.getByRole('button', { name: /翻译后的标题/i }));

  expect(markRead).toHaveBeenCalledWith(2);
  expect(push).toHaveBeenCalledWith('/?ctx=today&article=3');
});
