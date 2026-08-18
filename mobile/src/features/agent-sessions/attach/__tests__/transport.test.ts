import type { UIMessageChunk } from "ai";
import {
  AGENT_TICKET_HEADER,
  AGENT_TURN_IDEMPOTENCY_HEADER,
  createAgentSessionTransport,
} from "../transport";

const HEADERS = {
  "content-type": "text/event-stream",
  "x-vercel-ai-ui-message-stream": "v1",
};

type CapturedRequest = {
  url: string;
  method: string;
  ticket: string | null;
  idempotencyKey: string | null;
};

function fixtureFetch(): {
  fetch: typeof globalThis.fetch;
  requests: CapturedRequest[];
} {
  const requests: CapturedRequest[] = [];
  const chunks: UIMessageChunk[] = [
    { type: "start", messageId: "assistant-1" },
    { type: "finish" },
  ];
  const fetch = (async (input: RequestInfo | URL, init?: RequestInit) => {
    const headers = new Headers(init?.headers);
    requests.push({
      url: input.toString(),
      method: init?.method ?? "GET",
      ticket: headers.get(AGENT_TICKET_HEADER),
      idempotencyKey: headers.get(AGENT_TURN_IDEMPOTENCY_HEADER),
    });
    const encoder = new TextEncoder();
    return new Response(
      new ReadableStream<Uint8Array>({
        start(controller) {
          chunks.forEach((chunk, seq) => {
            controller.enqueue(
              encoder.encode(`id: ${seq}\ndata: ${JSON.stringify(chunk)}\n\n`),
            );
          });
          controller.enqueue(encoder.encode("data: [DONE]\n\n"));
          controller.close();
        },
      }),
      { status: 200, headers: HEADERS },
    );
  }) as typeof globalThis.fetch;
  return { fetch, requests };
}

async function drain<T>(stream: ReadableStream<T> | null): Promise<T[]> {
  if (!stream) return [];
  const output: T[] = [];
  for await (const chunk of stream) output.push(chunk);
  return output;
}

describe("agent-session AI SDK transport", () => {
  it("mints a fresh read ticket for every reconnect and resumes after its cursor", async () => {
    const actions: string[] = [];
    const { fetch, requests } = fixtureFetch();
    const transport = createAgentSessionTransport({
      sessionId: "ags-read",
      initialCursor: -1,
      fetch,
      mintTicket: async (action) => {
        actions.push(action);
        return { ticket: `ticket-${actions.length}` };
      },
      api: "https://api.bex.co/v1/agent-sessions/ags-read/stream",
    });

    const first = await drain(
      await transport.reconnectToStream({ chatId: "ags-read" }),
    );
    const second = await drain(
      await transport.reconnectToStream({ chatId: "ags-read" }),
    );

    expect(actions).toEqual(["read", "read"]);
    expect(requests.map((request) => request.ticket)).toEqual([
      "ticket-1",
      "ticket-2",
    ]);
    expect(requests[0].url).toContain("afterSeq=-1");
    expect(requests[1].url).toContain("afterSeq=1");
    expect(first.length > 0).toBe(true);
    expect(second).toEqual([]);
    expect(requests.some((request) => request.url.includes("ticket-"))).toBe(
      false,
    );
  });

  it("mints a turn ticket and sends the stable user idempotency key", async () => {
    const actions: string[] = [];
    const { fetch, requests } = fixtureFetch();
    const transport = createAgentSessionTransport({
      sessionId: "ags-turn",
      fetch,
      mintTicket: async (action) => {
        actions.push(action);
        return { ticket: "turn-ticket" };
      },
      api: "https://api.bex.co/v1/agent-sessions/ags-turn/stream",
    });

    await drain(
      await transport.sendMessages({
        trigger: "submit-message",
        chatId: "ags-turn",
        messageId: undefined,
        messages: [
          {
            id: "turn-key-1",
            role: "user",
            parts: [{ type: "text", text: "continue" }],
          },
        ],
        abortSignal: undefined,
      }),
    );

    expect(actions).toEqual(["turn"]);
    expect(requests[0].method).toBe("POST");
    expect(requests[0].ticket).toBe("turn-ticket");
    expect(requests[0].idempotencyKey).toBe("turn-key-1");
  });

  it("rejects a missing ticket before any stream request", async () => {
    const { fetch, requests } = fixtureFetch();
    const transport = createAgentSessionTransport({
      sessionId: "ags-empty-ticket",
      fetch,
      mintTicket: async () => ({ ticket: "" }),
      api: "https://api.bex.co/v1/agent-sessions/ags-empty-ticket/stream",
    });

    let message = "";
    try {
      await transport.reconnectToStream({ chatId: "ags-empty-ticket" });
    } catch (error) {
      message = error instanceof Error ? error.message : String(error);
    }
    expect(message).toBe("attachAgentSession returned no ticket");
    expect(requests).toEqual([]);
  });
});
