// Dashboard-side projection of a cloud coding-agent session (ADR047 D9), mapped
// from bex-api's Render-shaped GraphQL `AgentSession`
// (backend/internal/agentsessions/graphql.go). The UI keys off `phase` (the
// stable lifecycle enum), never the free-text `status`; `lib/mapper.ts` derives
// the isTerminal/isSteerable/duration helpers so pages never re-encode phase.

/**
 * The session lifecycle enum (backend `models.go` Phase* constants). Terminal
 * phases are completed/failed/canceled; every other phase is still converging.
 */
export type AgentSessionPhase =
  | "creating"
  | "running"
  | "resuming"
  | "redispatching"
  | "completed"
  | "failed"
  | "canceling"
  | "canceled";

/** How a turn's sandbox was obtained (backend Delivery* constants). */
export type AgentSessionDeliveryMode = "resume" | "redispatch";

/** The stable cross-surface driver config (agent/model/task/template). */
export interface AgentSessionConfigView {
  agent: string;
  model: string | null;
  modelEndpoint: string | null;
  task: string;
  template: string | null;
}

/** The bounded, verifiable turn extract the driver captures (ADR047 D4). */
export interface AgentSessionEvidenceView {
  commandLog: string[];
  testOutput: string[];
  outputTail: string | null;
  changedFiles: string[];
  commits: number | null;
  /** True when any cap dropped content — truncation is explicit, never silent. */
  truncated: boolean;
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
  evidence: AgentSessionEvidenceView | null;
  /** Number of prompt turns taken so far. */
  turns: number;
  deliveryMode: AgentSessionDeliveryMode | null;
  /** Populated on a failed session — the named reason (ADR047 D4). */
  failureReason: string | null;
  createdAt: string;
  updatedAt: string;
  canceledAt: string | null;
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
 * the browser presents `ticket` to the m43 stream endpoint at `url` until
 * `expiresAt`. Distinct from the polled view so ambient attach authority never
 * leaks onto list/detail reads.
 */
export interface AgentSessionTicket {
  /** The session (post-mint), so callers can route to it and re-read metadata. */
  session: AgentSessionView;
  ticket: string | null;
  url: string | null;
  expiresAt: string | null;
}
