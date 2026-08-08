import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import {
  BodyTooLargeError,
  readBoundedBody,
  readBoundedFormData,
  readBoundedJson,
} from "@/common/server-fn/bounded-body";
import { handleConsentDecision } from "@/common/server-fn/hydra-consent";

/**
 * A POST Request whose body is a ReadableStream — so undici does NOT set a
 * Content-Length header, exactly the chunked/HTTP-2 shape a length-only guard
 * mis-reads as length 0. `pulled` counts how many chunks the consumer drained,
 * so a test can prove the reader stopped early instead of buffering it all.
 */
function streamedRequest(
  bytesPerChunk: number,
  {
    totalChunks = Number.POSITIVE_INFINITY,
    contentType = "application/x-www-form-urlencoded",
    headers = {},
    url = "https://dashboard.bex.co/x",
  }: {
    totalChunks?: number;
    contentType?: string;
    headers?: Record<string, string>;
    url?: string;
  } = {},
): { request: Request; pulled: () => number } {
  let pulled = 0;
  const chunk = new Uint8Array(bytesPerChunk);
  const stream = new ReadableStream<Uint8Array>({
    pull(controller) {
      if (pulled >= totalChunks) {
        controller.close();
        return;
      }
      pulled += 1;
      controller.enqueue(chunk);
    },
  });
  const init = {
    method: "POST",
    body: stream,
    duplex: "half",
    headers: { "content-type": contentType, ...headers },
  };
  return {
    request: new Request(url, init as RequestInit),
    pulled: () => pulled,
  };
}

describe("readBoundedBody", () => {
  it("rejects an oversized stream that carries no Content-Length, before buffering it all", async () => {
    // 1 KiB chunks, 4 KiB cap, unbounded producer.
    const { request, pulled } = streamedRequest(1024);
    let caught: unknown;
    try {
      await readBoundedBody(request, 4096);
    } catch (err) {
      caught = err;
    }
    expect(caught).toBeInstanceOf(BodyTooLargeError);
    // It aborted early: a handful of chunks read, not an unbounded drain.
    expect(pulled()).toBeLessThan(16);
  });

  it("returns the bytes for a body within the cap", async () => {
    const { request } = streamedRequest(1024, { totalChunks: 3 }); // 3 KiB
    const bytes = await readBoundedBody(request, 4096);
    expect(bytes.byteLength).toBe(3072);
  });

  it("parses a normal-size urlencoded body via readBoundedFormData", async () => {
    const req = new Request("https://dashboard.bex.co/x", {
      method: "POST",
      headers: { "content-type": "application/x-www-form-urlencoded" },
      body: new URLSearchParams({ decision: "approve", csrf_token: "t" }),
    });
    const form = await readBoundedFormData(req, 1 << 16);
    expect(form.get("decision")).toBe("approve");
    expect(form.get("csrf_token")).toBe("t");
  });

  it("parses a normal-size JSON body via readBoundedJson", async () => {
    const req = new Request("https://dashboard.bex.co/x", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ action: "revoke", id: "abc" }),
    });
    const body = await readBoundedJson<{ action: string; id: string }>(
      req,
      1 << 14,
    );
    expect(body).toEqual({ action: "revoke", id: "abc" });
  });

  it("rejects an oversized formData stream with BodyTooLargeError", async () => {
    const { request } = streamedRequest(1024); // unbounded, 2 KiB cap
    let caught: unknown;
    try {
      await readBoundedFormData(request, 2048);
    } catch (err) {
      caught = err;
    }
    expect(caught).toBeInstanceOf(BodyTooLargeError);
  });
});

// Regression at the handler seam: pre-fix, handleConsentDecision only checked
// Content-Length (absent ⇒ 0 ⇒ passes) and then buffered the whole body — so a
// large chunked body slipped through to formData(). Post-fix it is stream-bound
// and answers 413.
describe("handleConsentDecision body bound (finding 11)", () => {
  const DASHBOARD = "https://dashboard.bex.co";

  beforeEach(() => {
    // Kratos whoami ⇒ a live session, so the handler reaches the body read.
    vi.stubGlobal(
      "fetch",
      vi.fn(async (url: string) => {
        if (String(url).includes("/sessions/whoami")) {
          return new Response(
            JSON.stringify({
              id: "session-abc",
              active: true,
              identity: {
                id: "identity-xyz",
                schema_id: "default",
                traits: {},
              },
            }),
            { status: 200 },
          );
        }
        return new Response("{}", { status: 200 });
      }),
    );
  });
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("413s an oversized consent POST that omits Content-Length", async () => {
    // ~256 KiB body (> the 64 KiB consent cap), streamed so undici sets no
    // Content-Length — the exact bypass the old length-only guard allowed.
    const { request } = streamedRequest(16 * 1024, {
      totalChunks: 16,
      headers: { origin: DASHBOARD, cookie: "ory_session=live" },
      url: `${DASHBOARD}/auth/consent`,
    });
    const res = await handleConsentDecision(request);
    expect(res.status).toBe(413);
  });
});
