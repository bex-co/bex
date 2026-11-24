import { DefaultChatTransport, type UIMessage, type UIMessageChunk } from "ai";
import { config } from "@/config/config";

// The chat transport for the agent-session conversation column (ADR047 D9 +
// w3/m43). It points `useChat` at the same-origin stream endpoint
// `api.bex.co/v1/agent-sessions/{id}/stream` — GET replays the durable
// transcript then live-tails (terminal sessions: replay + `[DONE]`), POST
// submits a live prompt turn — and authenticates every connection with a
// short-lived attach ticket carried in the `X-Bex-Agent-Ticket` header.
//
// The 90s ticket TTL makes a fresh mint mandatory per connection: each send
// (POST) and each reconnect (GET) mints exactly one ticket (each mint is a real
// `attach` mutation). The ticket rides a header, never the URL (web-shell
// precedent, D3 invariant #2). Cookies are irrelevant on this path —
// `credentials: "omit"` — the ticket is the sole credential.
//
// A conversation column holds at most one open GET (w6/171). The gateway counts
// each open stream against the person's concurrent sessions
// (`BEX_SSH_MAX_SESSIONS_PER_IDENTITY`, shared with SSH and the web shell), and
// a live splice ends only when the client hangs up or at the 4-hour cap. The
// SDK's own reconnect passes no abort signal, so a stream it opened stayed open
// after an in-place re-attach replaced it, or after the column unmounted. The
// reconnect here ends the stream it replaces before it mints, and `close()` ends
// the last one.

/** The header the m43 gateway attach listener verifies the attach ticket in. */
export const AGENT_TICKET_HEADER = "X-Bex-Agent-Ticket";

/** A freshly minted attach ticket (t001's attach/resume/steer mint verbs). */
export interface MintedTicket {
  ticket: string;
  /**
   * The mint's server-authoritative phase-1 SSE stream URL, if the backend has
   * BEX_API_PUBLIC_URL configured (w10/m9 t003, w3/013) — else the
   * config-derived one below. NOT the same as the mint's `url` field, which is
   * the phase-2 raw-ACP WebSocket gateway origin and is never a stream
   * endpoint; do not read `url` for this purpose.
   */
  streamUrl?: string | null;
}

export interface AgentSessionTransportOptions {
  sessionId: string;
  /**
   * Mints a fresh 90s attach ticket. Wired to t001's `attach` mutation in the
   * component; injected as a spy in tests. Called once per connection (send or
   * reconnect) so a stale ticket is never reused.
   */
  mintTicket: () => Promise<MintedTicket>;
  /** Override the endpoint (tests / a mint-advertised URL); else config-derived. */
  api?: string;
  /** Injectable fetch — the default transport uses `globalThis.fetch`. */
  fetch?: typeof globalThis.fetch;
}

/**
 * The same-origin m43 stream endpoint for a session. Same origin as bex-api's
 * REST/SSE surface (`config.apiBaseUrl`), so there is no second origin in
 * dashboard config and no CORS to configure on the dashboard side.
 */
export function agentSessionStreamUrl(sessionId: string): string {
  return `${config.agentStreamBaseUrl}/v1/agent-sessions/${encodeURIComponent(sessionId)}/stream`;
}

/**
 * The session's stream transport: `DefaultChatTransport` for the POST send,
 * with a reconnect GET that ends the one it replaces (see the file comment).
 */
export class AgentSessionTransport extends DefaultChatTransport<UIMessage> {
  readonly #ticketHeaders: () => Promise<Record<string, string>>;
  #live: AbortController | undefined;

  constructor(options: AgentSessionTransportOptions) {
    const ticketHeaders = async (): Promise<Record<string, string>> => {
      const minted = await options.mintTicket();
      return { [AGENT_TICKET_HEADER]: minted.ticket };
    };
    super({
      api: options.api ?? agentSessionStreamUrl(options.sessionId),
      // The ticket is the whole credential; the gateway ignores cookies on this
      // path, and sending them cross-subdomain would only add ambient authority.
      credentials: "omit",
      fetch: options.fetch,
      // Resolved per request — a fresh ticket for every POST send.
      headers: ticketHeaders,
    });
    this.#ticketHeaders = ticketHeaders;
  }

  /**
   * GET the session's stream (replay, then the live tail), after ending the
   * stream this transport opened before. One replaced before it connected
   * resolves to `null`, the SDK's "nothing to resume", never to an error.
   */
  override async reconnectToStream(
    _options: Parameters<
      DefaultChatTransport<UIMessage>["reconnectToStream"]
    >[0],
  ): Promise<ReadableStream<UIMessageChunk> | null> {
    this.close();
    const live = new AbortController();
    this.#live = live;
    try {
      const response = await (this.fetch ?? globalThis.fetch)(this.api, {
        method: "GET",
        headers: await this.#ticketHeaders(),
        credentials: "omit",
        signal: live.signal,
      });
      if (response.status === 204) return null;
      if (!response.ok) throw new Error(await response.text());
      if (!response.body) throw new Error("The response body is empty.");
      return this.processResponseStream(response.body);
    } catch (err) {
      if (live.signal.aborted) return null;
      throw err;
    }
  }

  /** Ends the open stream, if any. The column calls it when it unmounts. */
  close(): void {
    this.#live?.abort();
    this.#live = undefined;
  }
}

/**
 * Builds the transport for a session. Kept as a plain factory (no React) so it
 * is unit-testable with an injected `fetch` and `mintTicket`, and so the
 * component can construct it lazily on the client only.
 */
export function createAgentSessionTransport(
  options: AgentSessionTransportOptions,
): AgentSessionTransport {
  return new AgentSessionTransport(options);
}
