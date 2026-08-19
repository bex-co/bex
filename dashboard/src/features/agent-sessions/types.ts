// Dashboard-side projection of a cloud coding-agent session (ADR047 D9), mapped
// from bex-api's Render-shaped GraphQL `AgentSession`
// (backend/internal/agentsessions/graphql.go). The UI keys off `phase` (the
// stable lifecycle enum), never the free-text `status`; `lib/mapper.ts` derives
// the isTerminal/isSteerable/duration helpers so pages never re-encode phase.

/**
 * The session lifecycle enum (backend `models.go` Phase* constants). Terminal
 * phases are completed/failed/canceled; every other phase is still converging.
 * The const list is the single source (the list page's phase filter renders
 * it); the union derives from it.
 */
export const AGENT_SESSION_PHASES = [
  "creating",
  "running",
  "resuming",
  "redispatching",
  "hibernating",
  "hibernated",
  "completed",
  "failed",
  "canceling",
  "canceled",
] as const;

export type AgentSessionPhase = (typeof AGENT_SESSION_PHASES)[number];

/** Archive-membership values accepted by the sessions list URL. */
export type AgentSessionArchivedFilter = "true" | "all";

/** Shareable list context carried into a detail page's Back affordance. */
export interface AgentSessionListSearch {
  archived?: AgentSessionArchivedFilter;
  phase?: AgentSessionPhase;
}

/** How a turn's sandbox was obtained (backend Delivery* constants). */
export type AgentSessionDeliveryMode = "resume" | "redispatch";

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
  createdAt: string;
  updatedAt: string;
  canceledAt: string | null;
  /**
   * Hibernation (ADR059 D5/D6). `pinned` marks the never-expire tier;
   * `snapshotBytes` is the durable storage cost shown to the tenant (0 while
   * live); `hibernatedAt`/`retainUntil` show when it hibernated and when an
   * unpinned snapshot will be deleted (null while live or pinned).
   */
  pinned: boolean;
  snapshotBytes: number;
  hibernatedAt: string | null;
  retainUntil: string | null;
  /**
   * Archive (ADR065 D1): set ⇒ out of the working set, mutation verbs refused
   * (`AGENT_SESSION_ARCHIVED`) until unarchived; reads (transcript included)
   * always work. Orthogonal to phase.
   */
  archivedAt: string | null;
  /** hibernated — pod-less but resumable from a durable snapshot (ADR059 D2). */
  isHibernated: boolean;
  /** archivedAt non-null — the session is out of the working set (ADR065). */
  isArchived: boolean;
  /**
   * Terminal or hibernated — past all live work. Gates the delete verb and the
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
