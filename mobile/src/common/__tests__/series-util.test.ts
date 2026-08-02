import {
  alignSeries,
  filterSeriesByRange,
  normalizeSeries,
} from "../series-util";

const hour = 60 * 60 * 1000;
const iso = (offset: number) =>
  new Date(Date.UTC(2026, 0, 1) + offset * hour).toISOString();

describe("series utilities", () => {
  it("sorts, deduplicates, and drops malformed points", () => {
    expect(
      normalizeSeries([
        { timestamp: iso(1), value: 2 },
        { timestamp: "bad", value: 4 },
        { timestamp: iso(0), value: 1 },
        { timestamp: iso(1), value: 3 },
      ]),
    ).toEqual([
      { timestamp: iso(0), value: 1 },
      { timestamp: iso(1), value: 3 },
    ]);
  });

  it("anchors range filtering to the latest point", () => {
    const points = [
      { timestamp: iso(0), value: 1 },
      { timestamp: iso(10), value: 2 },
    ];
    expect(filterSeriesByRange(points, "6H")).toEqual([
      { timestamp: iso(10), value: 2 },
    ]);
  });

  it("aligns sparse series without inventing values", () => {
    expect(
      alignSeries([
        [{ timestamp: iso(0), value: 1 }],
        [{ timestamp: iso(1), value: 2 }],
      ]),
    ).toEqual({
      timestamps: [iso(0), iso(1)],
      values: [
        [1, null],
        [null, 2],
      ],
    });
  });
});
