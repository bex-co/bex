// Dashboard-side projection of a cloud coding-agent session (ADR047 D9), mapped
// from bex-api's Render-shaped GraphQL `AgentSession`
// (backend/internal/agentsessions/graphql.go). The UI keys off `phase` (the
// stable lifecycle enum), never the free-text `status`; `lib/mapper.ts` derives
// the isTerminal/isSteerable/duration helpers so pages never re-encode phase.

/** The latest turn's machine-readable outcome (w21/m257). */
export interface AgentSessionOutcome {
  state: string;
  steerable: boolean;
  turn: number;
  summary: string | null;
  deliveryNote: string | null;
  failureReasonCode: string | null;
  /**
   * The turn's final schema-validated structured result as JSON text (ADR094
   * D5, w21/003); null for a free-form turn or before the agent declared one.
   */
  result: string | null;
  /**
   * What the session has cost so far (ADR094 D7, w9/m178): model spend plus
   * sandbox compute, with totalUSD the one figure to compare with the bill.
   * computeUSD/totalUSD are null from a server that predates them; the whole
   * cost is null when nothing is priced yet or a part could not be read.
   */
  cost: {
    modelUSD: string;
    computeUSD: string | null;
    totalUSD: string | null;
  } | null;
}

/**
 * The session lifecycle enum (backend `models.go` Phase* constants). Terminal
 * phases are completed/failed/canceled; every other phase is still converging.
 * The const list is the single source (route search parses against it); the
 * union derives from it.
 */
export const AGENT_SESSION_PHASES = [
  "creating",
  "running",
  "resuming",
  "redispatching",
  "completed",
  "failed",
  "canceling",
  "canceled",
] as const;

export type AgentSessionPhase = (typeof AGENT_SESSION_PHASES)[number];

/** Archive-membership values accepted by the sessions list URL (UI key). */
export type AgentSessionArchivedFilter = "archived" | "all";

/** Parse current and legacy archive-membership values from route search. */
export function parseAgentSessionArchivedFilter(
  value: unknown,
): AgentSessionArchivedFilter | undefined {
  if (value === "archived" || value === "true" || value === true) {
    return "archived";
  }
  return value === "all" ? "all" : undefined;
}

/** A route search's phase filter, when it names a known phase. */
export function parseAgentSessionPhase(
  value: unknown,
): AgentSessionPhase | undefined {
  return AGENT_SESSION_PHASES.find((phase) => phase === value);
}

/**
 * A session PR's state as bex last read it from GitHub (w9/m386, Devin's
 * `pr_state`). A draft PR counts as open. The const list is the single source
 * for the route search, the list filter and the wire mapping.
 */
export const AGENT_SESSION_PR_STATES = ["open", "merged", "closed"] as const;

export type AgentSessionPrState = (typeof AGENT_SESSION_PR_STATES)[number];

/** A known PR state, or undefined for anything else (search value or wire). */
export function parseAgentSessionPrState(
  value: unknown,
): AgentSessionPrState | undefined {
  return AGENT_SESSION_PR_STATES.find((state) => state === value);
}

/** Translate the UI's `archived` name to the backend's `true` wire value. */
export function agentSessionArchivedQueryValue(
  value: AgentSessionArchivedFilter | "false" | undefined,
): "false" | "true" | "all" | null {
  return value === "archived" ? "true" : (value ?? null);
}

/** Shareable list context carried into a detail page's Back affordance. */
export interface AgentSessionListSearch {
  archived?: AgentSessionArchivedFilter;
  phase?: AgentSessionPhase;
  /** Show only sessions carrying this tag (w21/002). */
  tag?: string;
  /** Show only sessions whose PR is in this state (w9/m386). */
  prState?: AgentSessionPrState;
}

/** What a waiting session waits for (ADR047 § w10/m497): today only a machine. */
export type AgentSessionWaitingFor = "sandbox_capacity";

/**
 * Why a sandbox did not start, beside `SANDBOX_START_FAILED` (ADR047 §
 * w10/m497): the pool had no room, the image did not pull, the runtime did not
 * start the machine, the workspace's quota refused it, or bex-api could not
 * tell. The const list is the single source for the wire mapping.
 */
export const SANDBOX_START_FAILURE_CAUSES = [
  "pool_full",
  "image_pull",
  "runtime",
  "quota",
  "unknown",
] as const;

export type SandboxStartFailureCause =
  (typeof SANDBOX_START_FAILURE_CAUSES)[number];

/** How a turn's sandbox was obtained (backend Delivery* constants). */
export type AgentSessionDeliveryMode = "resume" | "redispatch";

/** Secret-free workspace readiness returned by agentSessionCapabilities. */
export interface AgentSessionCapabilitiesView {
  enabled: boolean;
  /** Profile ids this workspace can run (a platform-only one appears only where served). */
  agents: string[];
  /**
   * The listed profiles whose model credential is in place: the claude
   * profile on the platform route, any profile on the workspace's own key.
   * Codex and Gemini need the workspace's key even where the route serves
   * claude, so readiness is per profile (AGENT_SESSION_MODEL_KEY_REQUIRED).
   */
  modelKeyReadyAgents: string[];
  github: {
    connected: boolean;
  };
  attachments: AttachmentLimitsView;
}

/** The API's attachment limits (ADR095 D8); `enabled` is false until storage exists. */
export interface AttachmentLimitsView {
  enabled: boolean;
  maxBytes: number;
  maxPerTurn: number;
}

/** The stable cross-surface per-session config (agent/model/task/template). */
export interface AgentSessionConfigView {
  agent: string;
  model: string | null;
  modelEndpoint: string | null;
  task: string;
  template: string | null;
}

/**
 * The normalized session the list/detail pages render. Raw wire fields plus the
 * phase-derived helpers (`isTerminal`/`isSteerable`) computed once in the mapper.
 * Mint fields (ticket/url/expiresAt) are NOT here — they ride only the mint
 * mutations (see `AgentSessionTicket`), never the polled list/detail reads.
 */
export interface AgentSessionView {
  id: string;
  ownerId: string;
  /** GitHub owner/repository the session works against. */
  repo: string;
  /** The `bex-agent/*` working branch. */
  branch: string;
  agentConfig: AgentSessionConfigView;
  sandboxId: string | null;
  /**
   * `ags-<id>@<ssh-host>` for opening the session's sandbox over SSH — the
   * "Open in Zed" affordance (ADR054 D5). Non-null only while the sandbox is live
   * and BEX_SSH_HOST is configured; the backend omits it otherwise, so its mere
   * presence gates the button.
   */
  sshAddress: string | null;
  /** The title the caller gave the session (REST/GraphQL/MCP create); null if none. */
  title: string | null;
  /** The caller's tags, for the list's tag filter. */
  tags: string[];
  /** Names of the secrets attached to the session (ADR098); never values. */
  secretNames: string[];
  /**
   * The per-session model spend ceiling in USD as a decimal string (ADR094 D8,
   * w21/m260); null when the caller set none.
   */
  maxSpendUSD: string | null;
  /** The agent automation (aut-…) that started the session, ADR101. */
  automationId: string | null;
  /**
   * The repository environment build (arb-…) the latest turn's sandbox booted
   * from (ADR111 D4); null when the turn ran the setup steps itself.
   */
  environmentBuildId: string | null;
  /**
   * Who started the session (w7/m296): a member's identity subject or an API
   * key's client id. An automation's sessions carry its creator. Null on rows
   * from before it was recorded.
   */
  createdBy: string | null;
  /** The session (ags-…) that spawned this one (ADR102); null on a top-level session. */
  parentSessionId: string | null;
  /** The top of this session's tree (ADR102); null on a top-level session. */
  rootSessionId: string | null;
  /** Distance from the root of the tree; 0 on a top-level session. */
  depth: number;
  /** The lifecycle enum the whole UI keys off. */
  phase: AgentSessionPhase;
  /** Free-text human status line; display-only, never branched on. */
  status: string;
  headSha: string | null;
  prUrl: string | null;
  prNumber: number | null;
  /**
   * The PR's state as bex last read it from GitHub (w9/m386); null until bex
   * has read it, for a session with no PR, and for a value this dashboard
   * does not know.
   */
  prState: AgentSessionPrState | null;
  /** Number of prompt turns taken so far. */
  turns: number;
  deliveryMode: AgentSessionDeliveryMode | null;
  /** Populated on a failed session — the named reason (ADR047 D4). */
  failureReason: string | null;
  /**
   * A stable code beside `failureReason` when the UI acts on the cause, such as
   * `SANDBOX_CAPACITY_LIMIT` (w5/m132). Decide by this, never by the wording.
   */
  failureReasonCode: string | null;
  /**
   * Why a failed start failed, beside `SANDBOX_START_FAILED` only (w10/m497);
   * null otherwise, and for a cause this dashboard does not know.
   */
  failureCause: SandboxStartFailureCause | null;
  /**
   * What the turn waits for before it can start (w10/m497): a sandbox machine,
   * while the pool has no room. The phase stays creating or redispatching and
   * the state working. Null while nothing holds the turn up.
   */
  waitingFor: AgentSessionWaitingFor | null;
  /** RFC 3339: when the wait began; null when the session does not wait. */
  waitingSince: string | null;
  /**
   * Set on a completed session whose work the agent left undelivered — an
   * uncommitted tree, unpushed commits, or a pushed branch with no PR (ADR090
   * D5). The work stays in the sandbox only until it closes after 2h idle.
   */
  deliveryNote: string | null;
  createdAt: string;
  updatedAt: string;
  canceledAt: string | null;
  /**
   * Archive (ADR065 D1): set ⇒ out of the working set, mutation verbs refused
   * (`AGENT_SESSION_ARCHIVED`) until unarchived; reads (transcript included)
   * always work. Orthogonal to phase. Transcript part timing (`at` on UI-message
   * stream chunks) is not a field on this view — see
   * `lib/acp-parts.ts` `sourceTimestampsMs`.
   */
  archivedAt: string | null;
  /** archivedAt non-null — the session is out of the working set (ADR065). */
  isArchived: boolean;
  /**
   * Terminal — past all live work. Gates the delete verb and the
   * replay-only conversation attach (ADR065 D2/D4); the backend twin is
   * `finishedPhase`.
   */
  isFinished: boolean;
  /** completed/failed/canceled — the session will not change further on its own. */
  isTerminal: boolean;
  /**
   * True when a steer starts a turn now: an idle (completed/failed),
   * non-archived session. It is not the test for whether a message is accepted:
   * a session whose turn runs queues the message instead (ADR047 D12), so the
   * composer is enabled for any session that is not canceled or archived.
   */
  isSteerable: boolean;
  /**
   * The server-computed session state (`state`, w21/m257): what a caller should
   * do next. Null on a partial cache entry that predates the field.
   */
  state: string | null;
  /** The server-computed outcome of the latest turn; null before one exists. */
  outcome: AgentSessionOutcome | null;
}

/**
 * The short-lived attach ticket a mint op (create/steer/resume/attach) returns:
 * the browser presents `ticket` to the m43 stream endpoint until `expiresAt`.
 * Distinct from the polled view so ambient attach authority never leaks onto
 * list/detail reads.
 *
 * `url` is the phase-2 raw-ACP WebSocket gateway origin — NOT the phase-1 SSE
 * stream endpoint. `streamUrl` is that (w10/m9 t003, w3/013): the
 * server-authoritative `<id>/stream` URL, null when the backend has no
 * BEX_API_PUBLIC_URL configured (the transport falls back to deriving it
 * locally from `config.apiBaseUrl` in that case — see
 * `lib/transport.ts#agentSessionStreamUrl`).
 */
export interface AgentSessionTicket {
  /** The session (post-mint), so callers can route to it and re-read metadata. */
  session: AgentSessionView;
  ticket: string | null;
  url: string | null;
  streamUrl: string | null;
  expiresAt: string | null;
}

/**
 * A message sent to a session while its turn ran, waiting to run as a turn of
 * its own (ADR047 D12). The dispatcher takes the first one when the running
 * turn settles.
 */
export interface QueuedMessageView {
  id: string;
  /** Its 1-based place in the queue; 1 runs next. */
  position: number;
  prompt: string;
  /** The member the turn runs as: who queued it, or who last edited it. */
  author: string;
  /** How many files ride with it; only the sender may edit such a message. */
  attachmentCount: number;
  /**
   * Why the last attempt to run it failed (a spend ceiling, the live-sandbox
   * limit); the dispatcher retries within seconds. Null while nothing blocks it.
   */
  blockedReason: string | null;
  createdAt: string | null;
}

/**
 * What became of the unpushed work of a turn a person stopped (ADR047 D12,
 * w22/m413): its disk, which the next message continues from while it is kept.
 * The const lists are the single source for the wire mapping.
 */
export const TURN_RETENTION_STATES = [
  "none",
  "saving",
  "kept",
  "continued",
  "lost",
] as const;

export type TurnRetentionState = (typeof TURN_RETENTION_STATES)[number];

/** Why a lost disk was lost, when bex-api says so. */
export const TURN_RETENTION_LOST_REASONS = [
  "expired",
  "storage_full",
  "save_failed",
  "editor_open",
  "canceled",
  "archived",
  "not_restored",
  "sandbox_lost",
] as const;

export type TurnRetentionLostReason =
  (typeof TURN_RETENTION_LOST_REASONS)[number];

export interface TurnRetentionView {
  state: TurnRetentionState;
  /** RFC 3339: when a kept disk expires; null for every other state. */
  until: string | null;
  /** Why a lost disk was lost; null for every other state, or an unnamed cause. */
  lostReason: TurnRetentionLostReason | null;
  /**
   * What the turn left undelivered, for a disk kept or lost (w10/m491): how
   * many commits were not on the remote branch and whether there were
   * uncommitted changes; null when bex-api does not know.
   */
  unpushedCommits: number | null;
  uncommittedChanges: boolean | null;
}

/** A turn that ends with a note: a person stopped it, or its disk was kept or lost. */
export interface TurnEndView {
  interrupted: boolean;
  retention: TurnRetentionView;
}

/**
 * The turns of a session that end with a note, by turn number: those a person
 * stopped, and those whose undelivered work was kept or lost (ADR047 D12).
 */
export type InterruptedTurns = ReadonlyMap<number, TurnEndView>;

/**
 * What a steer answers: the session and its ticket as for any mint, plus the
 * message it queued when the session was busy (null when it started a turn).
 */
export interface SteerResult extends AgentSessionTicket {
  queuedMessage: QueuedMessageView | null;
}
