import { describe, expect, it } from "vitest";
import type { RangeSearch } from "@/features/metrics/lib/range";
import { Route } from "../databases.$databaseId";

describe("database detail tab search contract", () => {
  const validate = Route.options.validateSearch as (
    search: Record<string, unknown>,
  ) => { tab?: "logs" } & RangeSearch;

  it("keeps the directly linkable Logs tab and drops unknown tab values", () => {
    expect(validate({ tab: "logs" })).toEqual({ tab: "logs" });
    expect(validate({ tab: "metrics" })).toEqual({});
    expect(validate({})).toEqual({});
  });

  it("round-trips the log viewer's picked range through the URL (w6/065)", () => {
    // A preset survives alongside the tab key…
    expect(validate({ tab: "logs", range: "24h" })).toEqual({
      tab: "logs",
      range: "24h",
    });
    // …a custom window keeps its bounds…
    expect(
      validate({
        tab: "logs",
        range: "custom",
        rangeStart: "2026-07-01T00:00:00.000Z",
        rangeEnd: "2026-07-01T06:00:00.000Z",
      }),
    ).toEqual({
      tab: "logs",
      range: "custom",
      rangeStart: "2026-07-01T00:00:00.000Z",
      rangeEnd: "2026-07-01T06:00:00.000Z",
    });
    // …and malformed ranges drop out (default behavior preserved).
    expect(validate({ tab: "logs", range: "6h" })).toEqual({ tab: "logs" });
    expect(validate({ tab: "logs", range: "custom" })).toEqual({ tab: "logs" });
    // The Logs page's `r` Render alias is deliberately not honored here.
    expect(validate({ tab: "logs", r: "15m" })).toEqual({ tab: "logs" });
  });
});
