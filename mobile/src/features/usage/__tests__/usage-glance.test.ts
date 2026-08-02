import {
  aggregateTotals,
  buildUsageGlance,
  formatUsageTotal,
} from "../usage-glance";

describe("mobile usage glance", () => {
  it("keeps partial totals visible with the common evidence watermark", () => {
    const glance = buildUsageGlance({
      summary: {
        period: "2026-08",
        coverage: {
          state: "partial",
          through: "2026-08-02T10:00:00Z",
          degradedSources: ["http", "other", " direct ", "http", null],
        },
        services: [
          {
            rows: [
              { kind: "egress_bytes", total: 2_048 },
              { kind: "instance_seconds", total: 3_600 },
            ],
          },
          { rows: [{ kind: "egress_bytes", total: 1_024 }] },
        ],
      },
    });

    expect(glance).toEqual({
      period: "2026-08",
      state: "partial",
      refreshUnavailable: false,
      through: "2026-08-02T10:00:00.000Z",
      degradedSources: ["direct", "http", "other"],
      totals: [
        { kind: "instance_seconds", total: 3_600 },
        { kind: "egress_bytes", total: 3_072 },
      ],
    });
  });

  it("does not present legacy totals as complete", () => {
    const glance = buildUsageGlance({
      summary: {
        period: "2026-08",
        coverage: null,
        services: [{ rows: [{ kind: "egress_bytes", total: 512 }] }],
      },
    });
    expect(glance.state).toBe("unknown");
    expect(glance.totals).toEqual([{ kind: "egress_bytes", total: 512 }]);
    expect(glance.through).toBe(null);
  });

  it("distinguishes proven healthy empty from unavailable and unknown", () => {
    expect(
      buildUsageGlance({
        summary: {
          coverage: { state: "complete" },
          services: [],
        },
      }).state,
    ).toBe("healthy-empty");
    expect(buildUsageGlance({ summary: null, unavailable: true }).state).toBe(
      "unavailable",
    );
    expect(buildUsageGlance({ summary: null }).state).toBe("unknown");
  });

  it("retains explicit zero but never invents it from missing totals", () => {
    const totals = aggregateTotals([
      {
        rows: [
          { kind: "instance_seconds", total: 0 },
          { kind: "egress_bytes", total: null },
          { kind: "build_seconds" },
          { kind: "storage_gb_seconds", total: Number.NaN },
          { kind: "negative", total: -1 },
        ],
      },
    ]);
    expect(totals).toEqual([{ kind: "instance_seconds", total: 0 }]);
    expect(
      buildUsageGlance({
        summary: {
          coverage: { state: "complete" },
          services: [{ rows: [{ kind: "instance_seconds", total: 0 }] }],
        },
      }).state,
    ).toBe("healthy-empty");
  });

  it("keeps cached evidence visible when refresh becomes unavailable", () => {
    const glance = buildUsageGlance({
      summary: {
        coverage: { state: "complete" },
        services: [{ rows: [{ kind: "egress_bytes", total: 256 }] }],
      },
      unavailable: true,
    });
    expect(glance.state).toBe("complete");
    expect(glance.refreshUnavailable).toBe(true);
    expect(glance.totals).toEqual([{ kind: "egress_bytes", total: 256 }]);
  });

  it("preserves future meter totals and bounds untrusted source annotations", () => {
    const sources = [
      ...Array.from({ length: 20 }, (_, index) => `source-${index}`),
      "provider returned selector={tenant=secret}",
      "line\nbreak",
      "x".repeat(49),
    ];
    const glance = buildUsageGlance({
      summary: {
        coverage: { state: "mystery", degradedSources: sources },
        services: [{ rows: [{ kind: "future_meter", total: 12.5 }] }],
      },
    });
    expect(glance.state).toBe("unknown");
    expect(glance.totals).toEqual([{ kind: "future_meter", total: 12.5 }]);
    expect(glance.degradedSources.length).toBe(8);
    for (const unsafe of [
      "provider returned selector={tenant=secret}",
      "line\nbreak",
      "x".repeat(49),
    ]) {
      expect(glance.degradedSources.includes(unsafe)).toBe(false);
    }
  });

  it("formats natural meter units without billing or price semantics", () => {
    expect(formatUsageTotal("instance_seconds", 3_600)).toBe("1 h");
    expect(formatUsageTotal("egress_bytes", 1_024)).toBe("1 KiB");
    expect(formatUsageTotal("storage_gb_seconds", 7_200)).toBe("2 GB-h");
    expect(formatUsageTotal("sandbox_compute_seconds", 3_600_000)).toBe(
      "1 vCPU-h",
    );
  });
});
