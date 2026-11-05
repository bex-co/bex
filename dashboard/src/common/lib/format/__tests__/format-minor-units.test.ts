import { describe, expect, it } from "vitest";
import { formatMinorUnits } from "../format-minor-units";

describe("formatMinorUnits", () => {
  it("scales by the currency's own fraction digits", () => {
    expect(formatMinorUnits(300, "usd", "en")).toBe("$3.00");
    expect(formatMinorUnits(300, "JPY", "en")).toBe("¥300");
  });

  it("formats for the active locale with the narrow symbol", () => {
    expect(formatMinorUnits(123456, "usd", "en")).toBe("$1,234.56");
    expect(formatMinorUnits(123456, "usd", "de")).toBe("1.234,56\u00a0$");
    expect(formatMinorUnits(300, "usd", "zh")).toBe("$3.00");
  });

  it("falls back for an unknown currency code instead of throwing", () => {
    expect(formatMinorUnits(300, "not-a-code", "en")).toBe("3.00 NOT-A-CODE");
  });
});
