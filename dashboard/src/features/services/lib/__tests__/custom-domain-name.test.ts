import { describe, expect, it } from "vitest";
import { domainNameError, normalizedDomainName } from "../custom-domain-name";

// w21/048: the verdicts below are bex-api's own (lego/backend/internal/apps/
// domains.go normalizeHostname, canonicalHostname and claimableHostname, as
// probed on production), so the dialog never refuses what the server accepts.
describe("domainNameError", () => {
  it.each([
    "www.example.com",
    "qa-20261110.example.com",
    "QA-20261110.EXAMPLE.COM",
    // An internationalized TLD in punycode (.рф), a punycode label, a trailing dot.
    "qa-20261110.example.xn--p1ai",
    "xn--80ak6aa92e.com",
    "qa-20261110.example.com.",
    "  www.example.com  ",
    "a.b.c.d.example.co.uk",
    // The server's other rules stay the server's: it answers these as readable
    // sentences in the dialog.
    "co.uk",
    `${"a".repeat(64)}.example.com`,
  ])("lets %j through", (name) => {
    expect(domainNameError(name)).toBeNull();
  });

  it.each([
    "not a domain",
    "https://www.example.com",
    "www.example.com/path",
    "www.example.com:8080",
    "192.168.0.1",
    "localhost",
    "example",
    "example.123",
    "qa-20261110.example.123",
    ".example.com",
    "-bad.example.com",
    "bad-.example.com",
    // Raw Unicode: the server wants the punycode spelling.
    "пример.рф",
    "bücher.de",
    "example.com..",
    "example.xn--",
    "",
  ])("refuses %j", (name) => {
    expect(domainNameError(name)).toBe("invalid");
  });

  it.each(["*.example.com", "*.qa-20261110.example.com", " *.example.com "])(
    "marks the wildcard %j with its own reason",
    (name) => {
      expect(domainNameError(name)).toBe("wildcard");
    },
  );
});

describe("normalizedDomainName", () => {
  it("trims and drops one trailing dot, as the server stores it", () => {
    expect(normalizedDomainName("  qa.example.com.  ")).toBe("qa.example.com");
    expect(normalizedDomainName("qa.example.com")).toBe("qa.example.com");
    expect(normalizedDomainName("qa.example.com..")).toBe("qa.example.com.");
  });
});
