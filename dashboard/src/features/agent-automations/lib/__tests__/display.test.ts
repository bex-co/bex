import { describe, expect, it } from "vitest";
import {
  automationActions,
  automationStatus,
  outcomeDisplay,
  pauseToggleKind,
  pausedExplainKey,
  pausedReasonKey,
  runSessionStatus,
} from "@/features/agent-automations/lib/display";

describe("outcomeDisplay (ADR101 D3/D7)", () => {
  it("maps every known outcome to its label and badge", () => {
    expect(outcomeDisplay("started")).toMatchObject({
      labelKey: "agentAutomations.outcome.started",
      variant: "success",
      known: true,
    });
    expect(outcomeDisplay("claimed").variant).toBe("secondary");
    expect(outcomeDisplay("skipped_overlap").variant).toBe("outline");
    expect(outcomeDisplay("failed_create").variant).toBe("destructive");
    expect(outcomeDisplay("denied").variant).toBe("destructive");
  });

  it("marks an outcome it doesn't know so the caller shows the raw word", () => {
    expect(outcomeDisplay("brand_new")).toMatchObject({ known: false });
  });
});

// w10/m497: a started run's session waits for a sandbox machine (queued, not
// dropped) or did not start for a platform cause (not a broken template).
describe("runSessionStatus", () => {
  const session = (over: object = {}) => ({
    state: "working",
    waitingFor: null,
    waitingSince: null,
    failureReasonCode: null,
    failureCause: null,
    ...over,
  });

  it("reads a started run whose session waits for a machine as waiting", () => {
    expect(
      runSessionStatus({
        outcome: "started",
        session: session({
          waitingFor: "sandbox_capacity",
          waitingSince: "2026-11-18T12:00:00Z",
        }),
      }),
    ).toEqual({ kind: "waiting" });
  });

  it("reads a started run whose sandbox did not start as did-not-start, with the cause", () => {
    const failed = (failureCause: string | null) =>
      runSessionStatus({
        outcome: "started",
        session: session({
          state: "failed",
          failureReasonCode: "SANDBOX_START_FAILED",
          failureCause,
        }),
      });
    expect(failed("pool_full")).toEqual({
      kind: "did_not_start",
      cause: "pool_full",
    });
    expect(failed("image_pull")).toMatchObject({ cause: "image_pull" });
    // No cause, or one this build does not know: a platform issue.
    expect(failed(null)).toMatchObject({ cause: "unknown" });
    expect(failed("solar_flare")).toMatchObject({ cause: "unknown" });
  });

  it("is nothing for any other run or session", () => {
    // A broken template never started a session.
    expect(
      runSessionStatus({ outcome: "failed_create", session: null }),
    ).toBeNull();
    expect(runSessionStatus({ outcome: "started", session: null })).toBeNull();
    expect(
      runSessionStatus({ outcome: "started", session: session() }),
    ).toBeNull();
    expect(
      runSessionStatus({
        outcome: "started",
        session: session({ state: "failed", failureReasonCode: "OTHER" }),
      }),
    ).toBeNull();
    expect(
      runSessionStatus({
        outcome: "started",
        session: session({
          state: "failed",
          failureReasonCode: "SANDBOX_CAPACITY_LIMIT",
        }),
      }),
    ).toBeNull();
    // A wait the session no longer has once it settled.
    expect(
      runSessionStatus({
        outcome: "started",
        session: session({
          state: "succeeded",
          waitingFor: "sandbox_capacity",
        }),
      }),
    ).toBeNull();
  });
});

describe("pause reasons", () => {
  it("names the three platform reasons and falls back for others", () => {
    expect(pausedReasonKey("repeated_failures")).toBe(
      "agentAutomations.paused.repeated_failures",
    );
    expect(pausedReasonKey("creator_lost_access")).toBe(
      "agentAutomations.paused.creator_lost_access",
    );
    expect(pausedReasonKey("template_invalid")).toBe(
      "agentAutomations.paused.template_invalid",
    );
    expect(pausedReasonKey("zzz")).toBe("agentAutomations.paused.unknown");
    expect(pausedReasonKey("")).toBeNull();
    expect(pausedExplainKey("template_invalid")).toBe(
      "agentAutomations.pausedExplain.template_invalid",
    );
  });

  it("derives status and the toggle's direction", () => {
    expect(
      automationStatus({ enabled: true, nextFireAt: "2026-12-01T00:00:00Z" }),
    ).toBe("active");
    expect(automationStatus({ enabled: false })).toBe("paused");
    expect(
      automationStatus({ enabled: true, pausedReason: "template_invalid" }),
    ).toBe("paused");
    expect(automationStatus({ enabled: true, nextFireAt: null })).toBe("ended");
    expect(pauseToggleKind({ enabled: true })).toBe("pause");
    expect(pauseToggleKind({ enabled: false })).toBe("resume");
    expect(
      pauseToggleKind({ enabled: true, pausedReason: "repeated_failures" }),
    ).toBe("resume");
  });
});

describe("automationActions (ADR101 D4: who may do what)", () => {
  it("the creator edits, runs, pauses and deletes but cannot take over their own", () => {
    expect(
      automationActions({ canCreate: true, canManage: false, isCreator: true }),
    ).toEqual({ edit: true, operate: true, remove: true, takeOver: false });
  });

  it("an admin who is not the creator operates and takes over but does not edit", () => {
    expect(
      automationActions({ canCreate: true, canManage: true, isCreator: false }),
    ).toEqual({ edit: false, operate: true, remove: true, takeOver: true });
  });

  it("an admin who is the creator edits and cannot take over", () => {
    expect(
      automationActions({ canCreate: true, canManage: true, isCreator: true }),
    ).toEqual({ edit: true, operate: true, remove: true, takeOver: false });
  });

  it("another developer can do nothing", () => {
    expect(
      automationActions({
        canCreate: true,
        canManage: false,
        isCreator: false,
      }),
    ).toEqual({ edit: false, operate: false, remove: false, takeOver: false });
  });

  it("a creator who lost the developer role cannot edit or operate", () => {
    expect(
      automationActions({
        canCreate: false,
        canManage: false,
        isCreator: true,
      }),
    ).toEqual({ edit: false, operate: false, remove: false, takeOver: false });
  });
});
