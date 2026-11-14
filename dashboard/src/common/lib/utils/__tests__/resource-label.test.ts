import { describe, expect, it } from "vitest";
import { isValidLabel } from "@/common/lib/utils/resource-label";

const XID = "abcdefghij0123456789";

describe("isValidLabel (w5/m153, core.CheckLabel)", () => {
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
    // ID-shaped labels are accepted, as on Render (w5/m118 reversed w8/049).
    `srv-${XID}`,
    `dpg-${XID}`,
    ` red-${XID} `,
    // The form sends value.trim(), which strips whitespace at an edge, line
    // terminators included, before the control-character rule looks at it.
    "edge\n",
    "\nedge",
    "sep\u2028",
    "sep\u2029",
  ])("accepts %j", (value) => {
    expect(isValidLabel(value)).toBe(true);
  });

  it.each([
    "",
    "   ",
    "x".repeat(101),
    "tab\there",
    "line\nbreak",
    "nul\u0000",
    "del\u007f",
    "c1\u0085",
    "in\u2028side",
    "in\u2029side",
  ])("rejects %j", (value) => {
    expect(isValidLabel(value)).toBe(false);
  });

  it("counts code points, not UTF-16 units", () => {
    expect(isValidLabel("😀".repeat(100))).toBe(true);
    expect(isValidLabel("😀".repeat(101))).toBe(false);
  });
});
