import { render } from '@testing-library/react';
import { act } from 'react';
import { dispatchKey } from '@/lib/keyboard';
import { useUIStore } from '@/stores/useUIStore';
import { useGlobalShortcuts } from './useGlobalShortcuts';

function Harness() {
  useGlobalShortcuts();
  return null;
}

beforeEach(() => {
  useUIStore.setState({ isShortcutsOpen: false, focusMode: false });
});

test('question mark opens global shortcuts and escape closes it', () => {
  render(<Harness />);

  act(() => {
    expect(dispatchKey('?')).toBe(true);
  });
  expect(useUIStore.getState().isShortcutsOpen).toBe(true);

  act(() => {
    expect(dispatchKey('escape')).toBe(true);
  });
  expect(useUIStore.getState().isShortcutsOpen).toBe(false);
});

test('escape exits focus mode when shortcuts are closed', () => {
  useUIStore.setState({ focusMode: true, isShortcutsOpen: false });
  render(<Harness />);

  act(() => {
    expect(dispatchKey('escape')).toBe(true);
  });

  expect(useUIStore.getState().focusMode).toBe(false);
});
