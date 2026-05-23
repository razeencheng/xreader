import { act, render, screen } from '@testing-library/react';
import { dispatchKey } from '@/lib/keyboard';
import { useUIStore } from '@/stores/useUIStore';
import { useReaderShortcuts } from './useReaderShortcuts';

function Harness(props: Parameters<typeof useReaderShortcuts>[0]) {
  const { isShortcutsOpen } = useReaderShortcuts(props);
  return <div>{isShortcutsOpen ? 'shortcuts-open' : 'shortcuts-closed'}</div>;
}

beforeEach(() => {
  useUIStore.setState({ isShortcutsOpen: false });
});

test('global shortcuts modal blocks article actions while open', () => {
  const onNext = vi.fn();

  render(<Harness onNext={onNext} />);

  expect(screen.getByText('shortcuts-closed')).toBeInTheDocument();
  act(() => {
    useUIStore.getState().openShortcuts();
  });
  expect(screen.getByText('shortcuts-open')).toBeInTheDocument();

  expect(dispatchKey('l')).toBe(false);
  expect(onNext).not.toHaveBeenCalled();
});

test('reader shortcut actions fire when modal is closed', () => {
  const onNext = vi.fn();
  const onPrev = vi.fn();
  const onToggleStar = vi.fn();
  const onMarkRead = vi.fn();
  const onToggleFocus = vi.fn();

  render(
    <Harness
      onNext={onNext}
      onPrev={onPrev}
      onToggleStar={onToggleStar}
      onMarkRead={onMarkRead}
      onToggleFocus={onToggleFocus}
    />,
  );

  expect(dispatchKey('l')).toBe(true);
  expect(dispatchKey('h')).toBe(true);
  expect(dispatchKey('arrowright')).toBe(true);
  expect(dispatchKey('arrowleft')).toBe(true);
  expect(dispatchKey('s')).toBe(true);
  expect(dispatchKey('r')).toBe(true);
  expect(dispatchKey('f')).toBe(true);
  // j/k are no longer navigation (they become body scroll, handled by ArticleReader)
  expect(dispatchKey('j')).toBe(false);
  expect(dispatchKey('k')).toBe(false);

  expect(onNext).toHaveBeenCalledTimes(2); // l + arrowright
  expect(onPrev).toHaveBeenCalledTimes(2); // h + arrowleft
  expect(onToggleStar).toHaveBeenCalledTimes(1);
  expect(onMarkRead).toHaveBeenCalledTimes(1);
  expect(onToggleFocus).toHaveBeenCalledTimes(1);
});
