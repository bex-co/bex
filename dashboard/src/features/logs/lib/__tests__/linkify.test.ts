import { describe, expect, it } from "vitest";
import { parseAnsi } from "../ansi";
import { linkifySpans, linkifyText } from "../linkify";

const ESC = "\u001b";

describe("linkifyText", () => {
  it("returns null when a message contains no HTTP(S) URL", () => {
    expect(linkifyText("plain message without links")).toBeNull();
    expect(linkifyText("listening on port 10000")).toBeNull();
    expect(linkifyText("")).toBeNull();
  });

  it("linkifies http:// and https:// URLs while keeping exact text without adding a trailing slash", () => {
    expect(
      linkifyText("==> Available at your primary URL https://hello.onbex.co"),
    ).toEqual([
      { kind: "text", text: "==> Available at your primary URL " },
      {
        kind: "link",
        text: "https://hello.onbex.co",
        href: "https://hello.onbex.co",
      },
    ]);

    expect(
      linkifyText(
        "Local: http://localhost:3000 and https://app.example.com/v1?a=1#top",
      ),
    ).toEqual([
      { kind: "text", text: "Local: " },
      {
        kind: "link",
        text: "http://localhost:3000",
        href: "http://localhost:3000",
      },
      { kind: "text", text: " and " },
      {
        kind: "link",
        text: "https://app.example.com/v1?a=1#top",
        href: "https://app.example.com/v1?a=1#top",
      },
    ]);
  });

  it("trims trailing punctuation and unmatched closing delimiters while preserving balanced ones", () => {
    expect(
      linkifyText(
        'See (https://example.com/docs), "https://en.wikipedia.org/wiki/Foo_(bar).", and [http://[::1]:8080/health]!',
      ),
    ).toEqual([
      { kind: "text", text: "See (" },
      {
        kind: "link",
        text: "https://example.com/docs",
        href: "https://example.com/docs",
      },
      { kind: "text", text: '), "' },
      {
        kind: "link",
        text: "https://en.wikipedia.org/wiki/Foo_(bar)",
        href: "https://en.wikipedia.org/wiki/Foo_(bar)",
      },
      { kind: "text", text: '.", and [' },
      {
        kind: "link",
        text: "http://[::1]:8080/health",
        href: "http://[::1]:8080/health",
      },
      { kind: "text", text: "]!" },
    ]);
  });

  it("leaves non-http(s) schemes and malformed URLs as plain text", () => {
    for (const message of [
      "javascript:alert(1)",
      "JAVASCRIPT:alert(1)",
      "data:text/html,<script>alert(1)</script>",
      "vbscript:msgbox(1)",
      "file:///etc/passwd",
      "ftp://files.example.com/archive.tar.gz",
      "mailto:ops@example.com",
      "//protocol-relative.example.com/path",
      "https://",
      "http://:8080",
      "https://...",
      "https://---",
      "nothttps://example.com",
      "git+https://github.com/org/repo.git",
    ]) {
      expect(linkifyText(message), message).toBeNull();
    }
  });
});

describe("linkifySpans", () => {
  it("preserves ANSI styling around and inside Render's primary URL line", () => {
    const spans = parseAnsi(
      `${ESC}[0;32m${ESC}[1m==> ${ESC}[0m${ESC}[1mAvailable at your primary URL https://hello-go.onbex.co + 1 more domain${ESC}[0m`,
    );
    expect(linkifySpans(spans)).toEqual([
      {
        kind: "text",
        spans: [
          {
            text: "==> ",
            className: "font-semibold text-green-700 dark:text-green-400",
            style: undefined,
          },
          {
            text: "Available at your primary URL ",
            className: "font-semibold",
            style: undefined,
          },
        ],
      },
      {
        kind: "link",
        href: "https://hello-go.onbex.co",
        spans: [
          {
            text: "https://hello-go.onbex.co",
            className: "font-semibold",
            style: undefined,
          },
        ],
      },
      {
        kind: "text",
        spans: [
          {
            text: " + 1 more domain",
            className: "font-semibold",
            style: undefined,
          },
        ],
      },
    ]);
  });

  it("links a URL split across multiple ANSI spans (e.g. Vite's bold port)", () => {
    const spans = parseAnsi(
      `  Local: ${ESC}[36mhttp://localhost:${ESC}[1m5173${ESC}[22m/${ESC}[39m`,
    );
    expect(linkifySpans(spans)).toEqual([
      {
        kind: "text",
        spans: [{ text: "  Local: ", className: "", style: undefined }],
      },
      {
        kind: "link",
        href: "http://localhost:5173/",
        spans: [
          {
            text: "http://localhost:",
            className: "text-cyan-700 dark:text-cyan-400",
            style: undefined,
          },
          {
            text: "5173",
            className: "font-semibold text-cyan-700 dark:text-cyan-400",
            style: undefined,
          },
          {
            text: "/",
            className: "text-cyan-700 dark:text-cyan-400",
            style: undefined,
          },
        ],
      },
    ]);
  });

  it("returns null for ANSI spans with no safe HTTP(S) URL", () => {
    const spans = parseAnsi(
      `${ESC}[0;31m${ESC}[1m==> javascript:alert(1) data:text/html,hi${ESC}[0m`,
    );
    expect(linkifySpans(spans)).toBeNull();
  });
});
