// Maps bex-api's Render-shaped GraphQL `AgentSession` (the generated
// AgentSessionFields fragment) onto the normalized `AgentSessionView`, deriving
// the phase-keyed helpers once so pages never re-encode the lifecycle enum.

import type {
  AgentSessionFieldsFragment,
  AgentSessionMintFieldsFragment,
} from "@/graphql/definitions";
import type {
  AgentSessionDeliveryMode,
  AgentSessionPhase,
  AgentSessionTicket,
  AgentSessionView,
} from "@/features/agent-sessions/types";

/** Phases after which the session will not change on its own. */
const TERMINAL_PHASES: ReadonlySet<AgentSessionPhase> = new Set([
  "completed",
  "failed",
  "canceled",
]);

/** Idle phases the GraphQL steer/resume path accepts (hibernated rehydrates). */
const STEERABLE_PHASES: ReadonlySet<AgentSessionPhase> = new Set([
  "completed",
  "failed",
  "hibernated",
]);

/** True for completed/failed/canceled — a settled session. */
export function isTerminalPhase(phase: string): boolean {
  return TERMINAL_PHASES.has(phase as AgentSessionPhase);
}

/** True when the steer/resume mutation would accept a new prompt (idle session). */
export function isSteerablePhase(phase: string): boolean {
  return STEERABLE_PHASES.has(phase as AgentSessionPhase);
}

/** Projects one wire `AgentSession` onto its normalized view. */
export function toAgentSessionView(
  wire: AgentSessionFieldsFragment,
): AgentSessionView {
  const phase = wire.phase as AgentSessionPhase;
  // agentConfig is non-null in a fully-materialized wire session, but a partial
  // cache entry (e.g. a create/steer mutation result read back mid-flight, before
  // the full fragment lands) can omit it. Default it rather than throw — a missing
  // config must never white-screen the whole session page.
  const config = wire.agentConfig ?? null;
  return {
    id: wire.id,
    ownerId: wire.ownerId,
    repo: wire.repo,
    branch: wire.branch,
    agentConfig: {
      agent: config?.agent ?? "",
      model: config?.model ?? null,
      modelEndpoint: config?.modelEndpoint ?? null,
      task: config?.task ?? "",
      template: config?.template ?? null,
    },
    sandboxId: wire.sandboxId ?? null,
    sshAddress: wire.sshAddress ?? null,
    phase,
    status: wire.status,
    headSha: wire.headSha ?? null,
    prUrl: wire.prUrl || null,
    // A real GitHub PR number is ≥ 1; the wire default 0 (no PR — a repo-less
    // chat-only session, or one that pushed nothing) normalizes to null so the
    // "PR is ready" phrase and PR card key off genuine delivery, not 0.
    prNumber: wire.prNumber || null,
    turns: wire.turns ?? 0,
    deliveryMode:
      (wire.deliveryMode as AgentSessionDeliveryMode | null) ?? null,
    failureReason: wire.failureReason ?? null,
    createdAt: wire.createdAt,
    updatedAt: wire.updatedAt,
    canceledAt: wire.canceledAt ?? null,
    pinned: wire.pinned ?? false,
    snapshotBytes: wire.snapshotBytes ?? 0,
    hibernatedAt: wire.hibernatedAt ?? null,
    retainUntil: wire.retainUntil ?? null,
    archivedAt: wire.archivedAt ?? null,
    isHibernated: phase === "hibernated",
    isArchived: wire.archivedAt != null,
    isFinished: isTerminalPhase(phase) || phase === "hibernated",
    isTerminal: isTerminalPhase(phase),
    isSteerable: isSteerablePhase(phase),
  };
}

/** Maps a nullable list of wire sessions to views, dropping any null entries. */
export function toAgentSessionViews(
  wire: ReadonlyArray<AgentSessionFieldsFragment | null> | null | undefined,
): AgentSessionView[] {
  return (wire ?? []).flatMap((s) => (s ? [toAgentSessionView(s)] : []));
}

/** Maps a mint-op payload (core fields + ticket/url/expiresAt) to its ticket. */
export function toAgentSessionTicket(
  wire: AgentSessionFieldsFragment & AgentSessionMintFieldsFragment,
): AgentSessionTicket {
  return {
    session: toAgentSessionView(wire),
    ticket: wire.ticket ?? null,
    url: wire.url ?? null,
    streamUrl: wire.streamUrl ?? null,
    expiresAt: wire.expiresAt ?? null,
  };
}

/** Human-readable snapshot storage size for the hibernation cost display
 *  (ADR059 D6). Binary units; one decimal above KiB. 0 ⇒ "0 B". */
export function formatSnapshotBytes(bytes: number): string {
  if (!bytes || bytes < 0) return "0 B";
  const units = ["B", "KiB", "MiB", "GiB", "TiB"];
  let value = bytes;
  let unit = 0;
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024;
    unit += 1;
  }
  return `${unit === 0 ? value : value.toFixed(1)} ${units[unit]}`;
}

/** A session's display name — its task prompt, falling back to the raw id. */
export function sessionTitle(
  view: Pick<AgentSessionView, "id" | "agentConfig">,
): string {
  return view.agentConfig.task || view.id;
}

/** Bounded title for list rows — caps raw task text to avoid DOM/ARIA bloat. */
export const SESSION_TITLE_MAX = 140;
export function sessionTitleShort(
  view: Pick<AgentSessionView, "id" | "agentConfig">,
  maxLen: number = SESSION_TITLE_MAX,
): string {
  const raw = view.agentConfig.task || view.id;
  if (raw.length <= maxLen) return raw;
  return raw.slice(0, maxLen).trimEnd() + "…";
}

/**
 * The reason text a failed session's callout should show, or null when the
 * session carries nothing worth reading and the caller should use its generic
 * copy instead.
 *
 * `failureReason` is the Completer's named reason and `status` the lifecycle
 * line a background-provisioning failure stamps, but neither is guaranteed to
 * be informative. Two values in particular are noise: a reason that merely
 * restates the phase ("failed" — the callout's own title already says that),
 * and the literal `[object Object]`, which sessions failed before the driver
 * learned to describe a rejected JSON-RPC error (its SDK rejects with the raw
 * protocol object, which `String()` coerced to exactly that). Both are dead
 * text on a callout whose only job is to say what went wrong.
 */
export function agentSessionFailureReason(
  view: Pick<AgentSessionView, "phase" | "failureReason" | "status">,
): string | null {
  for (const candidate of [view.failureReason, view.status]) {
    const reason = candidate?.trim() ?? "";
    if (reason === "" || reason === "[object Object]" || reason === view.phase) {
      continue;
    }
    return reason;
  }
  return null;
}

/**
 * True when a failed session's cause is a sandbox capacity / plan-limit refusal
 * (bex-api records `sandbox.CapacityFailureReason` = "sandbox capacity reached"
 * on either `failureReason` or the lifecycle `status`). The failure callout keys
 * an "Upgrade plan" action off this instead of a dead-end retry.
 */
export function isSandboxCapacityFailure(
  view: Pick<AgentSessionView, "phase" | "failureReason" | "status">,
): boolean {
  if (view.phase !== "failed") return false;
  const reason = `${view.failureReason ?? ""} ${view.status ?? ""}`.toLowerCase();
  return reason.includes("sandbox capacity");
}

/** The i18n keys a status phrase resolves to; the settled phases reuse the
 *  phase chip's own copy rather than restating it. */
export type AgentSessionStatusPhraseKey =
  | "agentSessions.statusPhrase.prReady"
  | "agentSessions.statusPhrase.working"
  | "agentSessions.phase.completed"
  | "agentSessions.phase.failed"
  | "agentSessions.phase.canceled"
  | "agentSessions.phase.hibernated"
  | "agentSessions.phase.hibernating";

/**
 * Phase + PR presence → the sidebar's human status phrase (Devin's "PR is
 * ready • 1" shape). The two that aren't just the phase restated: a completed
 * session carrying a PR reads "PR is ready", and every still-converging phase
 * (creating/running/resuming/redispatching) collapses to "Working…".
 */
export function agentSessionStatusPhraseKey(
  view: Pick<AgentSessionView, "phase" | "prNumber">,
): AgentSessionStatusPhraseKey {
  switch (view.phase) {
    case "completed":
      return view.prNumber != null && view.prNumber !== 0
        ? "agentSessions.statusPhrase.prReady"
        : "agentSessions.phase.completed";
    case "failed":
      return "agentSessions.phase.failed";
    case "canceled":
    case "canceling":
      return "agentSessions.phase.canceled";
    case "hibernated":
      return "agentSessions.phase.hibernated";
    case "hibernating":
      return "agentSessions.phase.hibernating";
    default:
      return "agentSessions.statusPhrase.working";
  }
}

/**
 * Elapsed session wall-clock in milliseconds: creation → the session's end
 * (canceledAt, else the last update once terminal) or → `nowMs` while it's still
 * running. Returns 0 when the timestamps are unparseable. `nowMs` is injected so
 * callers stay deterministic under test and can drive a live-ticking display.
 */
export function agentSessionDurationMs(
  view: AgentSessionView,
  nowMs: number = Date.now(),
): number {
  const start = Date.parse(view.createdAt);
  if (Number.isNaN(start)) return 0;
  const endSource =
    view.canceledAt ?? (view.isTerminal ? view.updatedAt : null);
  const end = endSource ? Date.parse(endSource) : nowMs;
  const resolvedEnd = Number.isNaN(end) ? nowMs : end;
  return Math.max(0, resolvedEnd - start);
}
