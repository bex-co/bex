import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";
import {
  RESOURCE_ID_PREFIXES,
  isValidLabel,
  looksLikeResourceId,
} from "@/common/lib/utils/resource-label";

const XID = "abcdefghij0123456789";

describe("isValidLabel (w5/m153, core.CheckLabel + LooksLikeResourceID)", () => {
  it.each([
    "my-api",
    "My API",
    "My_DB",
    "Orders (EU) — primary",
    "数据库",
    "  padded  ",
    "x".repeat(100),
    ` ${"x".repeat(100)} `,
    "srv-short",
    `abc-${XID}`,
    `srv-${XID.toUpperCase()}`,
  ])("accepts %j", (value) => {
    expect(isValidLabel(value)).toBe(true);
  });

  it.each([
    "",
    "   ",
    "x".repeat(101),
    "tab\there",
    "line\nbreak",
    "edge\n",
    "nul\u0000",
    "del\u007f",
    "c1\u0085",
    "sep ",
    "sep ",
    `srv-${XID}`,
    `dpg-${XID}`,
    `red-${XID}`,
    `env-${XID}`,
    ` srv-${XID} `,
  ])("rejects %j", (value) => {
    expect(isValidLabel(value)).toBe(false);
  });

  it("counts code points, not UTF-16 units", () => {
    expect(isValidLabel("😀".repeat(100))).toBe(true);
    expect(isValidLabel("😀".repeat(101))).toBe(false);
  });
});

describe("looksLikeResourceId", () => {
  it("needs a registered prefix and a 20-char [0-9a-z] suffix", () => {
    expect(looksLikeResourceId(`srv-${XID}`)).toBe(true);
    expect(looksLikeResourceId(`srv-${XID}x`)).toBe(false);
    expect(looksLikeResourceId(`srv-${XID.slice(1)}`)).toBe(false);
    expect(looksLikeResourceId(`zzz-${XID}`)).toBe(false);
    expect(looksLikeResourceId(`srv-${XID.slice(1)}_`)).toBe(false);
    expect(looksLikeResourceId(XID)).toBe(false);
  });

  // The prefix list mirrors lego/backend/internal/id/id.go's registered kinds
  // (plus "env"); read it so a new kind cannot slip past the client.
  it("knows every prefix bex-api registers", () => {
    const source = readFileSync(
      `${process.cwd()}/../lego/backend/internal/id/id.go`,
      "utf8",
    );
    const prefixes = [...source.matchAll(/Kind\{prefix: "([a-z]+)"/g)].map(
      (m) => m[1],
    );
    expect(prefixes.length).toBeGreaterThan(30);
    expect(new Set([...prefixes, "env"])).toEqual(RESOURCE_ID_PREFIXES);
  });
});
