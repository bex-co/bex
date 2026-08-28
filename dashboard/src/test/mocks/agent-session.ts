import type { AgentSessionView } from "@/features/agent-sessions/types";

/**
 * Builds an `AgentSessionView` for tests. Seven near-identical copies of this
 * literal used to live in the agent-session test files; when w5/m65 changed the
 * shape (dropping `evidence`) one copy was missed
 * and rotted silently — `tsconfig.app.json` excludes tests from `tsc -b` and
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
    isFinished:
      ["completed", "failed", "canceled"].includes(phase) ||
      phase === "hibernated",
    id: "as-1",
    ownerId: "tea-1",
    repo: "acme/widgets",
    branch: "bex-agent/fix",
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
    turns: 0,
    deliveryMode: null,
    failureReason: null,
    createdAt: new Date().toISOString(),
    updatedAt: new Date().toISOString(),
    canceledAt: null,
    pinned: false,
    snapshotBytes: 0,
    hibernatedAt: null,
    retainUntil: null,
    isHibernated: phase === "hibernated",
    isTerminal: ["completed", "failed", "canceled"].includes(phase),
    isSteerable: ["completed", "failed", "hibernated"].includes(phase),
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
