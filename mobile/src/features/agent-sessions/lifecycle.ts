// Pure, render-free lifecycle helpers for cloud coding-agent sessions
// (ADR047 / w11/m6). The backend owns the phase vocabulary (agentsessions
// models.go); this mirrors it as display metadata + supervision predicates so
// the phone never invents a phase and cancel eligibility stays consistent with
// the server's terminal states. Kept pure so the jest-lite runner can assert it.

export type SessionTone = "active" | "success" | "danger" | "neutral";

export interface SessionPhaseView {
  labelKey: string;
  tone: SessionTone;
  terminal: boolean;
  cancelable: boolean;
}

// The ten server phases (agentsessions/models.go:29-40), including the ADR059
// hibernation pair. Only non-terminal, non-canceling phases are cancelable — a
// completed/failed/canceled session cannot be canceled again, matching the
// server's idempotent Cancel. `hibernated` mirrors the server's finishedPhase
// (past all live work): it groups with the terminal states for ordering and
// shows a resting label, but the phone never offers destructive reclaim on it.
const PHASES: Record<string, SessionPhaseView> = {
  creating: {
    labelKey: "agentSessions.phase.creating",
    tone: "active",
    terminal: false,
    cancelable: true,
  },
  running: {
    labelKey: "agentSessions.phase.running",
    tone: "active",
    terminal: false,
    cancelable: true,
  },
  resuming: {
    labelKey: "agentSessions.phase.resuming",
    tone: "active",
    terminal: false,
    cancelable: true,
  },
  redispatching: {
    labelKey: "agentSessions.phase.redispatching",
    tone: "active",
    terminal: false,
    cancelable: true,
  },
  completed: {
    labelKey: "agentSessions.phase.completed",
    tone: "success",
    terminal: true,
    cancelable: false,
  },
  failed: {
    labelKey: "agentSessions.phase.failed",
    tone: "danger",
    terminal: true,
    cancelable: false,
  },
  canceling: {
    labelKey: "agentSessions.phase.canceling",
    tone: "neutral",
    terminal: false,
    cancelable: false,
  },
  canceled: {
    labelKey: "agentSessions.phase.canceled",
    tone: "neutral",
    terminal: true,
    cancelable: false,
  },
  // ADR059 (w2/m68). `hibernating` is the transient snapshot-upload window —
  // still a live session, so it stays cancelable and sorts with the active
  // group. `hibernated` is the durable pod-less resting state: not terminal on
  // the server (a Resume can rehydrate it), but past all live work, so it sorts
  // and gates like a terminal state and the supervision-only phone never offers
  // the snapshot-reclaiming Cancel on it.
  hibernating: {
    labelKey: "agentSessions.phase.hibernating",
    tone: "active",
    terminal: false,
    cancelable: true,
  },
  hibernated: {
    labelKey: "agentSessions.phase.hibernated",
    tone: "neutral",
    terminal: true,
    cancelable: false,
  },
};

const UNKNOWN: SessionPhaseView = {
  labelKey: "agentSessions.phase.unknown",
  tone: "neutral",
  terminal: false,
  cancelable: false,
};

export function sessionPhaseView(
  phase: string | null | undefined,
): SessionPhaseView {
  if (!phase) return UNKNOWN;
  return PHASES[phase] ?? UNKNOWN;
}

export function isTerminalPhase(phase: string | null | undefined): boolean {
  return sessionPhaseView(phase).terminal;
}

export function isCancelablePhase(phase: string | null | undefined): boolean {
  return sessionPhaseView(phase).cancelable;
}

export interface SessionOrderable {
  phase?: string | null;
  updatedAt?: string | null;
  createdAt?: string | null;
}

// Supervision ordering: active sessions first (they need attention now), then
// most-recently-updated. A missing timestamp sorts last within its group so a
// malformed row never jumps the list.
export function orderSessions<T extends SessionOrderable>(sessions: T[]): T[] {
  const rank = (s: T): number => (isTerminalPhase(s.phase) ? 1 : 0);
  const millis = (iso: string | null | undefined): number => {
    if (!iso) return -Infinity;
    const t = Date.parse(iso);
    return Number.isNaN(t) ? -Infinity : t;
  };
  return [...sessions].sort((a, b) => {
    const byActive = rank(a) - rank(b);
    if (byActive !== 0) return byActive;
    return (
      millis(b.updatedAt ?? b.createdAt) - millis(a.updatedAt ?? a.createdAt)
    );
  });
}
