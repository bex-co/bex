import { describe, it, expect } from "vitest";
import { safeNext } from "@/common/lib/safe-next";

const ORIGIN = "https://dashboard.bex.co";

describe("safeNext", () => {
  describe("rejects external / obfuscated targets → /", () => {
    const external = [
      // absolute
      "https://evil.com",
      "http://evil.com/path",
      "https://evil.com@dashboard.bex.co", // userinfo trick
      // protocol-relative
      "//evil.com",
      "//evil.com/path?x=1",
      // percent-encoded
      "%2f%2fevil.com",
      "%2F%2Fevil.com",
      "https:%2f%2fevil.com",
      "https:%2F%2Fevil.com",
      // backslash-obfuscated (WHATWG normalizes \ → / for special schemes)
      "/\\evil.com",
      "\\/\\/evil.com",
      "\\\\evil.com",
      "/\\/\\evil.com",
      // mixed-case scheme
      "HtTpS://evil.com",
      "JAVASCRIPT:alert(1)",
      "javascript:alert(1)",
      "data:text/html,<script>alert(1)</script>",
    ];
    for (const value of external) {
      it(`maps ${JSON.stringify(value)} to /`, () => {
        expect(safeNext(value, ORIGIN)).toBe("/");
      });
    }
  });

  describe("falls back to / for empty / non-string input", () => {
    it.each([
      [undefined, "/"],
      [null, "/"],
      ["", "/"],
    ])("safeNext(%s) === %s", (input, expected) => {
      expect(safeNext(input as string | null | undefined, ORIGIN)).toBe(
        expected,
      );
    });
  });

  describe("preserves legitimate internal routes (with query + hash)", () => {
    const internal = [
      "/",
      "/services/srv-abc",
      "/services/srv-abc?tab=env",
      "/services/srv-abc?tab=env#logs",
      "/auth/consent?consent_challenge=abc123",
      "/auth/consent?consent_challenge=abc&next=/foo",
      "/settings#security",
    ];
    for (const value of internal) {
      it(`keeps ${JSON.stringify(value)} unchanged`, () => {
        expect(safeNext(value, ORIGIN)).toBe(value);
      });
    }

    it("keeps a same-origin path but strips an absolute origin prefix would not survive", () => {
      // A path that resolves same-origin is returned as pathname+search+hash.
      expect(safeNext("/a/b/../c?q=1#h", ORIGIN)).toBe("/a/c?q=1#h");
    });

    it("is idempotent on an already-safe value", () => {
      const once = safeNext("/services/srv-abc?tab=env#logs", ORIGIN);
      expect(safeNext(once, ORIGIN)).toBe(once);
    });
  });

  it("defaults the origin from window when none is passed", () => {
    // jsdom serves window.location.origin; an internal path still round-trips
    // and an external one still collapses.
    expect(safeNext("/dashboard/home")).toBe("/dashboard/home");
    expect(safeNext("https://evil.com")).toBe("/");
    expect(safeNext("//evil.com")).toBe("/");
  });
});
