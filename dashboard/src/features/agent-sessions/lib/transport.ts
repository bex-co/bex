import { DefaultChatTransport, type UIMessage } from "ai";
import { config } from "@/config/config";

// The chat transport for the agent-session conversation column (ADR047 D9 +
// w3/m43). It points `useChat` at the same-origin stream endpoint
// `api.bex.co/v1/agent-sessions/{id}/stream` — GET replays the durable
// transcript then live-tails (terminal sessions: replay + `[DONE]`), POST
// submits a live prompt turn — and authenticates every connection with a
// short-lived attach ticket carried in the `X-Bex-Agent-Ticket` header.
//
// The 90s ticket TTL makes a fresh mint mandatory per connection. The `headers`
// resolvable mints one ticket per request — HttpChatTransport re-resolves it on
// every send (POST) AND every reconnect (GET), so each connection carries a
// fresh ticket. `prepareReconnectToStreamRequest` is still required on the
// reconnect path, but only to pin the `api` URL: without it, the reconnect
// defaults to `${api}/${chatId}/stream`, double-suffixing our already-complete
// URL. It intentionally does NOT re-mint (that would mint twice per reconnect —
// each mint is a real `attach` mutation). The ticket rides a header, never the
// URL (web-shell precedent, D3 invariant #2). Cookies are irrelevant on this
// path — `credentials: "omit"` — the ticket is the sole credential.

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
 * Builds the `DefaultChatTransport` for a session. Kept as a plain factory (no
 * React) so it is unit-testable with an injected `fetch` and `mintTicket`, and
 * so the component can construct it lazily on the client only.
 */
export function createAgentSessionTransport(
  options: AgentSessionTransportOptions,
): DefaultChatTransport<UIMessage> {
  const api = options.api ?? agentSessionStreamUrl(options.sessionId);

  const ticketHeaders = async (): Promise<Record<string, string>> => {
    const minted = await options.mintTicket();
    return { [AGENT_TICKET_HEADER]: minted.ticket };
  };

  return new DefaultChatTransport<UIMessage>({
    api,
    // The ticket is the whole credential; the gateway ignores cookies on this
    // path, and sending them cross-subdomain would only add ambient authority.
    credentials: "omit",
    fetch: options.fetch,
    // Resolved per request — a fresh ticket for every POST send and every GET
    // reconnect (HttpChatTransport re-resolves `headers` on both paths).
    headers: ticketHeaders,
    // Pin the reconnect URL to our complete stream endpoint. Returning no
    // `headers` here lets the reconnect reuse the freshly minted `headers`
    // above (one mint per reconnect), rather than minting a second ticket.
    prepareReconnectToStreamRequest: () => ({ api }),
  });
}
