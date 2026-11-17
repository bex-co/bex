import { describe, expect, it } from "vitest";
import { restoreBounds, restoreTarget, toLocalInput } from "../restore-time";

// The earliest restore point of the w21/020 probe database.
const EARLIEST = "2026-10-11T03:01:34Z";
const NOW = Date.parse("2026-11-10T04:00:00Z");

describe("restoreTarget (w21/020)", () => {
  const bounds = restoreBounds(EARLIEST, NOW);

  it("sends the instant a local wall time denotes, not the same digits as UTC", () => {
    const picked = toLocalInput(Date.parse("2026-10-20T12:30:00Z"));
    const result = restoreTarget(picked, bounds);
    expect(result).toEqual({ ok: true, iso: "2026-10-20T12:30:00Z" });
  });

  it("clamps the earliest point's displayed minute to the exact instant", () => {
    const result = restoreTarget(toLocalInput(Date.parse(EARLIEST)), bounds);
    expect(result).toEqual({ ok: true, iso: "2026-10-11T03:01:34Z" });
  });

  it("refuses a time before the window and one after now", () => {
    expect(
      restoreTarget(toLocalInput(Date.parse("2026-09-01T12:00:00Z")), bounds),
    ).toEqual({ ok: false, reason: "outOfWindow" });
    expect(
      restoreTarget(toLocalInput(Date.parse("2027-01-01T12:00:00Z")), bounds),
    ).toEqual({ ok: false, reason: "outOfWindow" });
  });

  it("leaves an empty pick as latest", () => {
    expect(restoreTarget("", bounds)).toEqual({ ok: true, iso: undefined });
  });
});

describe("restoreBounds' latest point (w7/m453)", () => {
  const bounds = restoreBounds(EARLIEST, NOW);

  it("ends ten minutes before now, as Render and bex-api do", () => {
    expect(bounds.latestMs).toBe(NOW - 10 * 60 * 1000);
    expect(bounds.max).toBe(toLocalInput(NOW - 10 * 60 * 1000));
  });

  it("refuses a time within ten minutes of now and accepts one just outside", () => {
    expect(restoreTarget(toLocalInput(NOW - 5 * 60 * 1000), bounds)).toEqual({
      ok: false,
      reason: "outOfWindow",
    });
    expect(restoreTarget(toLocalInput(NOW - 10 * 60 * 1000), bounds)).toEqual({
      ok: true,
      iso: "2026-11-10T03:50:00Z",
    });
  });
});
