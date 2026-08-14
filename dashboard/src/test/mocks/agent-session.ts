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
  return {
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
    isTerminal: ["completed", "failed", "canceled"].includes(phase),
    isSteerable: ["completed", "failed"].includes(phase),
    ...over,
  };
}
