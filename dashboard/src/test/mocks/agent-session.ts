import type { AgentSessionTurnInterruptsQuery } from "@/graphql/definitions";
import type { AgentSessionView } from "@/features/agent-sessions/types";

/**
 * Builds an `AgentSessionView` for tests. Seven near-identical copies of this
 * literal used to live in the agent-session test files; when w5/m65 changed the
 * shape one copy was missed and rotted silently — `tsconfig.app.json` excludes tests from `tsc -b` and
 * vitest does not typecheck, so nothing catches it.
 *
 * `isTerminal`/`isSteerable` default to what the mapper derives from `phase`, so
 * a fixture cannot claim a phase/flag combination the real projection never
 * produces. `task` is a convenience override for the nested `agentConfig.task`.
 */
export function agentSessionView({
  task = "refactor the mapper",
  agentConfig,
  ...over
}: Partial<AgentSessionView> & { task?: string } = {}): AgentSessionView {
  const phase = over.phase ?? "running";
  const archivedAt = over.archivedAt ?? null;
  return {
    archivedAt,
    isArchived: archivedAt != null,
    isFinished: ["completed", "failed", "canceled"].includes(phase),
    id: "as-1",
    ownerId: "tea-1",
    repo: "acme/widgets",
    branch: "bex-agent/fix",
    title: null,
    tags: [],
    secretNames: [],
    maxSpendUSD: null,
    automationId: null,
    environmentBuildId: null,
    createdBy: null,
    parentSessionId: null,
    rootSessionId: null,
    depth: 0,
    state: null,
    outcome: null,
    agentConfig: agentConfig ?? {
      agent: "claude",
      model: null,
      modelEndpoint: null,
      task,
      template: null,
    },
    sandboxId: null,
    sshAddress: null,
    phase,
    status: phase,
    headSha: null,
    prUrl: null,
    prNumber: null,
    prState: null,
    prAutoMerge: null,
    prMergeQueued: false,
    turns: 0,
    deliveryMode: null,
    failureReason: null,
    failureReasonCode: null,
    failureCause: null,
    waitingFor: null,
    waitingSince: null,
    deliveryNote: null,
    createdAt: new Date().toISOString(),
    updatedAt: new Date().toISOString(),
    canceledAt: null,
    isTerminal: ["completed", "failed", "canceled"].includes(phase),
    isSteerable: ["completed", "failed"].includes(phase),
    ...over,
  };
}

/**
 * The repo-less (chat-only) shape — bex-api's `validateCreate` clears `repo` and
 * `branch` when a session is created without a repository, and the session runs
 * its prompt in an empty sandbox (no clone, no branch, no PR).
 *
 * It exists because every fixture above it had a repo, which is exactly why the
 * empty `<h1>` / `Working… · ·` bug (w1/m90) was invisible to the suite: the
 * only shape the tests ever rendered was the one that happened to work.
 */
export function repoLessAgentSessionView(
  over: Omit<Partial<AgentSessionView>, "repo" | "branch"> & {
    task?: string;
  } = {},
): AgentSessionView {
  return agentSessionView({
    id: "as-chat",
    task: "explain the mapper",
    ...over,
    repo: "",
    branch: "",
  });
}

/** When a kept stopped turn's disk expires, in the tests that read one. */
export const RETAINED_UNTIL = "2026-11-16T10:00:00Z";

type WireTurn =
  AgentSessionTurnInterruptsQuery["agentSessionTranscript"]["turns"][number];

/**
 * A turn of the transcript as bex-api's GraphQL answers the interrupted-turns
 * read (w22/m413 t004): a stopped turn that kept nothing unless overridden.
 * `retainedUntil` defaults to `RETAINED_UNTIL` for a kept disk and null for any
 * other, as the server sends it.
 */
export function wireTurn(over: Partial<WireTurn> = {}): WireTurn {
  const retention = over.retention ?? "none";
  return {
    __typename: "AgentSessionTranscriptTurn",
    turn: 1,
    interrupted: true,
    retention,
    retainedUntil: retention === "kept" ? RETAINED_UNTIL : null,
    retentionLostReason: "",
    unpushedCommits: null,
    uncommittedChanges: null,
    ...over,
  };
}
