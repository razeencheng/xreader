import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useUIStore } from '@/stores/useUIStore';

const push = vi.fn();
const usePathname = vi.fn();

vi.mock('next/navigation', () => ({
  useRouter: () => ({ push }),
  usePathname: () => usePathname(),
}));

import { Sidebar } from './Sidebar';

beforeEach(() => {
  push.mockReset();
  usePathname.mockReturnValue('/settings');
  useUIStore.setState({
    currentView: 'all',
    nativeLanguage: 'en-US',
    isShortcutsOpen: false,
    focusMode: false,
    selectedSourceId: null,
  });
});

test('clicking sidebar view routes back home from settings page', async () => {
  const user = userEvent.setup();

  render(<Sidebar />);

  await user.click(screen.getByTitle('Today'));

  expect(useUIStore.getState().currentView).toBe('today');
  expect(push).toHaveBeenCalledWith('/');
});

test('sidebar exposes the highlights and notes page', async () => {
  const user = userEvent.setup();

  render(<Sidebar />);

  await user.click(screen.getByTitle('My Highlights'));

  expect(push).toHaveBeenCalledWith('/highlights');
});

test('sidebar exposes a direct add source shortcut', async () => {
  const user = userEvent.setup();

  render(<Sidebar />);

  await user.click(screen.getByTitle('Add Source'));

  expect(push).toHaveBeenCalledWith('/sources#add-source');
});

test('sidebar keeps the add source shortcut directly under sources', () => {
  render(<Sidebar />);

  const navButtons = within(screen.getByRole('navigation')).getAllByRole('button');

  expect(navButtons.map((button) => button.getAttribute('title'))).toEqual([
    'Today',
    'All',
    'Starred',
    'Sources',
    'Add Source',
  ]);
});
