import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { OriginalArticleButton } from './OriginalArticleButton';

test('opens the original article in a new tab', async () => {
  const open = vi.spyOn(window, 'open').mockImplementation(() => null);

  render(<OriginalArticleButton href="https://example.com/original" />);

  await userEvent.click(screen.getByRole('button', { name: '阅读原文' }));

  expect(open).toHaveBeenCalledWith('https://example.com/original', '_blank', 'noopener,noreferrer');
  open.mockRestore();
});

test('renders as an inline metadata action instead of a primary button', () => {
  render(<OriginalArticleButton href="https://example.com/original" />);

  const button = screen.getByRole('button', { name: '阅读原文' });
  expect(button.className).toContain('min-h-11');
  expect(button.className).toContain('text-[var(--accent)]');
  expect(button.className).not.toContain('bg-[#f73a69]');
});
