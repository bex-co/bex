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
}

/** How a turn's sandbox was obtained (backend Delivery* constants). */
export type AgentSessionDeliveryMode = "resume" | "redispatch";

/** Secret-free workspace readiness returned by agentSessionCapabilities. */
export interface AgentSessionCapabilitiesView {
  enabled: boolean;
  modelKeyReady: boolean;
  /** Profile ids this workspace can run (a platform-only one appears only where served). */
  agents: string[];
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
   * True when the GraphQL steer/resume redispatch path accepts a new prompt: an
   * idle (completed/failed), non-canceled session. A live (running) session is
   * steered over the stream POST instead (t002), not this flag.
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
