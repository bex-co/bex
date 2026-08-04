// Maps bex-api's Render-shaped GraphQL `AgentSession` (the generated
// AgentSessionFields fragment) onto the normalized `AgentSessionView`, deriving
// the phase-keyed helpers once so pages never re-encode the lifecycle enum.

import type {
  AgentSessionFieldsFragment,
  AgentSessionMintFieldsFragment,
} from "@/graphql/definitions";
import type {
  AgentSessionDeliveryMode,
  AgentSessionEvidenceView,
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

/** Idle, non-canceled phases the GraphQL steer/resume redispatch path accepts. */
const STEERABLE_PHASES: ReadonlySet<AgentSessionPhase> = new Set([
  "completed",
  "failed",
]);

/** True for completed/failed/canceled — a settled session. */
export function isTerminalPhase(phase: string): boolean {
  return TERMINAL_PHASES.has(phase as AgentSessionPhase);
}

/** True when the steer/resume mutation would accept a new prompt (idle session). */
export function isSteerablePhase(phase: string): boolean {
  return STEERABLE_PHASES.has(phase as AgentSessionPhase);
}

function toEvidenceView(
  evidence: AgentSessionFieldsFragment["evidence"],
): AgentSessionEvidenceView | null {
  if (!evidence) return null;
  return {
    commandLog: evidence.commandLog ?? [],
    testOutput: evidence.testOutput ?? [],
    outputTail: evidence.outputTail ?? null,
    changedFiles: evidence.changedFiles ?? [],
    commits: evidence.commits ?? null,
    truncated: evidence.truncated ?? false,
  };
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
    phase,
    status: wire.status,
    headSha: wire.headSha ?? null,
    prUrl: wire.prUrl ?? null,
    prNumber: wire.prNumber ?? null,
    evidence: toEvidenceView(wire.evidence),
    turns: wire.turns ?? 0,
    deliveryMode:
      (wire.deliveryMode as AgentSessionDeliveryMode | null) ?? null,
    failureReason: wire.failureReason ?? null,
    createdAt: wire.createdAt,
    updatedAt: wire.updatedAt,
    canceledAt: wire.canceledAt ?? null,
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
