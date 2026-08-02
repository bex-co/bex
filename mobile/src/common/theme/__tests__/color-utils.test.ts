import { withAlpha } from "../color-utils";

describe("color-utils", () => {
  describe("withAlpha", () => {
    it("should append an 8-bit alpha channel to a 6-digit hex", () => {
      // Two representative tint strengths used by status surfaces.
      expect(withAlpha("#b8894b", 0.14)).toBe("#b8894b24");
      expect(withAlpha("#6fb0e8", 0.22)).toBe("#6fb0e838");
    });

    it("should pad single-digit alpha bytes", () => {
      expect(withAlpha("#ffffff", 0.02)).toBe("#ffffff05");
    });

    it("should handle the alpha extremes", () => {
      expect(withAlpha("#000000", 0)).toBe("#00000000");
      expect(withAlpha("#000000", 1)).toBe("#000000ff");
    });

    it("should clamp out-of-range alpha", () => {
      expect(withAlpha("#000000", -1)).toBe("#00000000");
      expect(withAlpha("#000000", 5)).toBe("#000000ff");
    });

    it("should accept uppercase hex", () => {
      expect(withAlpha("#B8894B", 1)).toBe("#B8894Bff");
    });

    it("should pass through colors it cannot extend", () => {
      expect(withAlpha("rgba(0, 0, 0, 0.5)", 0.5)).toBe("rgba(0, 0, 0, 0.5)");
      expect(withAlpha("#fff", 0.5)).toBe("#fff");
      expect(withAlpha("#b8894b24", 0.5)).toBe("#b8894b24");
      expect(withAlpha("", 0.5)).toBe("");
    });
  });
});
