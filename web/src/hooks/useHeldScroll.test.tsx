import { renderHook } from '@testing-library/react';
import { act } from 'react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { useRef } from 'react';
import { useHeldScroll } from './useHeldScroll';

let rafCbs: FrameRequestCallback[] = [];
let rafId = 0;
function flushFrames(n: number, msPerFrame = 16) {
  for (let i = 0; i < n; i++) {
    const cbs = rafCbs;
    rafCbs = [];
    const ts = (i + 1) * msPerFrame;
    act(() => cbs.forEach((cb) => cb(ts)));
  }
}

beforeEach(() => {
  rafCbs = [];
  rafId = 0;
  vi.stubGlobal('requestAnimationFrame', (cb: FrameRequestCallback) => {
    rafCbs.push(cb);
    return ++rafId;
  });
  vi.stubGlobal('cancelAnimationFrame', () => {});
});
afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

function setup(disabled = false) {
  const el = document.createElement('div');
  Object.defineProperty(el, 'scrollHeight', { value: 5000, configurable: true });
  Object.defineProperty(el, 'clientHeight', { value: 800, configurable: true });
  el.scrollTop = 0;
  const { unmount, rerender } = renderHook(
    ({ d }: { d: boolean }) => {
      const ref = useRef<HTMLDivElement | null>(el);
      useHeldScroll(ref, { disabled: d });
    },
    { initialProps: { d: disabled } },
  );
  return { el, unmount, rerender };
}

function keydown(key: string, repeat = false) {
  act(() => document.dispatchEvent(new KeyboardEvent('keydown', { key, repeat, cancelable: true, bubbles: true })));
}
function keyup(key: string) {
  act(() => document.dispatchEvent(new KeyboardEvent('keyup', { key, bubbles: true })));
}

describe('useHeldScroll', () => {
  it('scrolls the element down while j is held, stops on keyup', () => {
    const { el } = setup();
    keydown('j');
    flushFrames(3);
    expect(el.scrollTop).toBeGreaterThan(0);
    const afterHold = el.scrollTop;
    keyup('j');
    flushFrames(5);
    expect(el.scrollTop).toBe(afterHold);
  });

  it('scrolls up while k is held (clamped at 0)', () => {
    const { el } = setup();
    el.scrollTop = 1000;
    keydown('k');
    flushFrames(3);
    expect(el.scrollTop).toBeLessThan(1000);
    expect(el.scrollTop).toBeGreaterThanOrEqual(0);
  });

  it('a quick tap (keydown then keyup before any frame) still nudges', () => {
    const { el } = setup();
    keydown('j');
    keyup('j');
    flushFrames(3);
    expect(el.scrollTop).toBeGreaterThan(0);
  });

  it('ignores OS key-repeat keydown (guard returns before preventDefault)', () => {
    setup();
    const first = new KeyboardEvent('keydown', { key: 'j', cancelable: true, bubbles: true });
    act(() => document.dispatchEvent(first));
    expect(first.defaultPrevented).toBe(true);
    const repeat = new KeyboardEvent('keydown', { key: 'j', repeat: true, cancelable: true, bubbles: true });
    act(() => document.dispatchEvent(repeat));
    expect(repeat.defaultPrevented).toBe(false);
    keyup('j');
  });

  it('stops scrolling when disabled flips to true mid-hold', () => {
    const { el, rerender } = setup(false);
    keydown('j');
    flushFrames(2);
    const mid = el.scrollTop;
    expect(mid).toBeGreaterThan(0);
    rerender({ d: true });
    flushFrames(5);
    expect(el.scrollTop).toBe(mid);
  });

  it('does nothing when disabled', () => {
    const { el } = setup(true);
    keydown('j');
    flushFrames(5);
    expect(el.scrollTop).toBe(0);
  });

  it('does nothing when the event target is an editable element', () => {
    const { el } = setup();
    const input = document.createElement('input');
    document.body.appendChild(input);
    input.focus();
    act(() => input.dispatchEvent(new KeyboardEvent('keydown', { key: 'j', bubbles: true, cancelable: true })));
    flushFrames(5);
    expect(el.scrollTop).toBe(0);
    input.remove();
  });

  it('cancels the loop and key state on unmount', () => {
    const cancel = vi.fn();
    vi.stubGlobal('cancelAnimationFrame', cancel);
    const { el, unmount } = setup();
    keydown('j');
    flushFrames(1);
    unmount();
    const stopped = el.scrollTop;
    flushFrames(5);
    expect(el.scrollTop).toBe(stopped);
    expect(cancel).toHaveBeenCalled();
  });

  it('falls back to the other held key when one of two held keys is released', () => {
    const { el } = setup();
    keydown('j');
    flushFrames(2);
    const downPos = el.scrollTop;
    expect(downPos).toBeGreaterThan(0);
    keydown('k'); // both held; now scrolling up
    flushFrames(2);
    expect(el.scrollTop).toBeLessThan(downPos);
    const upPos = el.scrollTop;
    keyup('k'); // release k while j still held → must resume scrolling DOWN, not stop
    flushFrames(3);
    expect(el.scrollTop).toBeGreaterThan(upPos);
    keyup('j');
  });

  it('a duplicate non-repeat keydown of an already-held key does not add an extra nudge', () => {
    const { el } = setup();
    keydown('j'); // one immediate nudge, no frames flushed yet
    const afterFirst = el.scrollTop;
    expect(afterFirst).toBeGreaterThan(0);
    keydown('j'); // duplicate (repeat=false) — must be ignored, no second nudge
    expect(el.scrollTop).toBe(afterFirst);
    keyup('j');
  });
});
