export interface SSEParagraphEvent {
  index: number;
  original?: string;
  translation: string;
}

export interface SSEClient {
  onParagraph(callback: (paragraph: SSEParagraphEvent) => void): void;
  onDone(callback: () => void): void;
  onError(callback: (error: Event) => void): void;
  close(): void;
}

type EventCallback = () => void;

type ErrorCallback = (error: Event) => void;

type ParagraphCallback = (paragraph: SSEParagraphEvent) => void;

const RECONNECT_DELAY_MS = 500;

export function createSSEClient(url: string): SSEClient {
  const paragraphListeners = new Set<ParagraphCallback>();
  const doneListeners = new Set<EventCallback>();
  const errorListeners = new Set<ErrorCallback>();

  let source: EventSource | null = null;
  let closed = false;
  let reconnectAttempted = false;
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  let openScheduled = false;

  const cleanupSource = () => {
    if (!source) {
      return;
    }

    source.removeEventListener('paragraph', handleParagraph);
    source.removeEventListener('done', handleDone);
    source.removeEventListener('same-language', handleDone);
    source.removeEventListener('error', handleError);
    source.close();
    source = null;
  };

  const open = () => {
    if (closed) {
      return;
    }

    source = new EventSource(url, { withCredentials: true });
    source.addEventListener('paragraph', handleParagraph);
    source.addEventListener('done', handleDone);
    source.addEventListener('same-language', handleDone);
    source.addEventListener('error', handleError);
  };

  const scheduleOpen = () => {
    if (openScheduled || source || closed) {
      return;
    }

    openScheduled = true;
    queueMicrotask(() => {
      openScheduled = false;
      if (!source && !closed) {
        open();
      }
    });
  };

  const scheduleReconnect = () => {
    if (reconnectTimer || closed) {
      return;
    }

    reconnectTimer = setTimeout(() => {
      reconnectTimer = null;
      open();
    }, RECONNECT_DELAY_MS);
  };

  function handleParagraph(event: Event) {
    if (!('data' in event)) {
      return;
    }

    try {
      const paragraph = JSON.parse((event as MessageEvent<string>).data) as SSEParagraphEvent;
      paragraphListeners.forEach((listener) => listener(paragraph));
    } catch {
      // Ignore malformed paragraph payloads.
    }
  }

  function handleDone() {
    doneListeners.forEach((listener) => listener());
    close();
  }

  function handleError(event: Event) {
    errorListeners.forEach((listener) => listener(event));

    if (closed) {
      return;
    }

    cleanupSource();

    if (reconnectAttempted) {
      return;
    }

    reconnectAttempted = true;
    scheduleReconnect();
  }

  function close() {
    closed = true;

    if (reconnectTimer) {
      clearTimeout(reconnectTimer);
      reconnectTimer = null;
    }

    cleanupSource();
  }

  return {
    onParagraph(callback) {
      paragraphListeners.add(callback);
      scheduleOpen();
    },
    onDone(callback) {
      doneListeners.add(callback);
      scheduleOpen();
    },
    onError(callback) {
      errorListeners.add(callback);
      scheduleOpen();
    },
    close,
  };
}
