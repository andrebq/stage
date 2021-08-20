import {readable} from 'svelte/store';
import type {Readable} from 'svelte/store';

type MessageHandler = (payload: any) => void;

const maxBackoff = 10 * 60 * 1000;

function stageConnection(url: string, msgHandler: MessageHandler): () => void {
  let ws: WebSocket;

  let backoff = 1000;
  let attempt = 0;
  let intentional = false;

  let retryScheduled = false;
  const scheduleRetry = (fn: () => void, timeout: number) => {
    if (retryScheduled) {
      return;
    }
    setTimeout(() => {
      retryScheduled = false;
      fn();
    }, timeout);
    retryScheduled = true;
  };

  const openHandler = (ev: Event) => {
    console.info('stage connected to', url, ev);
    backoff = 1000;
  };
  const errorHandler = (ev: Event) => {
    console.error('error on stage', url, ev, 'attempt', attempt);
    backoff = Math.min(maxBackoff, backoff * 1.5);
    scheduleRetry(() => connect(), backoff);
  };
  const closeHandler = (ev: Event) => {
    console.info('connection closed', url, ev);
    if (!intentional) {
      backoff = Math.min(maxBackoff, backoff * 1.5);
      scheduleRetry(() => connect(), backoff);
    }
  };

  function connect() {
    ws = new WebSocket(url);
    attempt++;
    ws.onopen = openHandler;
    ws.onerror = errorHandler;
    ws.onmessage = ev => {
      msgHandler(ev.data);
    };
    ws.onclose = closeHandler;
  }

  connect();

  return () => {
    intentional = true;
    ws.close();
  };
}

export function connect(bridge: string, actorID: string): Readable<any> {
  const store = readable(null, set => {
    const done = stageConnection(
      bridge + '?actorid=' + encodeURIComponent(actorID),
      stageMessage => {
        try {
          const val = JSON.parse(stageMessage);
          set(val);
        } catch (ex: any) {
          console.error('invalid data from stage', ex);
        }
      }
    );
    () => done();
  });
  return store;
}
