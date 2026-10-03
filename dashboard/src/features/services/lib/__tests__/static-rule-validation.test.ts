import { describe, expect, it } from "vitest";
import {
  headerErrors,
  humanizeRuleError,
  routeErrors,
} from "../static-rule-validation";

// w4/145: the rules bex-api enforces, checked in the row before Save.
describe("static-site rule validation", () => {
  it("marks a redirect destination that is not a site path", () => {
    expect(
      routeErrors({
        type: "redirect",
        source: "/old",
        destination: "https://example.org/landing",
      }),
    ).toEqual({ destination: "services.staticRulePathSlash" });
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
