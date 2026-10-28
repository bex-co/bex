// Pure composer logic for assigning a fire-and-forget agent session
// (w11/m6 t003). Kept render-free so the jest-lite runner can assert the submit
// gate and the secret-free create payload. The composer only ever carries an
// agent profile id + the prompt; model endpoints, templates, and egress are
// desktop configuration and never collected here.

export interface ComposerFields {
  repo: string;
  prompt: string;
  agent: string;
}

export interface ComposerGate {
  fields: ComposerFields;
  ready: boolean; // capabilities.ready — GitHub + model key provisioned
  submitting: boolean;
}

// A submit is allowed only when the workspace is provisioned (ready), no submit
// is already in flight (the idempotency guard against double taps), and every
// field is non-empty after trimming.
export function canSubmit({
  fields,
  ready,
  submitting,
}: ComposerGate): boolean {
  if (!ready || submitting) return false;
  return (
    fields.repo.trim() !== "" &&
    fields.prompt.trim() !== "" &&
    fields.agent.trim() !== ""
  );
}

export interface CreateAgentSessionVariables {
  ownerId: string;
  repo: string;
  branch: string;
  agentConfig: { agent: string; task: string };
}

// Build the create variables. agentConfig carries ONLY the selected profile id
// and the prompt — the server sources the model key from OpenBao and defaults
// the endpoint/template/egress, so no secret is ever assembled on the phone.
// The working branch is always derived (the agent pushes only to bex-agent/*).
export function buildCreateVariables(
  ownerId: string,
  fields: ComposerFields,
  now: number = Date.now(),
): CreateAgentSessionVariables {
  const task = fields.prompt.trim();
  return {
    ownerId,
    repo: fields.repo.trim(),
    branch: deriveBranch(task, now),
    agentConfig: { agent: fields.agent, task },
  };
}

// The agent may only push to the bex-agent/* namespace (ADR047/ADR090), so
// the branch is derived from the task exactly as the dashboard composer does
// (dashboard/src/features/agent-sessions/lib/composer.ts deriveBranch): a
// lowercase ASCII slug capped at 40 chars, or session-<base36 time> when the
// task has no ASCII letters or digits (e.g. a Chinese task).
const BRANCH_PREFIX = "bex-agent/";
const BRANCH_SLUG_MAX = 40;

export function deriveBranch(task: string, now: number = Date.now()): string {
  const slug = task
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "")
    .slice(0, BRANCH_SLUG_MAX)
    .replace(/-+$/, "");
  return BRANCH_PREFIX + (slug || `session-${now.toString(36)}`);
}
