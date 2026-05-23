import type { Layout } from '@/stores/useUIStore';

export function getActiveReaderLayout(layout: Layout, focusMode: boolean): Layout {
  if (focusMode) {
    return 'focus';
  }

  return layout === 'focus' ? 'classic' : layout;
}

export function applyReaderLayoutSelection(
  layout: Layout,
  setLayout: (layout: Layout) => void,
  setFocusMode: (focusMode: boolean) => void,
) {
  if (layout === 'focus') {
    setLayout('focus');
    setFocusMode(true);
    return;
  }

  setLayout(layout);
  setFocusMode(false);
}

export function toggleReaderFocusMode(
  focusMode: boolean,
  layout: Layout,
  setLayout: (layout: Layout) => void,
  setFocusMode: (focusMode: boolean) => void,
) {
  if (focusMode) {
    setFocusMode(false);
    if (layout === 'focus') {
      setLayout('classic');
    }
    return;
  }

  setLayout('focus');
  setFocusMode(true);
}
