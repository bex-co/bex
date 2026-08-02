import { chartDomain, generateTicks, nearestIndex, zipSeries } from "../utils";

describe("chart utilities", () => {
  it("creates stable domains for empty, flat, and signed data", () => {
    expect(chartDomain([])).toEqual([0, 1]);
    expect(chartDomain([5, 5])).toEqual([4.5, 5.5]);
    expect(chartDomain([4, -2], true)).toEqual([-2, 4]);
  });

  it("zips only matching finite values", () => {
    expect(zipSeries(["CPU", "RAM", "extra"], [12, Number.NaN])).toEqual([
      { label: "CPU", value: 12 },
    ]);
  });

  it("maps scrub positions to bounded indexes", () => {
    expect(nearestIndex(-10, 100, 3, 10)).toBe(0);
    expect(nearestIndex(50, 100, 3, 10)).toBe(1);
    expect(nearestIndex(200, 100, 3, 10)).toBe(2);
  });

  it("generates requested ticks and handles invalid counts", () => {
    expect(generateTicks(0, 100, 5)).toEqual([0, 25, 50, 75, 100]);
    expect(generateTicks(0, 1, 0)).toEqual([]);
  });
});
