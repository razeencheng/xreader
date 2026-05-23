import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

class MockBroadcastChannel {
  name: string;
  listeners: Array<(event: MessageEvent) => void> = [];
  static instances: MockBroadcastChannel[] = [];
  sentMessages: unknown[] = [];

  constructor(name: string) {
    this.name = name;
    MockBroadcastChannel.instances.push(this);
  }

  postMessage(data: unknown) {
    this.sentMessages.push(data);
    MockBroadcastChannel.instances
      .filter((channel) => channel !== this && channel.name === this.name)
      .forEach((channel) => {
        channel.listeners.forEach((listener) => listener({ data } as MessageEvent));
      });
  }

  addEventListener(_: string, listener: (event: MessageEvent) => void) {
    this.listeners.push(listener);
  }

  removeEventListener(_: string, listener: (event: MessageEvent) => void) {
    this.listeners = this.listeners.filter((current) => current !== listener);
  }

  close() {}
}

describe('broadcast', () => {
  beforeEach(() => {
    MockBroadcastChannel.instances = [];
    vi.stubGlobal('BroadcastChannel', MockBroadcastChannel as unknown as typeof BroadcastChannel);
    vi.stubGlobal('crypto', {
      randomUUID: vi.fn(() => 'tab-origin'),
    } as unknown as Crypto);
    vi.resetModules();
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });


  it('broadcasts messages and receives them from other tabs', async () => {
    const { broadcast, subscribe, close, getBroadcastOrigin } = await import('./broadcast');

    const received: Array<{ articleId: number; origin: string }> = [];
    const unsubscribe = subscribe((msg) => {
      received.push({ articleId: msg.articleId, origin: msg.origin });
    });

    const otherTab = new MockBroadcastChannel('xreader');
    otherTab.addEventListener('message', (event) => {
      const data = event.data as { type: string; articleId: number; origin: string };
      if (data.type === 'state-change') {
        received.push({ articleId: data.articleId, origin: data.origin });
      }
    });

    otherTab.postMessage({
      type: 'state-change',
      articleId: 1,
      is_read: true,
      origin: 'other-tab',
    });

    expect(received).toEqual([{ articleId: 1, origin: 'other-tab' }]);

    broadcast({ type: 'state-change', articleId: 2, is_starred: true });
    expect(received).toContainEqual({ articleId: 2, origin: 'tab-origin' });
    expect(getBroadcastOrigin()).toBe('tab-origin');

    unsubscribe();
    close();
  });

  it('ignores messages from the same origin', async () => {
    const { subscribe, close } = await import('./broadcast');

    const received: Array<number> = [];
    const unsubscribe = subscribe((msg) => {
      received.push(msg.articleId);
    });

    const channel = MockBroadcastChannel.instances[0] ?? new MockBroadcastChannel('xreader');
    channel.postMessage({
      type: 'state-change',
      articleId: 3,
      is_read: false,
      origin: 'tab-origin',
    });

    expect(received).toEqual([]);

    unsubscribe();
    close();
  });
});
