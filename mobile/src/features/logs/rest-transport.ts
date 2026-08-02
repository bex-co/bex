import { buildLogQuery } from "./query";
import { classifyLogError, SSEParser } from "./sse-parser";
import type {
  LogFilters,
  LogHistoryPage,
  LogTailCallbacks,
  LogTailConnection,
  LogTransport,
} from "./types";
import { LogApiError } from "./types";

export type LogCredentials = {
  getAccessToken: () => Promise<string>;
  forceRefresh: () => Promise<unknown>;
};

type FetchLike = typeof fetch;

function responseMessage(body: unknown, fallback: string): string {
  if (typeof body !== "object" || body === null) return fallback;
  const message = (body as { message?: unknown }).message;
  const error = (body as { error?: unknown }).error;
  if (typeof message === "string") return message;
  if (typeof error === "string") return error;
  return fallback;
}

export class RestLogTransport implements LogTransport {
  constructor(
    private readonly apiOrigin: string,
    private readonly credentials: LogCredentials,
    private readonly fetchImpl: FetchLike = fetch,
  ) {}

  async history(
    filters: LogFilters,
    signal: AbortSignal,
  ): Promise<LogHistoryPage> {
    let response: Response;
    try {
      response = await this.fetchHistory(filters, signal);
    } catch (error) {
      if (signal.aborted) throw error;
      throw new LogApiError(
        "network",
        error instanceof Error ? error.message : "Log history disconnected.",
      );
    }
    if (response.status === 401) {
      await this.credentials.forceRefresh();
      try {
        response = await this.fetchHistory(filters, signal);
      } catch (error) {
        if (signal.aborted) throw error;
        throw new LogApiError(
          "network",
          error instanceof Error ? error.message : "Log history disconnected.",
        );
      }
    }
    let body: unknown;
    try {
      body = await response.json();
    } catch {
      body = undefined;
    }
    if (!response.ok) {
      throw classifyLogError(
        response.status,
        responseMessage(body, `Log history failed (${response.status}).`),
      );
    }
    return body as LogHistoryPage;
  }

  private async fetchHistory(
    filters: LogFilters,
    signal: AbortSignal,
  ): Promise<Response> {
    const accessToken = await this.credentials.getAccessToken();
    return this.fetchImpl(
      `${this.apiOrigin}/v1/logs?${buildLogQuery(filters)}`,
      {
        method: "GET",
        headers: { Authorization: `Bearer ${accessToken}` },
        signal,
      },
    );
  }

  async subscribe(
    filters: LogFilters,
    callbacks: LogTailCallbacks,
    signal: AbortSignal,
  ): Promise<LogTailConnection> {
    const accessToken = await this.credentials.getAccessToken();
    let refreshing = false;
    return openXHRStream(
      `${this.apiOrigin}/v1/logs/subscribe?${buildLogQuery(filters)}`,
      accessToken,
      {
        ...callbacks,
        onError: (error) => {
          if (error.code !== "unauthorized" || refreshing || signal.aborted) {
            callbacks.onError(error);
            return;
          }
          refreshing = true;
          void this.credentials.forceRefresh().then(
            () =>
              callbacks.onError(
                new LogApiError("network", "Log credentials refreshed."),
              ),
            () => callbacks.onError(error),
          );
        },
      },
      signal,
    );
  }
}

function openXHRStream(
  url: string,
  accessToken: string,
  callbacks: LogTailCallbacks,
  signal: AbortSignal,
): LogTailConnection {
  const xhr = new XMLHttpRequest();
  const parser = new SSEParser();
  let consumed = 0;
  let terminal = false;

  const close = () => {
    if (terminal) return;
    terminal = true;
    xhr.abort();
  };
  signal.addEventListener("abort", close, { once: true });
  xhr.open("GET", url, true);
  xhr.setRequestHeader("Accept", "text/event-stream");
  xhr.setRequestHeader("Authorization", `Bearer ${accessToken}`);
  xhr.onprogress = () => {
    const chunk = xhr.responseText.slice(consumed);
    consumed = xhr.responseText.length;
    for (const event of parser.feed(chunk)) {
      if (event.type === "line") callbacks.onLine(event.line);
      else callbacks.onError(event.error);
    }
  };
  xhr.onerror = () => {
    if (!terminal)
      callbacks.onError(classifyLogError(0, "Log stream disconnected."));
  };
  xhr.onload = () => {
    if (terminal) return;
    terminal = true;
    if (xhr.status < 200 || xhr.status >= 300) {
      callbacks.onError(
        classifyLogError(
          xhr.status,
          responseMessageFromText(xhr.responseText, xhr.status),
        ),
      );
    } else {
      callbacks.onClose();
    }
  };
  xhr.send();
  return { close };
}

function responseMessageFromText(text: string, status: number): string {
  try {
    return responseMessage(JSON.parse(text), `Log stream failed (${status}).`);
  } catch {
    return text || `Log stream failed (${status}).`;
  }
}
