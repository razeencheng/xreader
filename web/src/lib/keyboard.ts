'use client';

type ShortcutHandler = () => void;

const SHORTCUT_TIMEOUT_MS = 1000;
const shortcuts = new Map<string, ShortcutHandler>();
let pendingPrefix: string | null = null;
let pendingTimeout: ReturnType<typeof setTimeout> | null = null;
let listenerAttached = false;

function normalizeKey(key: string) {
  const normalized = key.trim().toLowerCase();
  return normalized === 'esc' ? 'escape' : normalized;
}

function normalizeShortcut(key: string) {
  return key
    .trim()
    .split(/\s+/)
    .filter(Boolean)
    .map((part) => normalizeKey(part))
    .join(' ');
}

function clearPending() {
  if (pendingTimeout) {
    clearTimeout(pendingTimeout);
    pendingTimeout = null;
  }

  pendingPrefix = null;
}

function schedulePending(prefix: string) {
  clearPending();
  pendingPrefix = prefix;
  pendingTimeout = setTimeout(() => {
    clearPending();
  }, SHORTCUT_TIMEOUT_MS);
}

function hasChordWithPrefix(prefix: string) {
  const match = `${prefix} `;
  for (const shortcut of shortcuts.keys()) {
    if (shortcut.startsWith(match)) {
      return true;
    }
  }

  return false;
}

function isEditableElement(element: Element | null) {
  if (!element) {
    return false;
  }

  if (!(element instanceof HTMLElement)) {
    return false;
  }

  return element.isContentEditable || ['INPUT', 'TEXTAREA', 'SELECT'].includes(element.tagName);
}

function shouldIgnoreShortcuts(target: EventTarget | null) {
  if (typeof document === 'undefined') {
    return true;
  }

  if (isEditableElement(document.activeElement)) {
    return true;
  }

  return target instanceof Element ? isEditableElement(target) : false;
}

function hasBrowserModifier(event: KeyboardEvent) {
  return event.metaKey || event.ctrlKey || event.altKey;
}

function getShortcutKey(event: KeyboardEvent) {
  if (event.shiftKey && event.key === '/') {
    return '?';
  }

  return event.key;
}

function dispatchFromRoot(key: string) {
  const directHandler = shortcuts.get(key);
  if (directHandler) {
    directHandler();
    return true;
  }

  if (hasChordWithPrefix(key)) {
    schedulePending(key);
    return true;
  }

  return false;
}

function dispatchNormalizedKey(key: string) {
  if (!key) {
    return false;
  }

  if (pendingPrefix) {
    const combo = `${pendingPrefix} ${key}`;
    clearPending();
    const comboHandler = shortcuts.get(combo);
    if (comboHandler) {
      comboHandler();
      return true;
    }
  }

  return dispatchFromRoot(key);
}

function ensureListener() {
  if (listenerAttached || typeof document === 'undefined') {
    return;
  }

  document.addEventListener('keydown', (event) => {
    if (shouldIgnoreShortcuts(event.target) || hasBrowserModifier(event)) {
      return;
    }

    if (dispatchKey(getShortcutKey(event))) {
      event.preventDefault();
    }
  });
  listenerAttached = true;
}

export function registerShortcut(key: string, handler: ShortcutHandler) {
  ensureListener();
  shortcuts.set(normalizeShortcut(key), handler);
}

export function unregisterShortcut(key: string) {
  const normalized = normalizeShortcut(key);
  shortcuts.delete(normalized);

  if (pendingPrefix === normalized) {
    clearPending();
  }
}

export function dispatchKey(key: string) {
  if (shouldIgnoreShortcuts(null)) {
    return false;
  }

  return dispatchNormalizedKey(normalizeKey(key));
}
