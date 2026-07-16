import { describe, expect, it } from "vitest";
import { logRangeFromSearch, parseLogSearch } from "../log-search";

describe("log range URL state", () => {
  it("round-trips a supported selection through route validation", () => {
    const restored = parseLogSearch({ range: "12h" });
    expect(restored).toEqual({ range: "12h" });
    expect(logRangeFromSearch(restored).id).toBe("12h");
  });

  it("drops malformed values and restores the one-hour default", () => {
    const restored = parseLogSearch({ range: "31d" });
    expect(restored).toEqual({});
    expect(logRangeFromSearch(restored).id).toBe("1h");
  });
});
