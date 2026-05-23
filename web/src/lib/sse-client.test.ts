import { afterEach, beforeEach, expect, test, vi } from 'vitest';
import { createSSEClient, type SSEParagraphEvent } from './sse-client';

type EventListener = (event: Event) => void;

class FakeEventSource {
  static nextParagraphOnAttach: SSEParagraphEvent | null = null;
  static instances: FakeEventSource[] = [];

  private listeners = new Map<string, Set<EventListener>>();

  constructor(
    readonly url: string,
    readonly options?: { withCredentials?: boolean },
  ) {
    FakeEventSource.instances.push(this);
  }

  addEventListener(event: string, listener: EventListener) {
    const listeners = this.listeners.get(event) ?? new Set<EventListener>();
    listeners.add(listener);
    this.listeners.set(event, listeners);

    if (event === 'paragraph' && FakeEventSource.nextParagraphOnAttach) {
      const payload = FakeEventSource.nextParagraphOnAttach;
      FakeEventSource.nextParagraphOnAttach = null;
      listener(new MessageEvent('paragraph', { data: JSON.stringify(payload) }));
    }
  }

  removeEventListener(event: string, listener: EventListener) {
    this.listeners.get(event)?.delete(listener);
  }

  close() {}
}

const OriginalEventSource = globalThis.EventSource;

beforeEach(() => {
  FakeEventSource.instances = [];
  FakeEventSource.nextParagraphOnAttach = null;
  vi.stubGlobal('EventSource', FakeEventSource as unknown as typeof EventSource);
});

afterEach(() => {
  vi.unstubAllGlobals();
  if (OriginalEventSource) {
    vi.stubGlobal('EventSource', OriginalEventSource);
  }
});

test('delivers early paragraph events after listeners are registered', async () => {
  const paragraph = { index: 0, translation: '第一段' };
  FakeEventSource.nextParagraphOnAttach = paragraph;

  const onParagraph = vi.fn();
  const onDone = vi.fn();
  const onError = vi.fn();

  const client = createSSEClient('/api/articles/1/body-translation');
  client.onParagraph(onParagraph);
  client.onDone(onDone);
  client.onError(onError);

  await Promise.resolve();

  expect(FakeEventSource.instances).toHaveLength(1);
  expect(FakeEventSource.instances[0]?.url).toBe('/api/articles/1/body-translation');
  expect(onParagraph).toHaveBeenCalledWith(paragraph);
});
