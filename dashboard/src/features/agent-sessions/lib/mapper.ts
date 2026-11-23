// Maps bex-api's Render-shaped GraphQL `AgentSession` (the generated
// AgentSessionFields fragment) onto the normalized `AgentSessionView`, deriving
// the phase-keyed helpers once so pages never re-encode the lifecycle enum.

import type {
  AgentSessionFieldsFragment,
  AgentSessionMintFieldsFragment,
  AgentSessionQueuedMessageFieldsFragment,
  AgentSessionTurnInterruptsQuery,
} from "@/graphql/definitions";
import {
  parseAgentSessionPrState,
  SANDBOX_START_FAILURE_CAUSES,
  TURN_RETENTION_LOST_REASONS,
  TURN_RETENTION_STATES,
  type AgentSessionConfigView,
  type AgentSessionDeliveryMode,
  type AgentSessionPhase,
  type AgentSessionTicket,
  type AgentSessionView,
  type AgentSessionWaitingFor,
  type InterruptedTurns,
  type QueuedMessageView,
  type SandboxStartFailureCause,
  type SteerResult,
  type TurnEndView,
  type TurnRetentionView,
} from "@/features/agent-sessions/types";

/** Phases after which the session will not change on its own. */
const TERMINAL_PHASES: ReadonlySet<AgentSessionPhase> = new Set([
  "completed",
  "failed",
  "canceled",
]);

/** Idle phases the GraphQL steer path accepts: a closed session continues in a
 *  fresh sandbox on the same branch (w10/m15). */
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

/** The one thing a session waits for today (ADR047 § w10/m497). */
export const SANDBOX_CAPACITY: AgentSessionWaitingFor = "sandbox_capacity";

/**
 * What a session waits for, when this build knows it and the session can
 * still start: a settled session waits for nothing, whatever a stale read says.
 */
export function parseAgentSessionWaitingFor(
  value: string | null | undefined,
  phase: string,
): AgentSessionWaitingFor | null {
  return value === SANDBOX_CAPACITY && !isTerminalPhase(phase)
    ? SANDBOX_CAPACITY
    : null;
}

/** A start-failure cause this build knows, or null (a newer bex-api's word). */
export function parseSandboxStartFailureCause(
  value: string | null | undefined,
): SandboxStartFailureCause | null {
  return SANDBOX_START_FAILURE_CAUSES.find((cause) => cause === value) ?? null;
}

/** Projects one wire `AgentSession` onto its normalized view. */
export function toAgentSessionView(
  wire: AgentSessionFieldsFragment,
): AgentSessionView {
  const phase = wire.phase as AgentSessionPhase;
  const waitingFor = parseAgentSessionWaitingFor(wire.waitingFor, phase);
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
    title: wire.title?.trim() || null,
    tags: wire.tags ?? [],
    secretNames: wire.secretNames ?? [],
    maxSpendUSD: wire.maxSpendUSD || null,
    automationId: wire.automationId || null,
    environmentBuildId: wire.environmentBuildId || null,
    createdBy: wire.createdBy || null,
    parentSessionId: wire.parentSessionId || null,
    rootSessionId: wire.rootSessionId || null,
    depth: wire.depth ?? 0,
    agentConfig: {
      agent: config?.agent ?? "",
      model: config?.model ?? null,
      modelEndpoint: config?.modelEndpoint ?? null,
      task: config?.task ?? "",
      template: config?.template ?? null,
      githubCli: config?.githubCli ?? null,
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
    prState: parseAgentSessionPrState(wire.prState) ?? null,
    prAutoMerge: wire.prAutoMerge
      ? {
          method: wire.prAutoMerge.method,
          enabledBy: wire.prAutoMerge.enabledBy ?? null,
          enabledAt: wire.prAutoMerge.enabledAt ?? null,
        }
      : null,
    prMergeQueued: wire.prMergeQueued ?? false,
    turns: wire.turns ?? 0,
    deliveryMode:
      (wire.deliveryMode as AgentSessionDeliveryMode | null) ?? null,
    failureReason: wire.failureReason ?? null,
    failureReasonCode: wire.failureReasonCode ?? null,
    failureCause: parseSandboxStartFailureCause(wire.failureCause),
    waitingFor,
    waitingSince: waitingFor ? (wire.waitingSince ?? null) : null,
    deliveryNote: wire.deliveryNote || null,
    createdAt: wire.createdAt,
    updatedAt: wire.updatedAt,
    canceledAt: wire.canceledAt ?? null,
    archivedAt: wire.archivedAt ?? null,
    isArchived: wire.archivedAt != null,
    isFinished: isTerminalPhase(phase),
    isTerminal: isTerminalPhase(phase),
    // The server computes steerability (w21/m257); the phase table is only the
    // fallback for a partial cache entry that predates the field.
    isSteerable: wire.steerable ?? isSteerablePhase(phase),
    state: wire.state ?? null,
    outcome: wire.outcome
      ? {
          state: wire.outcome.state,
          steerable: wire.outcome.steerable,
          turn: wire.outcome.turn ?? 0,
          summary: wire.outcome.summary ?? null,
          deliveryNote: wire.outcome.deliveryNote ?? null,
          failureReasonCode: wire.outcome.failureReasonCode ?? null,
          result: wire.outcome.result ?? null,
          cost: wire.outcome.cost
            ? {
                modelUSD: wire.outcome.cost.modelUSD,
                computeUSD: wire.outcome.cost.computeUSD ?? null,
                totalUSD: wire.outcome.cost.totalUSD ?? null,
              }
            : null,
        }
      : null,
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

/** Projects one queued message (ADR047 D12) onto its view. */
export function toQueuedMessageView(
  wire: AgentSessionQueuedMessageFieldsFragment,
): QueuedMessageView {
  return {
    id: wire.id,
    position: wire.position,
    prompt: wire.prompt,
    author: wire.author,
    attachmentCount: wire.attachmentIds?.length ?? 0,
    blockedReason: wire.blockedReason || null,
    model: wire.model || null,
    createdAt: wire.createdAt ?? null,
  };
}

/** Maps a nullable queue to views, next to run first as the server orders it. */
export function toQueuedMessageViews(
  wire:
    ReadonlyArray<AgentSessionQueuedMessageFieldsFragment> | null | undefined,
): QueuedMessageView[] {
  return (wire ?? []).map(toQueuedMessageView);
}

/** Maps a steer's payload: its ticket plus the message it queued, if any. */
export function toSteerResult(
  wire: AgentSessionFieldsFragment &
    AgentSessionMintFieldsFragment & {
      queuedMessage?: AgentSessionQueuedMessageFieldsFragment | null;
    },
): SteerResult {
  return {
    ...toAgentSessionTicket(wire),
    queuedMessage: wire.queuedMessage
      ? toQueuedMessageView(wire.queuedMessage)
      : null,
  };
}

type WireTurnRetention =
  AgentSessionTurnInterruptsQuery["agentSessionTranscript"]["turns"][number];

/**
 * Projects a stopped turn's retention (ADR047 D12, w22/m413). A state or reason
 * this build does not know reads as none and as an unnamed loss: a newer
 * bex-api never makes the transcript claim something it did not say. A kept
 * disk with no time is none, since "kept until" needs one.
 */
export function toTurnRetentionView(
  wire: Pick<
    WireTurnRetention,
    | "retention"
    | "retainedUntil"
    | "retentionLostReason"
    | "unpushedCommits"
    | "uncommittedChanges"
  >,
): TurnRetentionView {
  const state = TURN_RETENTION_STATES.find((s) => s === wire.retention);
  if (!state || state === "none" || (state === "kept" && !wire.retainedUntil)) {
    return {
      state: "none",
      until: null,
      lostReason: null,
      unpushedCommits: null,
      uncommittedChanges: null,
    };
  }
  return {
    state,
    until: state === "kept" ? wire.retainedUntil : null,
    lostReason:
      state === "lost"
        ? (TURN_RETENTION_LOST_REASONS.find(
            (reason) => reason === wire.retentionLostReason,
          ) ?? null)
        : null,
    unpushedCommits: wire.unpushedCommits ?? null,
    uncommittedChanges: wire.uncommittedChanges ?? null,
  };
}

/**
 * The turns that end with a note, by turn number: each one a person stopped,
 * and each whose undelivered work was kept or lost (w10/m491), with its
 * retention.
 */
export function toInterruptedTurns(
  wire: ReadonlyArray<WireTurnRetention> | null | undefined,
): InterruptedTurns {
  const noted = new Map<number, TurnEndView>();
  for (const turn of wire ?? []) {
    const retention = toTurnRetentionView(turn);
    if (turn.interrupted || retention.state !== "none") {
      noted.set(turn.turn, { interrupted: turn.interrupted, retention });
    }
  }
  return noted;
}

/** Bounded title for list rows — caps raw task text to avoid DOM/ARIA bloat. */
export const SESSION_TITLE_MAX = 140;

/**
 * Cap for the untruncated text a `title`/tooltip attribute carries. A prompt is
 * validated up to 100k chars and would otherwise ride a DOM attribute (and the
 * SSR payload) whole; nobody reads a tooltip past a screenful anyway.
 */
export const SESSION_TOOLTIP_MAX = 1000;

/**
 * A boundary is only honored when it sits in the last fifth of the window;
 * cutting at an earlier space would throw away most of the allowance for the
 * sake of a whole word.
 */
const MIN_WORD_BOUNDARY_RATIO = 0.8;

/**
 * Caps `raw` at `maxLen`, preferring the last word boundary inside the window so
 * a truncated prompt ends on a whole word rather than mid-syllable. A single
 * token longer than the window has no boundary to cut on and falls back to a
 * hard slice. The one truncation the agent-session surfaces share — row titles,
 * the detail heading, and the document title all read through it.
 */
function truncateForDisplay(raw: string, maxLen: number): string {
  if (raw.length <= maxLen) return raw;
  const capped = raw.slice(0, maxLen);
  const boundary = capped.lastIndexOf(" ");
  const cut =
    boundary >= Math.floor(maxLen * MIN_WORD_BOUNDARY_RATIO)
      ? capped.slice(0, boundary)
      : capped;
  return cut.trimEnd() + "…";
}

/** The subset of a session its name derives from. */
export type NamedAgentSession = Pick<AgentSessionView, "repo"> & {
  agentConfig: Pick<AgentSessionConfigView, "task">;
};

/** A session's row/mention name — its task prompt, falling back to the raw id. */
export function sessionTitle(
  view: Pick<AgentSessionView, "id"> & {
    /** The caller-supplied title (w21/m258); absent on a legacy session. */
    title?: string | null;
    agentConfig: Pick<AgentSessionConfigView, "task">;
  },
): string {
  const title = view.title?.trim();
  if (title) return title;
  // No title: the first line of the task, not the whole prompt.
  const firstLine = view.agentConfig.task.trim().split("\n", 1)[0]?.trim();
  return firstLine || view.id;
}

export function sessionTitleShort(
  view: Pick<AgentSessionView, "id"> & {
    title?: string | null;
    agentConfig: Pick<AgentSessionConfigView, "task">;
  },
  maxLen: number = SESSION_TITLE_MAX,
): string {
  return truncateForDisplay(sessionTitle(view), maxLen);
}

/** A session's derived name: bounded text to render plus its source for a tooltip. */
export interface AgentSessionDisplayName {
  /** Word-boundary-truncated text — what a heading or a tab title renders. */
  text: string;
  /** The same source at tooltip length, for a `title` attribute. */
  full: string;
}

/**
 * The one name every surface that *identifies* a session renders: the detail
 * heading and the browser tab (t002/t004). A repo-backed session is named by its
 * repository — the shape those surfaces have always shown; a repo-less
 * (chat-only) session has no repository to name it, so it falls back to its own
 * prompt, and finally to the caller's localized "Untitled session" for the
 * degenerate case where neither is present (a partial cache entry — bex-api's
 * `validateCreate` requires a task, so a real session always has one).
 *
 * Deliberately NOT the same order as `sessionTitleShort`, which names list rows:
 * a row's job is to tell sibling sessions apart, so the prompt leads there and
 * the repo never appears in the title. Both read the same truncation, and for a
 * repo-less session — the shape this exists for — the two agree on the prompt.
 *
 * Pure and UI-side on purpose: the display name is presentation, never an API
 * field (w1/m90 t005), so the fallback string is injected rather than looked up.
 */
export function agentSessionDisplayName(
  session: NamedAgentSession,
  fallback: string,
): AgentSessionDisplayName {
  const source =
    session.repo.trim() || session.agentConfig.task.trim() || fallback;
  return {
    text: truncateForDisplay(source, SESSION_TITLE_MAX),
    full: truncateForDisplay(source, SESSION_TOOLTIP_MAX),
  };
}

/**
 * The name fields of a wire session, bounded — for a caller that must carry a
 * session's name somewhere narrow (the route title loader dehydrates its result
 * into the SSR payload) rather than the whole session. Bounded past the tooltip
 * cap, so the derivation reads identically to what the full text would give.
 */
export function agentSessionNameSource(wire: {
  repo?: string | null;
  agentConfig?: { task?: string | null } | null;
}): NamedAgentSession {
  return {
    repo: wire.repo?.trim() ?? "",
    agentConfig: {
      task: (wire.agentConfig?.task ?? "")
        .trim()
        .slice(0, SESSION_TOOLTIP_MAX + 1),
    },
  };
}

/**
 * The reason text a failed session's callout should show, or null when the
 * session carries nothing worth reading and the caller should use its generic
 * copy instead.
 *
 * `failureReason` is the named reason and `status` the lifecycle line, where a
 * provisioning failure was stamped before w5/m80, but neither is guaranteed to
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
    if (
      reason === "" ||
      reason === "[object Object]" ||
      reason === view.phase
    ) {
      continue;
    }
    return reason;
  }
  return null;
}

/** bex-api's `sandbox.CodeSandboxCapacityLimit`, a session failure's code. */
export const SANDBOX_CAPACITY_LIMIT = "SANDBOX_CAPACITY_LIMIT";

/**
 * bex-api's `sandbox.CodeSandboxStartFailed`: the sandbox did not start for a
 * cause on bex's side, named by `failureCause` (w10/m497). Never a plan limit,
 * so it never offers an upgrade.
 */
export const SANDBOX_START_FAILED = "SANDBOX_START_FAILED";

/** The failure code of a turn whose repository setup step failed (ADR109 D5). */
export const AGENT_SESSION_SETUP_FAILED = "AGENT_SESSION_SETUP_FAILED";

/** True when a failed session's repository setup step failed. */
export function isRepoSetupFailure(
  view: Pick<AgentSessionView, "phase" | "failureReasonCode">,
): boolean {
  return (
    view.phase === "failed" &&
    view.failureReasonCode === AGENT_SESSION_SETUP_FAILED
  );
}

/**
 * True when a failed session's cause is a sandbox capacity / plan-limit refusal,
 * which bex-api records as the `SANDBOX_CAPACITY_LIMIT` failure code (w5/m132).
 * The failure callout keys an "Upgrade plan" action off this instead of a
 * dead-end retry.
 */
export function isSandboxCapacityFailure(
  view: Pick<AgentSessionView, "phase" | "failureReasonCode">,
): boolean {
  return (
    view.phase === "failed" && view.failureReasonCode === SANDBOX_CAPACITY_LIMIT
  );
}

/**
 * Why a failed session's sandbox did not start, beside `SANDBOX_START_FAILED`
 * (w10/m497); null for every other failure. A start failure with no cause this
 * build knows (recorded before causes were, or a newer word) is a platform
 * issue, which is what that code has always meant.
 */
export function sandboxStartFailureCause(
  view: Pick<AgentSessionView, "phase" | "failureReasonCode" | "failureCause">,
): SandboxStartFailureCause | null {
  if (view.phase !== "failed") return null;
  return view.failureReasonCode === SANDBOX_START_FAILED
    ? (view.failureCause ?? "unknown")
    : null;
}

/** The i18n keys a status phrase resolves to; the settled phases reuse the
 *  phase chip's own copy rather than restating it. */
export type AgentSessionStatusPhraseKey =
  | "agentSessions.statusPhrase.prReady"
  | "agentSessions.statusPhrase.working"
  | "agentSessions.statusPhrase.waitingForSandbox"
  | "agentSessions.phase.completed"
  | "agentSessions.phase.failed"
  | "agentSessions.phase.canceled";

/**
 * Phase + PR presence → the sidebar's human status phrase (Devin's "PR is
 * ready • 1" shape). The ones that aren't just the phase restated: a completed
 * session carrying a PR reads "PR is ready", a session waiting for a sandbox
 * machine says so (w10/m497, Devin's "waiting for machine capacity"), and every
 * other still-converging phase (creating/running/resuming/redispatching)
 * collapses to "Working…".
 */
export function agentSessionStatusPhraseKey(
  view: Pick<AgentSessionView, "phase" | "prNumber"> &
    Partial<Pick<AgentSessionView, "waitingFor">>,
): AgentSessionStatusPhraseKey {
  if (view.waitingFor && !isTerminalPhase(view.phase)) {
    return "agentSessions.statusPhrase.waitingForSandbox";
  }
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
    default:
      return "agentSessions.statusPhrase.working";
  }
}

/** Compact `h/m/s` elapsed label ("1h 4m", "12m 8s", "3s"). */
export function formatDurationShort(ms: number): string {
  const total = Math.max(0, Math.floor(ms / 1000));
  const hours = Math.floor(total / 3600);
  const mins = Math.floor((total % 3600) / 60);
  const secs = total % 60;
  if (hours > 0) return `${hours}h ${mins}m`;
  if (mins > 0) return `${mins}m ${secs}s`;
  return `${secs}s`;
}

/**
 * How long a session has waited for a sandbox machine at `nowMs`, or null
 * when it does not wait or the wait's start is unreadable (w10/m497).
 */
export function agentSessionWaitMs(
  view: Pick<AgentSessionView, "waitingFor" | "waitingSince">,
  nowMs: number,
): number | null {
  if (!view.waitingFor || !view.waitingSince) return null;
  const since = Date.parse(view.waitingSince);
  return Number.isNaN(since) ? null : Math.max(0, nowMs - since);
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
