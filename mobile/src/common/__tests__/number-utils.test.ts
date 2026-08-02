import { formatBytes, formatPercent, shortNumber } from "../number-utils";

describe("number utilities", () => {
  it("formats operational values", () => {
    expect(shortNumber(1250)).toBe("1.3K");
    expect(formatPercent(42.25)).toBe("42.3%");
    expect(formatBytes(1536)).toBe("1.5 KiB");
  });

  it("fails honestly for invalid values", () => {
    expect(formatPercent(Number.NaN)).toBe("—");
    expect(formatBytes(-1)).toBe("—");
  });
});
