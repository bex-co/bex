import {
  isCancelablePhase,
  isTerminalPhase,
  orderSessions,
  sessionPhaseView,
} from "../lifecycle";

describe("agent-session lifecycle", () => {
  it("maps active phases to cancelable, non-terminal views", () => {
    for (const phase of ["creating", "running", "resuming", "redispatching"]) {
      const view = sessionPhaseView(phase);
      expect(view.terminal).toBe(false);
      expect(view.cancelable).toBe(true);
      expect(view.tone).toBe("active");
    }
  });

  it("marks completed/failed/canceled terminal and not cancelable", () => {
    expect(isTerminalPhase("completed")).toBe(true);
    expect(isTerminalPhase("failed")).toBe(true);
    expect(isTerminalPhase("canceled")).toBe(true);
    expect(isCancelablePhase("completed")).toBe(false);
    expect(isCancelablePhase("failed")).toBe(false);
    expect(isCancelablePhase("canceled")).toBe(false);
  });

  it("treats canceling as non-cancelable in-flight (no double cancel)", () => {
    expect(isCancelablePhase("canceling")).toBe(false);
    expect(isTerminalPhase("canceling")).toBe(false);
  });

  it("falls back to an unknown, non-cancelable view for junk phases", () => {
    const view = sessionPhaseView("nonsense");
    expect(view.labelKey).toBe("agentSessions.phase.unknown");
    expect(view.cancelable).toBe(false);
    expect(sessionPhaseView(null).labelKey).toBe("agentSessions.phase.unknown");
    expect(sessionPhaseView(undefined).cancelable).toBe(false);
  });

  it("orders active sessions before terminal, then by recency", () => {
    const ordered = orderSessions([
      { phase: "completed", updatedAt: "2026-01-03T00:00:00Z" },
      { phase: "running", updatedAt: "2026-01-01T00:00:00Z" },
      { phase: "failed", updatedAt: "2026-01-05T00:00:00Z" },
      { phase: "creating", updatedAt: "2026-01-02T00:00:00Z" },
    ]);
    // Two active first (newest active first), then two terminal (newest first).
    expect(ordered.map((s) => s.phase)).toEqual([
      "creating",
      "running",
      "failed",
      "completed",
    ]);
  });

  it("sorts a malformed timestamp last within its group, never crashing", () => {
    const ordered = orderSessions([
      { phase: "running", updatedAt: "not-a-date" },
      { phase: "running", updatedAt: "2026-01-01T00:00:00Z" },
    ]);
    expect(ordered[0].updatedAt).toBe("2026-01-01T00:00:00Z");
    expect(ordered[1].updatedAt).toBe("not-a-date");
  });
});
