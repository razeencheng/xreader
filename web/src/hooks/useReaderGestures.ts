'use client';

import { useCallback, useMemo, useRef, useState, type RefObject, type TouchEvent } from 'react';

export type ReaderGestureHint = 'next' | 'previous' | 'back' | 'nextUnavailable' | 'previousUnavailable' | null;

interface UseReaderGesturesOptions {
  scrollRef: RefObject<HTMLElement | null>;
  progress: number;
  hasNext: boolean;
  hasPrev: boolean;
  onNext: () => void;
  onPrev: () => void;
  onBack: () => void;
  disabled?: boolean;
}

type TouchPoint = {
  x: number;
  y: number;
};

type TouchLike = {
  clientX: number;
  clientY: number;
};

const EDGE_PROGRESS = 0.08;
const DISTANCE_THRESHOLD = 72;
const HORIZONTAL_DOMINANCE = 1.35;
const VERTICAL_DOMINANCE = 1.2;

function getPoint(touch: TouchLike | undefined): TouchPoint | null {
  if (!touch) return null;
  return { x: touch.clientX, y: touch.clientY };
}

function shouldIgnoreTarget(target: EventTarget | null) {
  if (!(target instanceof Element)) return false;

  return Boolean(
    target.closest('a, button, input, textarea, select, [contenteditable="true"], [data-reader-gesture-ignore="true"]'),
  );
}

function hasTextSelection() {
  if (typeof window === 'undefined' || typeof window.getSelection !== 'function') return false;
  return Boolean(window.getSelection()?.toString().trim());
}

export function useReaderGestures({
  scrollRef,
  progress,
  hasNext,
  hasPrev,
  onNext,
  onPrev,
  onBack,
  disabled = false,
}: UseReaderGesturesOptions) {
  const startPointRef = useRef<TouchPoint | null>(null);
  const hintTimerRef = useRef<number | null>(null);
  const [gestureHint, setGestureHint] = useState<ReaderGestureHint>(null);

  const clearHintTimer = useCallback(() => {
    if (hintTimerRef.current == null) return;
    window.clearTimeout(hintTimerRef.current);
    hintTimerRef.current = null;
  }, []);

  const showHint = useCallback(
    (hint: ReaderGestureHint, timeout = 900) => {
      setGestureHint(hint);
      clearHintTimer();

      if (hint) {
        hintTimerRef.current = window.setTimeout(() => {
          setGestureHint(null);
          hintTimerRef.current = null;
        }, timeout);
      }
    },
    [clearHintTimer],
  );

  const getBoundaryState = useCallback(() => {
    const element = scrollRef.current;
    const hasScrollableMetrics = Boolean(element && element.scrollHeight > element.clientHeight);

    if (!element || !hasScrollableMetrics) {
      return {
        atTop: progress <= EDGE_PROGRESS,
        atBottom: progress >= 1 - EDGE_PROGRESS,
      };
    }

    const remaining = element.scrollHeight - element.clientHeight - element.scrollTop;
    return {
      atTop: progress <= EDGE_PROGRESS || element.scrollTop <= 32,
      atBottom: progress >= 1 - EDGE_PROGRESS || remaining <= 32,
    };
  }, [progress, scrollRef]);

  const resolveGesture = useCallback(
    (point: TouchPoint): ReaderGestureHint => {
      const start = startPointRef.current;
      if (!start) return null;

      const dx = point.x - start.x;
      const dy = point.y - start.y;
      const absX = Math.abs(dx);
      const absY = Math.abs(dy);

      if (dx < -DISTANCE_THRESHOLD && absX > absY * HORIZONTAL_DOMINANCE) {
        return 'back';
      }

      if (absY < DISTANCE_THRESHOLD || absY < absX * VERTICAL_DOMINANCE) {
        return null;
      }

      const { atTop, atBottom } = getBoundaryState();

      if (dy < -DISTANCE_THRESHOLD && atBottom) {
        return hasNext ? 'next' : 'nextUnavailable';
      }

      if (dy > DISTANCE_THRESHOLD && atTop) {
        return hasPrev ? 'previous' : 'previousUnavailable';
      }

      return null;
    },
    [getBoundaryState, hasNext, hasPrev],
  );

  const touchHandlers = useMemo(
    () => ({
      onTouchStart(event: TouchEvent<HTMLElement>) {
        if (disabled || shouldIgnoreTarget(event.target) || hasTextSelection()) {
          startPointRef.current = null;
          return;
        }

        startPointRef.current = getPoint(event.touches[0]);
        showHint(null);
      },
      onTouchMove(event: TouchEvent<HTMLElement>) {
        if (disabled) return;
        const point = getPoint(event.touches[0]);
        if (!point) return;

        const hint = resolveGesture(point);
        if (hint) {
          showHint(hint, 650);
        }
      },
      onTouchEnd(event: TouchEvent<HTMLElement>) {
        if (disabled) return;
        const point = getPoint(event.changedTouches[0]);
        if (!point) return;

        const gesture = resolveGesture(point);
        startPointRef.current = null;

        if (gesture === 'next') {
          showHint(null);
          onNext();
          return;
        }

        if (gesture === 'previous') {
          showHint(null);
          onPrev();
          return;
        }

        if (gesture === 'back') {
          showHint(null);
          onBack();
          return;
        }

        if (gesture === 'nextUnavailable' || gesture === 'previousUnavailable') {
          showHint(gesture, 1000);
          return;
        }

        showHint(null);
      },
      onTouchCancel() {
        startPointRef.current = null;
        showHint(null);
      },
    }),
    [disabled, onBack, onNext, onPrev, resolveGesture, showHint],
  );

  return { gestureHint, touchHandlers };
}
