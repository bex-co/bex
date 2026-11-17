import { describe, expect, it } from "vitest";
import { shortUserAgent } from "@/features/audit/lib/short-user-agent";

// w9/m450: the audit row's Source cell names the client in short.
describe("shortUserAgent", () => {
  it.each([
    [
      "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36",
      "Chrome 131 · macOS",
    ],
    [
      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36 Edg/131.0.2903.86",
      "Edge 131 · Windows",
    ],
    [
      "Mozilla/5.0 (X11; Linux x86_64; rv:133.0) Gecko/20100101 Firefox/133.0",
      "Firefox 133 · Linux",
    ],
    [
      "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.1 Safari/605.1.15",
      "Safari 18 · macOS",
    ],
    [
      "Mozilla/5.0 (iPhone; CPU iPhone OS 18_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.1 Mobile/15E148 Safari/604.1",
      "Safari 18 · iOS",
    ],
    [
      "Mozilla/5.0 (Linux; Android 15; Pixel 9) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Mobile Safari/537.36",
      "Chrome 131 · Android",
    ],
  ])("names the browser and system of %s", (ua, want) => {
    expect(shortUserAgent(ua)).toBe(want);
  });

  it("keeps a non-browser client's first token", () => {
    expect(shortUserAgent("render-cli/2.27.0 (darwin; arm64)")).toBe(
      "render-cli/2.27.0",
    );
    expect(shortUserAgent("curl/8.4.0")).toBe("curl/8.4.0");
  });

  it("is empty for an empty agent", () => {
    expect(shortUserAgent("")).toBe("");
    expect(shortUserAgent("   ")).toBe("");
  });
});
