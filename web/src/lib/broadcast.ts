'use client';

const CHANNEL_NAME = 'xreader';
const LOCAL_CHANGE_TTL_MS = 2 * 60 * 1000;

export interface StateChangeMessage {
  type: 'state-change';
  articleId: number;
  is_read?: boolean;
  is_starred?: boolean;
  origin: string;
}

export type StateChangePayload = Omit<StateChangeMessage, 'origin'>;

let channel: BroadcastChannel | null = null;
const tabOrigin = createTabOrigin();
const localChanges = new Map<string, number>();

function createTabOrigin() {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID();
  }

  return `tab-${Math.random().toString(36).slice(2)}`;
}

function getChannel() {
  if (typeof BroadcastChannel === 'undefined') {
    return null;
  }

  if (!channel) {
    channel = new BroadcastChannel(CHANNEL_NAME);
  }

  return channel;
}

function stateChangeKey(change: Pick<StateChangePayload, 'articleId' | 'is_read' | 'is_starred'>) {
  return JSON.stringify([change.articleId, change.is_read ?? null, change.is_starred ?? null]);
}

function pruneLocalChanges() {
  const now = Date.now();

  for (const [key, timestamp] of localChanges.entries()) {
    if (now - timestamp > LOCAL_CHANGE_TTL_MS) {
      localChanges.delete(key);
    }
  }
}

function rememberLocalChange(change: StateChangePayload) {
  pruneLocalChanges();
  localChanges.set(stateChangeKey(change), Date.now());
}

export function wasLocallyBroadcast(
  change: Pick<StateChangePayload, 'articleId' | 'is_read' | 'is_starred'>,
) {
  pruneLocalChanges();
  return localChanges.has(stateChangeKey(change));
}

export function consumeLocalBroadcast(
  change: Pick<StateChangePayload, 'articleId' | 'is_read' | 'is_starred'>,
) {
  pruneLocalChanges();
  const key = stateChangeKey(change);
  const exists = localChanges.has(key);
  localChanges.delete(key);
  return exists;
}

export function broadcast(msg: StateChangePayload) {
  rememberLocalChange(msg);

  const ch = getChannel();
  ch?.postMessage({ ...msg, origin: tabOrigin });
}

export function subscribe(callback: (msg: StateChangeMessage) => void) {
  const ch = getChannel();
  if (!ch) {
    return () => undefined;
  }

  const listener = (event: MessageEvent<StateChangeMessage>) => {
    const data = event.data;
    if (!data || data.type !== 'state-change') {
      return;
    }

    if (data.origin === tabOrigin) {
      return;
    }

    callback(data);
  };

  ch.addEventListener('message', listener);
  return () => ch.removeEventListener('message', listener);
}

export function close() {
  channel?.close();
  channel = null;
  localChanges.clear();
}

export function getBroadcastOrigin() {
  return tabOrigin;
}
