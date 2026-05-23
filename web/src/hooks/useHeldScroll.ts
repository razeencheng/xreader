'use client';

import { useEffect, type RefObject } from 'react';

interface HeldScrollOptions {
  /** When true the hook is inert (e.g. shortcuts modal open). */
  disabled?: boolean;
}

// DOWN_KEYS/UP_KEYS must stay non-default-scroll keys (j/k): browser native
// scroll keys (arrows/space/PageUp/PageDown) would native-scroll on held-key
// repeats since we only preventDefault the first keydown.
const DOWN_KEYS = new Set(['j']);
const UP_KEYS = new Set(['k']);

// Per-frame velocity (px). Starts gentle, ramps up the longer the key is held,
// like holding an arrow key in an editor. Tuned for ~60fps.
const BASE_VELOCITY = 11;
const MAX_VELOCITY = 34;
const RAMP_MS = 380;

function isEditableTarget(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false;
  return target.isContentEditable || ['INPUT', 'TEXTAREA', 'SELECT'].includes(target.tagName);
}

/**
 * Continuous hold-to-scroll for a scrollable element: hold J to glide down,
 * K to glide up; release to stop. A quick tap nudges by one step.
 * Bound to document keydown/keyup (the global keyboard.ts registry is
 * keydown-only and cannot express press-and-hold).
 */
export function useHeldScroll(
  ref: RefObject<HTMLElement | null>,
  { disabled = false }: HeldScrollOptions = {},
) {
  useEffect(() => {
    if (disabled) return;

    const held = new Set<string>(); // currently-held scroll keys ('j' / 'k')
    let direction = 0;
    let rafId: number | null = null;
    let pressStartedAt = 0;

    const now = () => (typeof performance !== 'undefined' ? performance.now() : Date.now());

    const applyScroll = (dir: number, velocity: number) => {
      const el = ref.current;
      if (!el) return;
      const max = el.scrollHeight - el.clientHeight;
      const next = el.scrollTop + dir * velocity;
      el.scrollTop = next < 0 ? 0 : next > max ? max : next;
    };

    const step = () => {
      if (direction === 0) {
        rafId = null;
        return;
      }
      const ramp = Math.min(1, (now() - pressStartedAt) / RAMP_MS);
      applyScroll(direction, BASE_VELOCITY + (MAX_VELOCITY - BASE_VELOCITY) * ramp);
      rafId = requestAnimationFrame(step);
    };

    // Begin (or redirect) the glide. Called only on a genuine key-state
    // change, so the immediate tap nudge never double-fires on duplicate
    // keydowns of an already-held key.
    const drive = (dir: number) => {
      applyScroll(dir, BASE_VELOCITY); // immediate nudge → a quick tap always moves
      direction = dir;
      pressStartedAt = now();
      if (rafId === null) {
        rafId = requestAnimationFrame(step);
      }
    };

    const stop = () => {
      direction = 0;
      if (rafId !== null) {
        cancelAnimationFrame(rafId);
        rafId = null;
      }
    };

    // Re-derive motion from the still-held keys (last-press semantics:
    // whichever scroll key remains held wins; none held → stop).
    const settle = () => {
      if (held.has('j')) drive(1);
      else if (held.has('k')) drive(-1);
      else stop();
    };

    const onKeyDown = (event: KeyboardEvent) => {
      if (event.repeat) return;
      if (event.metaKey || event.ctrlKey || event.altKey) return;
      if (isEditableTarget(event.target) || isEditableTarget(document.activeElement)) return;
      const key = event.key.toLowerCase();
      if (!DOWN_KEYS.has(key) && !UP_KEYS.has(key)) return;
      event.preventDefault();
      if (held.has(key)) return; // ignore duplicate keydown of an already-held key (no extra nudge)
      held.add(key);
      drive(DOWN_KEYS.has(key) ? 1 : -1);
    };

    const onKeyUp = (event: KeyboardEvent) => {
      const key = event.key.toLowerCase();
      if (!held.delete(key)) return;
      settle(); // fall back to the other held key, or stop if none
    };

    const onBlur = () => {
      held.clear();
      stop();
    };

    document.addEventListener('keydown', onKeyDown);
    document.addEventListener('keyup', onKeyUp);
    window.addEventListener('blur', onBlur);

    return () => {
      held.clear();
      stop();
      document.removeEventListener('keydown', onKeyDown);
      document.removeEventListener('keyup', onKeyUp);
      window.removeEventListener('blur', onBlur);
    };
  }, [ref, disabled]);
}
