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
  return {
    id: wire.id,
    ownerId: wire.ownerId,
    repo: wire.repo,
    branch: wire.branch,
    agentConfig: {
      agent: wire.agentConfig.agent,
      model: wire.agentConfig.model ?? null,
      modelEndpoint: wire.agentConfig.modelEndpoint ?? null,
      task: wire.agentConfig.task,
      template: wire.agentConfig.template ?? null,
    },
    sandboxId: wire.sandboxId ?? null,
    sshAddress: wire.sshAddress ?? null,
    phase,
    status: wire.status,
    headSha: wire.headSha ?? null,
    prUrl: wire.prUrl ?? null,
    prNumber: wire.prNumber ?? null,
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

/** The i18n keys a status phrase resolves to; the settled phases reuse the
 *  phase chip's own copy rather than restating it. */
export type AgentSessionStatusPhraseKey =
  | "agentSessions.statusPhrase.prReady"
  | "agentSessions.statusPhrase.working"
  | "agentSessions.phase.completed"
  | "agentSessions.phase.failed"
  | "agentSessions.phase.canceled";

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
      return view.prNumber != null
        ? "agentSessions.statusPhrase.prReady"
        : "agentSessions.phase.completed";
    case "failed":
      return "agentSessions.phase.failed";
    case "canceled":
    case "canceling":
      return "agentSessions.phase.canceled";
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
