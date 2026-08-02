import { LogApiError, type LogLine } from "./types";

export type SSEEvent =
  { type: "line"; line: LogLine } | { type: "error"; error: LogApiError };

function parseLine(raw: string): SSEEvent {
  let value: unknown;
  try {
    value = JSON.parse(raw);
  } catch {
    return {
      type: "error",
      error: new LogApiError("unknown", "The log stream sent invalid data."),
    };
  }
  if (
    typeof value !== "object" ||
    value === null ||
    typeof (value as Partial<LogLine>).id !== "string" ||
    typeof (value as Partial<LogLine>).message !== "string" ||
    typeof (value as Partial<LogLine>).timestamp !== "string" ||
    !Array.isArray((value as Partial<LogLine>).labels)
  ) {
    return {
      type: "error",
      error: new LogApiError("unknown", "The log stream sent an invalid line."),
    };
  }
  return { type: "line", line: value as LogLine };
}

export class SSEParser {
  private pending = "";

  feed(chunk: string): SSEEvent[] {
    // Normalize after concatenating so a CRLF split across two network chunks
    // is still recognized as one line ending.
    this.pending = (this.pending + chunk).replace(/\r\n/g, "\n");
    const events: SSEEvent[] = [];
    let boundary = this.pending.indexOf("\n\n");
    while (boundary >= 0) {
      const block = this.pending.slice(0, boundary);
      this.pending = this.pending.slice(boundary + 2);
      const event = this.parseBlock(block);
      if (event) events.push(event);
      boundary = this.pending.indexOf("\n\n");
    }
    return events;
  }

  private parseBlock(block: string): SSEEvent | undefined {
    let eventName = "message";
    const data: string[] = [];
    for (const line of block.split("\n")) {
      if (line.startsWith(":")) continue;
      if (line.startsWith("event:")) eventName = line.slice(6).trim();
      if (line.startsWith("data:")) data.push(line.slice(5).trimStart());
    }
    if (data.length === 0) return undefined;
    const payload = data.join("\n");
    if (eventName === "error") {
      let message = payload;
      try {
        const decoded: unknown = JSON.parse(payload);
        if (typeof decoded === "string") message = decoded;
      } catch {
        // The server's terminal SSE error is normally a JSON string, but retain
        // a plain-text message so an upstream proxy cannot hide the failure.
      }
      return { type: "error", error: classifyLogError(400, message) };
    }
    return parseLine(payload);
  }
}

export function classifyLogError(status: number, message: string): LogApiError {
  const lower = message.toLowerCase();
  if (
    lower.includes("durable log store") ||
    lower.includes("structured log filters require")
  ) {
    return new LogApiError("store_unavailable", message, status);
  }
  if (status === 400) return new LogApiError("invalid_filter", message, status);
  if (status === 401) return new LogApiError("unauthorized", message, status);
  if (status === 403) return new LogApiError("forbidden", message, status);
  if (status === 503) return new LogApiError("unavailable", message, status);
  if (status === 0) return new LogApiError("network", message, status);
  return new LogApiError("unknown", message, status);
}
