import { describe, expect, it } from "vitest";
import { environmentLookupStatus } from "@/features/env-groups/lib/scope";

// w4/304: only a settled, successful lookup may call an absent id "Unknown".
describe("environmentLookupStatus", () => {
  const failed = new Error("unavailable");

  it.each([
    ["first read in flight", { ready: false, loading: true }, "loading"],
    ["workspace not resolved yet", { ready: false, loading: false }, "loading"],
    [
      "first read failed",
      { ready: false, loading: false, error: failed },
      "error",
    ],
    [
      "refresh in flight after a success",
      { ready: true, loading: true },
      "loading",
    ],
    ["settled success", { ready: true, loading: false }, undefined],
    [
      "refresh failed after a success (last-good index stands)",
      { ready: true, loading: false, error: failed },
      undefined,
    ],
  ] as const)("%s -> %s", (_, index, expected) => {
    expect(environmentLookupStatus(index)).toBe(expected);
  });
});
