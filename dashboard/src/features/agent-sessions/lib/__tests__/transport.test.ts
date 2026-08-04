import { describe, expect, it, vi } from "vitest";
import {
  AGENT_TICKET_HEADER,
  agentSessionStreamUrl,
  createAgentSessionTransport,
} from "@/features/agent-sessions/lib/transport";
import {
  makeFixtureFetch,
  TERMINAL_TRANSCRIPT,
} from "@/features/agent-sessions/lib/mock-stream";

// Records each request the transport issues so the tests can assert the URL and
// the per-connection ticket header without a real network.
interface CapturedRequest {
  url: string;
  method: string;
  ticket: string | null;
}

function capturingFetch(): {
  fetch: typeof globalThis.fetch;
  requests: CapturedRequest[];
} {
  const requests: CapturedRequest[] = [];
  const inner = makeFixtureFetch(TERMINAL_TRANSCRIPT);
  const fetch = (async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = typeof input === "string" ? input : input.toString();
    const headers = new Headers(init?.headers);
    requests.push({
      url,
      method: init?.method ?? "GET",
      ticket: headers.get(AGENT_TICKET_HEADER),
    });
    return inner(input, init);
  }) as typeof globalThis.fetch;
  return { fetch, requests };
}

describe("createAgentSessionTransport", () => {
  it("mints a fresh ticket on every reconnect and never puts it in the URL", async () => {
    let n = 0;
    const mintTicket = vi.fn(async () => ({ ticket: `ticket-${++n}` }));
    const { fetch, requests } = capturingFetch();
    const transport = createAgentSessionTransport({
      sessionId: "as-123",
      mintTicket,
      fetch,
    });

    await transport.reconnectToStream({ chatId: "as-123" });
    await transport.reconnectToStream({ chatId: "as-123" });
    await transport.reconnectToStream({ chatId: "as-123" });

    expect(mintTicket).toHaveBeenCalledTimes(3);
    const tickets = requests.map((r) => r.ticket);
    // Each reconnect carried a distinct, freshly minted ticket.
    expect(tickets).toEqual(["ticket-1", "ticket-2", "ticket-3"]);
    expect(new Set(tickets).size).toBe(3);
    // The ticket rides the header, never the URL (D3 invariant #2).
    for (const req of requests) {
      expect(req.method).toBe("GET");
      expect(req.url).not.toContain("ticket-");
      expect(req.url).not.toContain(AGENT_TICKET_HEADER);
    }
  });

  it("reconnects to the exact same-origin stream URL (no double /stream suffix)", async () => {
    const mintTicket = vi.fn(async () => ({ ticket: "t" }));
    const { fetch, requests } = capturingFetch();
    const transport = createAgentSessionTransport({
      sessionId: "as-xyz",
      mintTicket,
      fetch,
    });

    await transport.reconnectToStream({ chatId: "as-xyz" });

    const expected = agentSessionStreamUrl("as-xyz");
    expect(requests[0].url).toBe(expected);
    expect(requests[0].url.match(/\/stream/g)).toHaveLength(1);
  });

  it("mints a fresh ticket on the send (POST) path too", async () => {
    let n = 0;
    const mintTicket = vi.fn(async () => ({ ticket: `send-${++n}` }));
    const { fetch, requests } = capturingFetch();
    const transport = createAgentSessionTransport({
      sessionId: "as-1",
      mintTicket,
      fetch,
    });

    await transport.sendMessages({
      trigger: "submit-message",
      chatId: "as-1",
      messageId: undefined,
      messages: [],
      abortSignal: undefined,
    });

    expect(mintTicket).toHaveBeenCalledTimes(1);
    expect(requests[0].method).toBe("POST");
    expect(requests[0].ticket).toBe("send-1");
    expect(requests[0].url).toBe(agentSessionStreamUrl("as-1"));
  });
});
