import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  sharedFileIdFrom,
  isInlineImage,
  turnAttachmentsFrom,
  uploadAttachment,
  uploadFailureFrom,
} from "@/features/agent-sessions/lib/attachments";

vi.mock("@/config/config", () => ({
  config: { apiBaseUrl: "https://api.example.test" },
}));

describe("isInlineImage", () => {
  it("inlines only the four raster types the API serves inline", () => {
    for (const t of ["image/png", "image/jpeg", "image/gif", "image/webp"]) {
      expect(isInlineImage(t)).toBe(true);
    }
    // SVG is an image to the eye and a script host to the browser.
    for (const t of ["image/svg+xml", "text/html", "application/pdf", ""]) {
      expect(isInlineImage(t)).toBe(false);
    }
  });
});

describe("uploadFailureFrom", () => {
  it("keeps the server's own words verbatim", () => {
    expect(
      uploadFailureFrom(
        413,
        JSON.stringify({
          message: "attachment exceeds 25 MiB",
          code: "AGENT_ATTACHMENT_TOO_LARGE",
        }),
      ),
    ).toEqual({ kind: "server", message: "attachment exceeds 25 MiB" });
  });

  it("types a non-JSON edge answer and a dropped connection", () => {
    expect(uploadFailureFrom(502, "<html>bad gateway</html>")).toEqual({
      kind: "http",
      status: 502,
    });
    expect(uploadFailureFrom(0, "")).toEqual({ kind: "network" });
  });
});

describe("turnAttachmentsFrom", () => {
  it("keeps well-formed entries and drops malformed ones", () => {
    expect(
      turnAttachmentsFrom([
        { id: "att-1", name: "log.txt", mediaType: "text/plain", size: 12 },
        { id: "", name: "nameless" },
        null,
        "att-2",
        { id: "att-3", size: "big" },
      ]),
    ).toEqual([
      { id: "att-1", name: "log.txt", mediaType: "text/plain", size: 12 },
      { id: "att-3", name: "att-3", mediaType: "", size: 0 },
    ]);
    expect(turnAttachmentsFrom(undefined)).toEqual([]);
  });
});

/** A scriptable XMLHttpRequest the upload drives through jsdom's global. */
class FakeXHR {
  static last: FakeXHR | null = null;
  method = "";
  url = "";
  withCredentials = false;
  headers: Record<string, string> = {};
  status = 0;
  responseText = "";
  body: unknown;
  upload: { onprogress: ((e: ProgressEvent) => void) | null } = {
    onprogress: null,
  };
  onload: (() => void) | null = null;
  onerror: (() => void) | null = null;
  onabort: (() => void) | null = null;
  constructor() {
    FakeXHR.last = this;
  }
  open(method: string, url: string) {
    this.method = method;
    this.url = url;
  }
  setRequestHeader(name: string, value: string) {
    this.headers[name] = value;
  }
  send(body: unknown) {
    this.body = body;
  }
  abort() {
    this.onabort?.();
  }
  respond(status: number, body: string) {
    this.status = status;
    this.responseText = body;
    this.onload?.();
  }
}

describe("uploadAttachment", () => {
  const original = globalThis.XMLHttpRequest;
  beforeEach(() => {
    globalThis.XMLHttpRequest = FakeXHR as unknown as typeof XMLHttpRequest;
  });
  afterEach(() => {
    globalThis.XMLHttpRequest = original;
  });

  it("posts the raw file with the cookie session and reports progress", async () => {
    const file = new File(["hello"], "notes.log", { type: "text/plain" });
    const progress = vi.fn();
    const pending = uploadAttachment(file, {
      ownerId: "tea-1",
      onProgress: progress,
    });
    const xhr = FakeXHR.last!;
    expect(xhr.method).toBe("POST");
    const url = new URL(xhr.url);
    expect(url.origin + url.pathname).toBe(
      "https://api.example.test/v1/agent-attachments",
    );
    expect(url.searchParams.get("ownerId")).toBe("tea-1");
    expect(url.searchParams.get("name")).toBe("notes.log");
    expect(xhr.withCredentials).toBe(true);
    expect(xhr.body).toBe(file);

    xhr.upload.onprogress?.({
      lengthComputable: true,
      loaded: 2,
      total: 4,
    } as ProgressEvent);
    expect(progress).toHaveBeenCalledWith(0.5);

    xhr.respond(
      201,
      JSON.stringify({
        id: "att-9",
        name: "notes.log",
        mediaType: "text/plain",
        size: 5,
      }),
    );
    await expect(pending).resolves.toMatchObject({ id: "att-9" });
  });

  it("rejects with the server's message on a refusal", async () => {
    const pending = uploadAttachment(new File(["x"], "big.bin"), {
      ownerId: null,
    });
    FakeXHR.last!.respond(
      413,
      JSON.stringify({
        message: "too large",
        code: "AGENT_ATTACHMENT_TOO_LARGE",
      }),
    );
    await expect(pending).rejects.toMatchObject({
      failure: { kind: "server", message: "too large" },
    });
    expect(new URL(FakeXHR.last!.url).searchParams.has("ownerId")).toBe(false);
  });

  it("rejects a 201 whose body names no attachment", async () => {
    const pending = uploadAttachment(new File(["x"], "a.txt"), {
      ownerId: "tea-1",
    });
    FakeXHR.last!.respond(201, JSON.stringify({ name: "a.txt" }));
    await expect(pending).rejects.toMatchObject({
      failure: { kind: "http", status: 201 },
    });
  });

  it("rejects a dropped connection as a network failure", async () => {
    const pending = uploadAttachment(new File(["x"], "a.txt"), {
      ownerId: "tea-1",
    });
    FakeXHR.last!.onerror?.();
    await expect(pending).rejects.toMatchObject({
      failure: { kind: "network" },
    });
  });

  it("aborts when its signal fires", async () => {
    const controller = new AbortController();
    const pending = uploadAttachment(new File(["x"], "a.txt"), {
      ownerId: "tea-1",
      signal: controller.signal,
    });
    controller.abort();
    await expect(pending).rejects.toMatchObject({ name: "AbortError" });
  });
});

describe("sharedFileIdFrom", () => {
  const file = {
    attachmentId: "att-abc1",
    name: "r.csv",
    mediaType: "text/csv",
    size: 9,
  };

  it("reads the id from every shape an ACP adapter relays", () => {
    for (const output of [
      file,
      JSON.stringify(file),
      [{ type: "text", text: JSON.stringify(file) }],
      { content: [{ type: "text", text: JSON.stringify(file) }] },
      { structuredContent: file, content: [] },
    ]) {
      expect(sharedFileIdFrom(output)).toBe("att-abc1");
    }
  });

  it("ignores errors, other ids and plain text", () => {
    for (const output of [
      { error: { code: "too_large", message: "x" } },
      { attachmentId: "srv-abc", name: "x" },
      { attachmentId: "att-ABC", name: "x" },
      "not json",
      '{"attachmentId": broken',
      null,
      undefined,
      42,
    ]) {
      expect(sharedFileIdFrom(output)).toBeUndefined();
    }
  });
});
