import { classifyLogError, SSEParser } from "../sse-parser";

describe("SSEParser", () => {
  it("parses lines split across arbitrary network chunks", () => {
    const subject = new SSEParser();
    expect(subject.feed('data: {"id":"a","message":"he')).toEqual([]);
    const events = subject.feed(
      'llo","timestamp":"2026-08-02T00:00:00Z","labels":[]}\n\n',
    );
    expect(events).toEqual([
      {
        type: "line",
        line: {
          id: "a",
          message: "hello",
          timestamp: "2026-08-02T00:00:00Z",
          labels: [],
        },
      },
    ]);
  });

  it("handles a CRLF event boundary split across chunks", () => {
    const subject = new SSEParser();
    expect(
      subject.feed(
        'data: {"id":"a","message":"ok","timestamp":"2026-08-02T00:00:00Z","labels":[]}\r',
      ),
    ).toEqual([]);
    expect(subject.feed("\n\r\n")[0].type).toBe("line");
  });

  it("surfaces terminal SSE errors instead of treating them as EOF", () => {
    const [event] = new SSEParser().feed(
      'event: error\ndata: "no active build is running"\n\n',
    );
    expect(event.type).toBe("error");
    if (event.type === "error") {
      expect(event.error.code).toBe("invalid_filter");
      expect(event.error.message).toBe("no active build is running");
    }
  });

  it("classifies the backend's explicit store-only refusal", () => {
    const error = classifyLogError(
      503,
      "request logs and structured log filters require the durable log store",
    );
    expect(error.code).toBe("store_unavailable");
  });
});
