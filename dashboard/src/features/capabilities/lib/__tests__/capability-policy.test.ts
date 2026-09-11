import { describe, it, expect } from "vitest";
import {
  allowsAction,
  confirmedDenied,
  downgradeDetected,
  grantReasonKey,
  toSnapshot,
  checkingCapabilities,
  unavailableCapabilities,
  CAPABILITY_FRESHNESS_MS,
} from "@/features/capabilities/lib/capability-policy";

describe("capability-policy", () => {
  const grants = [
    { action: "can_create", outcome: "allowed", reason: null },
    {
      action: "can_operate",
      outcome: "denied",
      reason: "insufficient_permission",
    },
    { action: "can_view_sensitive", outcome: "unavailable", reason: "outage" },
    { action: "unknown_action", outcome: "allowed", reason: null },
  ];

  it("normalizes grants and drops unknown actions", () => {
    const snap = toSnapshot("tea-1", grants, "ADMIN", 1_000);
    expect(snap.grants.can_create).toBe("allowed");
    expect(snap.grants.can_operate).toBe("denied");
    expect(snap.grants.can_view_sensitive).toBe("unavailable");
    expect(snap.grants).not.toHaveProperty("unknown_action");
  });

  it("allowsAction is affirmative-only and freshness-bound", () => {
    const ready = {
      status: "ready" as const,
      snapshot: toSnapshot("tea-1", grants, null, 1_000),
    };
    expect(allowsAction(ready, "tea-1", "can_create", 1_000)).toBe(true);
    expect(
      allowsAction(ready, "tea-1", "can_create", 1_000 + CAPABILITY_FRESHNESS_MS),
    ).toBe(false);
    expect(allowsAction(ready, "tea-2", "can_create", 1_000)).toBe(false);
    expect(allowsAction(checkingCapabilities, "tea-1", "can_create")).toBe(
      false,
    );
    expect(allowsAction(unavailableCapabilities, "tea-1", "can_create")).toBe(
      false,
    );
  });

  it("confirmedDenied never fires for unavailable outcomes", () => {
    const ready = {
      status: "ready" as const,
      snapshot: toSnapshot("tea-1", grants, null, 1_000),
    };
    expect(confirmedDenied(ready, "tea-1", "can_operate")).toBe(true);
    expect(confirmedDenied(ready, "tea-1", "can_view_sensitive")).toBe(false);
    expect(confirmedDenied(unavailableCapabilities, "tea-1", "can_operate")).toBe(
      false,
    );
  });

  it("grantReasonKey distinguishes role denial from recovery copy", () => {
    const ready = {
      status: "ready" as const,
      snapshot: toSnapshot("tea-1", grants, null, 1_000),
    };
    expect(grantReasonKey(ready, "tea-1", "can_operate", 1_000)).toBe(
      "capabilities.reasonCanOperate",
    );
    expect(grantReasonKey(ready, "tea-1", "can_view_sensitive", 1_000)).toBe(
      "capabilities.grantUnavailable",
    );
    expect(
      grantReasonKey(unavailableCapabilities, "tea-1", "can_create"),
    ).toBe("capabilities.grantUnavailable");
    expect(
      grantReasonKey(ready, "tea-1", "can_create", 1_000 + CAPABILITY_FRESHNESS_MS),
    ).toBe("capabilities.grantStale");
  });

  it("downgradeDetected requires two ready snapshots", () => {
    const before = {
      status: "ready" as const,
      snapshot: toSnapshot(
        "tea-1",
        [{ action: "can_create", outcome: "allowed", reason: null }],
        null,
        1,
      ),
    };
    const after = {
      status: "ready" as const,
      snapshot: toSnapshot(
        "tea-1",
        [
          {
            action: "can_create",
            outcome: "denied",
            reason: "insufficient_permission",
          },
        ],
        null,
        2,
      ),
    };
    expect(downgradeDetected(before, after)).toBe(true);
    expect(downgradeDetected(before, unavailableCapabilities)).toBe(false);
  });
});
