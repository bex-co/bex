import { describe, expect, it } from "vitest";
import { isInvalidLabel } from "../resource-label";

// w21/036: the predicate the create/rename forms share with the server's rule.
describe("isInvalidLabel", () => {
  it("is false for blank (required-ness is each form's own rule) and for valid names", () => {
    expect(isInvalidLabel("")).toBe(false);
    expect(isInvalidLabel("   ")).toBe(false);
    expect(isInvalidLabel("shop")).toBe(false);
    expect(isInvalidLabel("受".repeat(100))).toBe(false);
  });

  it("is true for 101 code points and for control characters", () => {
    expect(isInvalidLabel("a".repeat(101))).toBe(true);
    expect(isInvalidLabel("受".repeat(101))).toBe(true);
    expect(isInvalidLabel("a\nb")).toBe(true);
    expect(isInvalidLabel("a\u0007b")).toBe(true);
  });
});
