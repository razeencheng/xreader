import { fireEvent, render, screen } from '@testing-library/react';
import { useRef } from 'react';
import { useReaderGestures } from './useReaderGestures';

function Harness({
  progress,
  hasNext = true,
  hasPrev = true,
  onNext = vi.fn(),
  onPrev = vi.fn(),
  onBack = vi.fn(),
}: {
  progress: number;
  hasNext?: boolean;
  hasPrev?: boolean;
  onNext?: () => void;
  onPrev?: () => void;
  onBack?: () => void;
}) {
  const scrollRef = useRef<HTMLDivElement>(null);
  const { touchHandlers } = useReaderGestures({
    scrollRef,
    progress,
    hasNext,
    hasPrev,
    onNext,
    onPrev,
    onBack,
  });

  return (
    <div ref={scrollRef} data-testid="reader-surface" {...touchHandlers}>
      Reader
    </div>
  );
}

function swipe(element: HTMLElement, from: { x: number; y: number }, to: { x: number; y: number }) {
  fireEvent.touchStart(element, { touches: [{ clientX: from.x, clientY: from.y }] });
  fireEvent.touchMove(element, { touches: [{ clientX: to.x, clientY: to.y }] });
  fireEvent.touchEnd(element, { changedTouches: [{ clientX: to.x, clientY: to.y }] });
}

test('swiping up near the end opens the next article', () => {
  const onNext = vi.fn();
  render(<Harness progress={0.96} onNext={onNext} />);

  swipe(screen.getByTestId('reader-surface'), { x: 180, y: 520 }, { x: 178, y: 360 });

  expect(onNext).toHaveBeenCalledTimes(1);
});

test('swiping up in the middle of the article keeps normal scrolling behavior', () => {
  const onNext = vi.fn();
  render(<Harness progress={0.5} onNext={onNext} />);

  swipe(screen.getByTestId('reader-surface'), { x: 180, y: 520 }, { x: 178, y: 360 });

  expect(onNext).not.toHaveBeenCalled();
});

test('swiping down near the beginning opens the previous article', () => {
  const onPrev = vi.fn();
  render(<Harness progress={0.02} onPrev={onPrev} />);

  swipe(screen.getByTestId('reader-surface'), { x: 180, y: 180 }, { x: 178, y: 340 });

  expect(onPrev).toHaveBeenCalledTimes(1);
});

test('swiping left returns to the list', () => {
  const onBack = vi.fn();
  render(<Harness progress={0.5} onBack={onBack} />);

  swipe(screen.getByTestId('reader-surface'), { x: 280, y: 420 }, { x: 120, y: 410 });

  expect(onBack).toHaveBeenCalledTimes(1);
});
