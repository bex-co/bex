import { DefaultChatTransport, type UIMessage } from "ai";
import { mobileConfig } from "../../auth/config";

export const AGENT_TICKET_HEADER = "X-Bex-Agent-Ticket";
export const AGENT_TURN_IDEMPOTENCY_HEADER = "Idempotency-Key";

// Match the mobile log stream's defensive ceiling: a broken or hostile peer
// cannot grow an unterminated SSE frame forever while the chat remains open.
export const MAX_PENDING_AGENT_EVENT_BYTES = 1024 * 1024;

export type AgentTicketAction = "read" | "turn";

export type MintedAgentTicket = {
  ticket: string;
  url?: string | null;
  expiresAt?: string | null;
};

export type AgentSessionTransportOptions = {
  sessionId: string;
  mintTicket: (action: AgentTicketAction) => Promise<MintedAgentTicket>;
  /** `expo/fetch` in production; injectable so protocol tests stay native-free. */
  fetch: typeof globalThis.fetch;
  initialCursor?: number;
  api?: string;
};

export function agentSessionStreamUrl(sessionId: string): string {
  return `${mobileConfig.apiOrigin}/v1/agent-sessions/${encodeURIComponent(sessionId)}/stream`;
}

/**
 * AI SDK 6 transport for the gateway's UI-message stream.
 *
 * The gateway ticket is the only stream credential, so requests omit ambient
 * cookies and OAuth headers. Every network connection mints a new action-bound
 * ticket: `read` for GET replay/live-tail and `turn` for POST. Durable SSE ids
 * advance an in-memory cursor; reconnect asks only for the later tail and drops
 * a replayed boundary event. Tickets and cursors are never persisted or logged.
 */
export function createAgentSessionTransport(
  options: AgentSessionTransportOptions,
): DefaultChatTransport<UIMessage> {
  const api = options.api ?? agentSessionStreamUrl(options.sessionId);
  let cursor = normalizeInitialCursor(options.initialCursor);
  const cursorFetch = observeAgentCursor(
    options.fetch,
    (seq) => {
      cursor = Math.max(cursor, seq);
    },
    () => cursor,
  );

  return new DefaultChatTransport<UIMessage>({
    api,
    credentials: "omit",
    fetch: cursorFetch,
    prepareSendMessagesRequest: async ({
      id,
      messages,
      body,
      trigger,
      messageId,
    }) => {
      const turnKey =
        [...messages].reverse().find((message) => message.role === "user")
          ?.id ?? messageId;
      if (!turnKey) {
        throw new Error("agent turn requires a stable user message id");
      }
      const minted = await options.mintTicket("turn");
      requireTicket(minted);
      return {
        api,
        headers: {
          [AGENT_TICKET_HEADER]: minted.ticket,
          [AGENT_TURN_IDEMPOTENCY_HEADER]: turnKey,
        },
        body: { ...body, id, messages, trigger, messageId },
      };
    },
    prepareReconnectToStreamRequest: async () => {
      const minted = await options.mintTicket("read");
      requireTicket(minted);
      return {
        api: withAfterSeq(api, cursor),
        headers: { [AGENT_TICKET_HEADER]: minted.ticket },
      };
    },
  });
}

function requireTicket(minted: MintedAgentTicket): void {
  if (!minted.ticket.trim()) {
    throw new Error("attachAgentSession returned no ticket");
  }
}

function normalizeInitialCursor(cursor: number | undefined): number {
  if (cursor === undefined) return -1;
  if (!Number.isSafeInteger(cursor) || cursor < -1) {
    throw new Error("agent conversation returned an invalid cursor");
  }
  return cursor;
}

export function withAfterSeq(api: string, cursor: number): string {
  const url = new URL(api);
  url.searchParams.set("afterSeq", String(cursor));
  return url.toString();
}

export function observeAgentCursor(
  fetchImpl: typeof globalThis.fetch,
  onCursor: (seq: number) => void,
  currentCursor: () => number,
): typeof globalThis.fetch {
  return (async (input: RequestInfo | URL, init?: RequestInit) => {
    const response = await fetchImpl(input, init);
    if (!response.ok || !response.body) return response;

    const decoder = new TextDecoder();
    const encoder = new TextEncoder();
    let pending = "";
    const observed = response.body.pipeThrough(
      new TransformStream<Uint8Array, Uint8Array>({
        transform(chunk, controller) {
          pending += decoder.decode(chunk, { stream: true });
          emitCompleteEvents(controller);
          if (pending.length > MAX_PENDING_AGENT_EVENT_BYTES) {
            throw new Error("agent stream returned an oversized event frame");
          }
        },
        flush(controller) {
          pending += decoder.decode();
          if (pending.length > MAX_PENDING_AGENT_EVENT_BYTES) {
            throw new Error("agent stream returned an oversized event frame");
          }
          if (pending) emitEvent(pending, controller);
        },
      }),
    );

    function emitCompleteEvents(
      controller: TransformStreamDefaultController<Uint8Array>,
    ): void {
      for (;;) {
        const boundary = /\r?\n\r?\n/.exec(pending);
        if (!boundary || boundary.index == null) return;
        const end = boundary.index + boundary[0].length;
        const event = pending.slice(0, end);
        pending = pending.slice(end);
        emitEvent(event, controller);
      }
    }

    function emitEvent(
      event: string,
      controller: TransformStreamDefaultController<Uint8Array>,
    ): void {
      const idLine = event
        .split(/\r?\n/)
        .find((line) => line.startsWith("id:"));
      if (!idLine) {
        controller.enqueue(encoder.encode(event));
        return;
      }
      const seq = Number(idLine.slice(3).trim());
      if (!Number.isSafeInteger(seq) || seq < 0) {
        throw new Error("agent stream returned an invalid cursor");
      }
      if (seq <= currentCursor()) return;
      onCursor(seq);
      controller.enqueue(encoder.encode(event));
    }

    return new Response(observed, {
      status: response.status,
      statusText: response.statusText,
      headers: response.headers,
    });
  }) as typeof globalThis.fetch;
}
