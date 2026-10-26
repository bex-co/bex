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
    expect(
      headerErrors({ path: "/*", name: "X-Test", value: "a\nb" }),
    ).toEqual({ value: "services.staticRuleHeaderValueInvalid" });
    expect(
      headerErrors({ path: "/*", name: "X-Test", value: "a\tb" }),
    ).toEqual({});
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
