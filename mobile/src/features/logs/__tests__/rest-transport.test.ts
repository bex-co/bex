import { RestLogTransport } from "../rest-transport";
import { LogApiError } from "../types";
import { installAccessCheck } from "../../../common/apollo/access-link";

let removeAccessCheck: () => void;
beforeEach(() => {
  removeAccessCheck = installAccessCheck(() => true);
});
afterEach(() => removeAccessCheck());

// A minimal XMLHttpRequest stand-in: node has no XHR, and the live-tail path
// under test only needs open/setRequestHeader/send/onprogress/abort.
class FakeXHR {
  static last: FakeXHR | undefined;
  onprogress: (() => void) | undefined;
  responseText = "";
  private aborted = false;
  open() {}
  setRequestHeader() {}
  send() {
    FakeXHR.last = this;
  }
  abort() {
    this.aborted = true;
  }
  get isAborted() {
    return this.aborted;
  }
  deliver(chunk: string) {
    this.responseText += chunk;
    this.onprogress?.();
  }
}

const emptyPage = {
  hasMore: false,
  nextStartTime: "2026-08-02T00:00:00Z",
  nextEndTime: "2026-08-02T00:00:00Z",
  logs: [],
};

describe("RestLogTransport history", () => {
  it("rechecks access after waiting for credentials before dispatch", async () => {
    let resolveToken!: (token: string) => void;
    let allowed = true;
    removeAccessCheck();
    removeAccessCheck = installAccessCheck(() => allowed);
    let requests = 0;
    const transport = new RestLogTransport(
      "https://api.bex.co",
      {
        getAccessToken: () =>
          new Promise((resolve) => {
            resolveToken = resolve;
          }),
        forceRefresh: async () => undefined,
      },
      (async () => {
        requests++;
        return new Response(JSON.stringify(emptyPage));
      }) as typeof fetch,
    );
    const result = transport
      .history({ resource: "srv-1" }, new AbortController().signal)
      .then(
        () => "sent",
        () => "blocked",
      );
    allowed = false;
    resolveToken("synthetic-token");
    expect(await result).toBe("blocked");
    expect(requests).toBe(0);
  });

  it("uses the bearer token and refreshes once after a 401", async () => {
    const requests: Array<{ url: string; authorization?: string }> = [];
    const responses = [
      new Response(JSON.stringify({ message: "expired" }), { status: 401 }),
      new Response(JSON.stringify(emptyPage), { status: 200 }),
    ];
    let token = "old-token";
    let refreshes = 0;
    const transport = new RestLogTransport(
      "https://api.bex.co",
      {
        getAccessToken: async () => token,
        forceRefresh: async () => {
          refreshes += 1;
          token = "new-token";
        },
      },
      (async (input, init) => {
        requests.push({
          url: String(input),
          authorization: (init?.headers as Record<string, string>)
            .Authorization,
        });
        const response = responses.shift();
        if (!response) throw new Error("unexpected request");
        return response;
      }) as typeof fetch,
    );

    const page = await transport.history(
      { resource: "srv-1", types: ["app"] },
      new AbortController().signal,
    );
    expect(page).toEqual(emptyPage);
    expect(refreshes).toBe(1);
    expect(requests.map(({ authorization }) => authorization)).toEqual([
      "Bearer old-token",
      "Bearer new-token",
    ]);
    expect(requests[0].url).toBe(
      "https://api.bex.co/v1/logs?resource=srv-1&type=app",
    );
  });

  it("does not collapse a store-only 503 into a generic network error", async () => {
    const transport = new RestLogTransport(
      "https://api.bex.co",
      {
        getAccessToken: async () => "token",
        forceRefresh: async () => undefined,
      },
      (async () =>
        new Response(
          JSON.stringify({
            message:
              "request logs and structured log filters require the durable log store",
          }),
          { status: 503 },
        )) as typeof fetch,
    );
    let caught: unknown;
    try {
      await transport.history(
        { resource: "srv-1", statusCode: ["5xx"] },
        new AbortController().signal,
      );
    } catch (error) {
      caught = error;
    }
    expect(caught instanceof LogApiError).toBe(true);
    if (caught instanceof LogApiError) {
      expect(caught.code).toBe("store_unavailable");
      expect(caught.status).toBe(503);
    }
  });
});

describe("RestLogTransport live tail bounds (round-9 #11)", () => {
  beforeEach(() => {
    FakeXHR.last = undefined;
    (globalThis as { XMLHttpRequest?: unknown }).XMLHttpRequest =
      FakeXHR as unknown;
  });

  it("does not open a stream after its credential wait is canceled", async () => {
    let resolveToken!: (token: string) => void;
    const controller = new AbortController();
    const transport = new RestLogTransport("https://api.bex.co", {
      getAccessToken: () =>
        new Promise((resolve) => {
          resolveToken = resolve;
        }),
      forceRefresh: async () => undefined,
    });
    const result = transport
      .subscribe(
        { resource: "srv-1" },
        {
          onLine: () => undefined,
          onError: () => undefined,
          onClose: () => undefined,
        },
        controller.signal,
      )
      .then(
        () => "opened",
        () => "blocked",
      );
    controller.abort();
    resolveToken("synthetic-token");
    expect(await result).toBe("blocked");
    expect(FakeXHR.last).toBe(undefined);
  });

  it("ignores queued progress after a boundary abort closes the stream", async () => {
    const controller = new AbortController();
    const transport = new RestLogTransport("https://api.bex.co", {
      getAccessToken: async () => "synthetic-token",
      forceRefresh: async () => undefined,
    });
    const lines: unknown[] = [];
    await transport.subscribe(
      { resource: "srv-1" },
      {
        onLine: (line) => lines.push(line),
        onError: () => undefined,
        onClose: () => undefined,
      },
      controller.signal,
    );
    const xhr = FakeXHR.last!;
    controller.abort();
    xhr.deliver(
      'data: {"id":"late","message":"private","timestamp":"2026-08-02T00:00:00Z","labels":[]}\n\n',
    );
    expect(xhr.isAborted).toBe(true);
    expect(lines.length).toBe(0);
  });

  it("recycles the stream when the cumulative response budget is exceeded", async () => {
    const transport = new RestLogTransport(
      "https://api.bex.co",
      {
        getAccessToken: async () => "token",
        forceRefresh: async () => undefined,
      },
      fetch,
      1024, // a tiny budget so one frame trips it
    );
    const errors: LogApiError[] = [];
    const lines: unknown[] = [];
    const connection = await transport.subscribe(
      { resource: "srv-1", types: ["app"] },
      {
        onLine: (line) => lines.push(line),
        onError: (error) => errors.push(error),
        onClose: () => undefined,
      },
      new AbortController().signal,
    );
    const xhr = FakeXHR.last!;
    // A well-formed frame first, so the happy path is proven…
    xhr.deliver(
      'data: {"id":"a","message":"ok","timestamp":"2026-08-02T00:00:00Z","labels":[]}\n\n',
    );
    expect(lines.length).toBe(1);
    // …then more bytes than the budget: the connection must abort and surface
    // a network-class error (which the session turns into reconnect-from-newest),
    // never keep buffering into an unbounded responseText.
    xhr.deliver("x".repeat(2048));
    expect(xhr.isAborted).toBe(true);
    expect(errors.length).toBe(1);
    expect(errors[0].code).toBe("network");
    // The recycled stream is terminal: no further delivery does anything.
    xhr.deliver('data: {"id":"z"');
    expect(lines.length).toBe(1);
    connection.close();
  });
});
