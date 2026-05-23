import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { KeyboardShortcutsModal } from './KeyboardShortcutsModal';
import { useUIStore } from '@/stores/useUIStore';

test('renders handoff shortcut groups and closes from the close button', async () => {
  const user = userEvent.setup();
  const onClose = vi.fn();

  useUIStore.setState({ nativeLanguage: 'en-US' });
  render(<KeyboardShortcutsModal open onClose={onClose} />);

  expect(screen.getByRole('dialog', { name: /keyboard shortcuts/i })).toBeInTheDocument();
  expect(screen.getByText('Navigation')).toBeInTheDocument();
  expect(screen.getByText('Article')).toBeInTheDocument();
  expect(screen.getByText('View')).toBeInTheDocument();
  expect(screen.getByText('Next article')).toBeInTheDocument();
  expect(screen.getByText('Previous article')).toBeInTheDocument();
  expect(screen.getByText('Scroll down')).toBeInTheDocument();
  expect(screen.getByText('Scroll up')).toBeInTheDocument();
  expect(screen.getByText('H')).toBeInTheDocument();
  expect(screen.getByText('L')).toBeInTheDocument();
  expect(screen.getByText('Star / unstar current article')).toBeInTheDocument();
  expect(screen.getByText('Toggle focus mode')).toBeInTheDocument();

  await user.click(screen.getByRole('button', { name: /close keyboard shortcuts/i }));

  expect(onClose).toHaveBeenCalledTimes(1);
});
