import { fireEvent, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useUIStore } from '@/stores/useUIStore';
import { LanguageModal } from './LanguageModal';

test('closes when pressing Escape', () => {
  const onClose = vi.fn();
  const onSelect = vi.fn();

  useUIStore.setState({ nativeLanguage: 'zh-CN' });
  render(<LanguageModal currentLanguage="zh-CN" onSelect={onSelect} onClose={onClose} />);

  expect(screen.getByText('母语')).toBeInTheDocument();

  // Focus trap captures Escape on the dialog container
  const dialog = screen.getByRole('dialog');
  fireEvent.keyDown(dialog, { key: 'Escape' });

  expect(onClose).toHaveBeenCalledTimes(1);
});

test('clicking a language triggers select and close', async () => {
  const user = userEvent.setup();
  const onClose = vi.fn();
  const onSelect = vi.fn();

  useUIStore.setState({ nativeLanguage: 'zh-CN' });
  render(<LanguageModal currentLanguage="zh-CN" onSelect={onSelect} onClose={onClose} />);

  await user.click(screen.getByRole('button', { name: /English/i }));

  expect(onSelect).toHaveBeenCalledTimes(1);
  expect(onSelect).toHaveBeenCalledWith('en-US');
  expect(onClose).toHaveBeenCalledTimes(1);
});
