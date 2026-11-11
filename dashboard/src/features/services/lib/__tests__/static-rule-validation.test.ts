import { describe, expect, it } from "vitest";
import {
  RESERVED_STATIC_HEADERS,
  headerErrors,
  humanizeRuleError,
  routeErrors,
} from "../static-rule-validation";

// w4/145: the rules bex-api enforces, checked in the row before Save.
describe("static-site rule validation", () => {
  it("accepts a redirect or a rewrite to a fixed external host (w8/184)", () => {
    expect(
      routeErrors({
        type: "redirect",
        source: "/old",
        destination: "https://example.org/landing",
      }),
    ).toEqual({});
    expect(
      routeErrors({
        type: "rewrite",
        source: "/old",
        destination: "https://example.org/landing",
      }),
    ).toEqual({});
    for (const destination of [
      "https://user@example.org/x",
      "https://*.example.org/x",
      "https:///x",
    ]) {
      expect(
        routeErrors({ type: "redirect", source: "/old", destination }),
      ).toEqual({ destination: "services.staticRuleExternalHost" });
    }
  });

  it("marks a redirect destination that is not a site path", () => {
    expect(
      routeErrors({
        type: "redirect",
        source: "/old",
        destination: "//evil.test",
      }),
    ).toEqual({ destination: "services.staticRuleLocalPath" });
  });

  it("accepts paths the server would accept after trimming", () => {
    expect(
      routeErrors({
        type: "rewrite",
        source: " /qa/* ",
        destination: " /:splat ",
      }),
    ).toEqual({});
    expect(
      headerErrors({ path: "/*", name: "X-Frame-Options", value: "" }),
    ).toEqual({});
  });

  it("requires every path and a header name", () => {
    expect(
      routeErrors({ type: "rewrite", source: "", destination: "" }),
    ).toEqual({
      source: "services.staticRuleRequired",
      destination: "services.staticRuleRequired",
    });
    expect(headerErrors({ path: "app/*", name: " ", value: "x" })).toEqual({
      path: "services.staticRulePathSlash",
      name: "services.staticRuleRequired",
    });
  });

  // w4/402: the row-local header rules bex-api also enforces, checked in the
  // row. The reserved list mirrors lego/types/v1alpha1/static_headers.go.
  it("pins the reserved header list against the Go list", () => {
    expect([...RESERVED_STATIC_HEADERS]).toEqual([
      "Content-Length",
      "Transfer-Encoding",
      "Connection",
      "Keep-Alive",
      "Upgrade",
      "Te",
      "Trailer",
      "Proxy-Connection",
    ]);
  });

  it.each(["Content-Length", "content-length", "TRANSFER-ENCODING"])(
    "marks the reserved header name %s in its row",
    (name) => {
      expect(headerErrors({ path: "/*", name, value: "5" })).toEqual({
        name: "services.staticRuleHeaderNameReserved",
      });
    },
  );

  it("marks a malformed header name and a control character in the value", () => {
    expect(headerErrors({ path: "/*", name: "Bad Name", value: "x" })).toEqual({
      name: "services.staticRuleHeaderNameInvalid",
    });
    expect(headerErrors({ path: "/*", name: "X-Test", value: "a\nb" })).toEqual(
      { value: "services.staticRuleHeaderValueInvalid" },
    );
    expect(headerErrors({ path: "/*", name: "X-Test", value: "a\tb" })).toEqual(
      {},
    );
  });

  it("marks Set-Cookie with a Domain, and accepts one without", () => {
    expect(
      headerErrors({
        path: "/*",
        name: "Set-Cookie",
        value: "a=b; Domain=onbex.co",
      }),
    ).toEqual({ value: "services.staticRuleCookieDomain" });
    expect(
      headerErrors({ path: "/*", name: "Set-Cookie", value: "a=b; Path=/" }),
    ).toEqual({});
  });

  it("marks over-long path, name and value", () => {
    expect(
      headerErrors({
        path: `/${"a".repeat(2048)}`,
        name: "X-Test",
        value: "x",
      }),
    ).toEqual({ path: "services.staticRulePathTooLong" });
    expect(
      headerErrors({ path: "/*", name: "X".repeat(257), value: "x" }),
    ).toEqual({ name: "services.staticRuleHeaderNameTooLong" });
    expect(
      headerErrors({ path: "/*", name: "X-Test", value: "x".repeat(4097) }),
    ).toEqual({ value: "services.staticRuleHeaderValueTooLong" });
  });

  it("names the server's zero-based wire index as the one-based row", () => {
    const row = (n: number) => `Row ${n}:`;
    expect(
      humanizeRuleError(
        "Routes[1].destination must be a path starting with /",
        row,
      ),
    ).toBe("Row 2: destination must be a path starting with /");
    expect(humanizeRuleError("headers[0].name is required", row)).toBe(
      "Row 1: name is required",
    );
    expect(
      humanizeRuleError(
        "Routes[5].destination references :nobody, which the rule's source does not declare",
        row,
      ),
    ).toBe(
      "Row 6: destination references :nobody, which the rule's source does not declare",
    );
    expect(humanizeRuleError("Could not save routes. Try again.", row)).toBe(
      "Could not save routes. Try again.",
    );
  });
});

it.each([
  "/bad%",
  "/ok?x=%zz",
  "/ok#bad%2",
  "/%2fhost",
  "/%5chost",
  "/nested/%5cfile",
  "/%00file",
  "/%1ffile",
  "/%7ffile",
  "/bad\tfile",
])("refuses unsafe or malformed destination %s", (destination) => {
  expect(routeErrors({ type: "rewrite", source: "/old", destination })).toEqual(
    { destination: "services.staticRuleLocalPath" },
  );
});

it.each([
  "/%72ender.yaml?from=rule#section",
  "/files/:splat?q=:splat#/*",
  "/files/*",
  "/a%252fb",
  "/ok?x=%00#%1f",
  "/nested/../file",
])("preserves valid configured destination %s", (destination) => {
  expect(
    routeErrors({ type: "rewrite", source: "/old/*", destination }),
  ).toEqual({});
});

// w21/043: the rules bex-api applies to a route's length and `:name`
// placeholders, checked in the row.
describe("static-site route length and placeholders (w21/043)", () => {
  const rule = (source: string, destination: string) =>
    routeErrors({ type: "redirect", source, destination });

  it("counts a rule path in UTF-8 bytes, as the server does", () => {
    // 2048 bytes saves; one more does not.
    expect(rule(`/${"a".repeat(2047)}`, "/index.html")).toEqual({});
    expect(rule(`/${"a".repeat(2048)}`, "/index.html")).toEqual({
      source: "services.staticRulePathTooLong",
    });
    expect(rule("/ok", `/${"b".repeat(2047)}`)).toEqual({});
    expect(rule("/ok", `/${"b".repeat(2048)}`)).toEqual({
      destination: "services.staticRulePathTooLong",
    });
    // 700 Chinese characters are 2100 bytes: 701 characters, over the budget.
    const chinese = `/${"中".repeat(700)}`;
    expect(chinese.length).toBe(701);
    expect(rule(chinese, "/index.html")).toEqual({
      source: "services.staticRulePathTooLong",
    });
    expect(headerErrors({ path: chinese, name: "X-Long", value: "1" })).toEqual(
      { path: "services.staticRulePathTooLong" },
    );
    // 682 Chinese characters are 2046 bytes plus the slash: 2047 bytes, fine.
    expect(
      headerErrors({
        path: `/${"中".repeat(682)}`,
        name: "X-Long",
        value: "1",
      }),
    ).toEqual({});
  });

  it("counts the length of an external destination too", () => {
    expect(rule("/ok", `https://example.org/${"c".repeat(2048)}`)).toEqual({
      destination: "services.staticRulePathTooLong",
    });
  });

  it("counts a header value in bytes, as the server does", () => {
    // U+00E9 is allowed in a value and is two bytes: 2049 of them are 4098.
    expect(
      headerErrors({ path: "/*", name: "X-Test", value: "é".repeat(2048) }),
    ).toEqual({});
    expect(
      headerErrors({ path: "/*", name: "X-Test", value: "é".repeat(2049) }),
    ).toEqual({ value: "services.staticRuleHeaderValueTooLong" });
  });

  it("marks a source that declares the same :name twice", () => {
    expect(rule("/a/:x/:x", "/b/:x")).toEqual({
      source: "services.staticRuleSourceRepeatsPlaceholder",
    });
    expect(rule("/a/:x/:y", "/b/:x/:y")).toEqual({});
  });

  it("marks a destination :name the source does not declare", () => {
    expect(rule("/blog/:slug", "/posts/:slg")).toEqual({
      destination: "services.staticRuleUndeclaredPlaceholder",
    });
    expect(rule("/blog/:slug", "/posts/:slug")).toEqual({});
    // :splat is always available, and a destination with no placeholder is fine.
    expect(rule("/files/*", "/dl/:splat")).toEqual({});
    expect(rule("/old", "/new")).toEqual({});
  });

  it("reads a placeholder anywhere in the destination path, not its query", () => {
    expect(rule("/blog/:slug", "/posts/x-:slg")).toEqual({
      destination: "services.staticRuleUndeclaredPlaceholder",
    });
    // A query or fragment is data, not a path selector.
    expect(rule("/blog/:slug", "/posts?q=:nobody#:also")).toEqual({});
    // Not a placeholder: a lone colon and a leading digit.
    expect(rule("/blog/:slug", "/v1/items:")).toEqual({});
    expect(rule("/blog/:slug", "/a/:1x")).toEqual({});
    // The percent-decoded path is what the server reads.
    expect(rule("/blog/:slug", "/posts/%3Aslg")).toEqual({
      destination: "services.staticRuleUndeclaredPlaceholder",
    });
  });

  it("applies the placeholder rule to an external destination's path", () => {
    expect(rule("/blog/:slug", "https://example.org/p/:slug")).toEqual({});
    expect(rule("/blog/:slug", "https://example.org/p/:slg")).toEqual({
      destination: "services.staticRuleUndeclaredPlaceholder",
    });
    // The host's port colon is not a placeholder.
    expect(rule("/old", "https://example.org:8443/x")).toEqual({});
  });
});
