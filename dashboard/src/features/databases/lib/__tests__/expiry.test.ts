import { describe, expect, it } from "vitest";
import {
  freeDeletionAt,
  freeExpiryState,
  utcDate,
} from "@/features/databases/lib/expiry";

// w8/m126: the dashboard reads bex-api's expiresAt with ADR009's leads.
describe("free database expiry", () => {
  const expiresAt = "2026-12-02T09:30:00Z";
  const at = Date.parse(expiresAt);
  const day = 24 * 60 * 60 * 1000;

  it("classifies paid, active, soon and expired", () => {
    expect(freeExpiryState(null, at)).toBe("none");
    expect(freeExpiryState(undefined, at)).toBe("none");
    expect(freeExpiryState("not a date", at)).toBe("none");
    expect(freeExpiryState(expiresAt, at - 7 * day - 1)).toBe("active");
    expect(freeExpiryState(expiresAt, at - 7 * day)).toBe("soon");
    expect(freeExpiryState(expiresAt, at - 1)).toBe("soon");
    expect(freeExpiryState(expiresAt, at)).toBe("expired");
  });

  it("deletes 14 days after expiry and names UTC dates", () => {
    expect(freeDeletionAt(expiresAt)).toBe("2026-12-16T09:30:00.000Z");
    expect(utcDate(expiresAt)).toBe("2026-12-02 UTC");
  });
});
