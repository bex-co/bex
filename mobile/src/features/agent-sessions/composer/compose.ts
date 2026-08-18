// Pure composer logic for assigning a fire-and-forget agent session
// (w11/m6 t003). Kept render-free so the jest-lite runner can assert the submit
// gate and the secret-free create payload. The composer only ever carries an
// agent profile id + the prompt; model endpoints, templates, and egress are
// desktop configuration and never collected here.

export interface ComposerFields {
  repo: string;
  // Optional working-branch override. Empty ⇒ derive from the prompt. Whatever
  // is entered is always normalized into the mandated `bex-agent/` namespace by
  // resolveWorkingBranch — the backend rejects any branch outside it.
  branch: string;
  prompt: string;
  agent: string;
}

export interface ComposerGate {
  fields: ComposerFields;
  ready: boolean; // capabilities.ready — GitHub + model key provisioned
  submitting: boolean;
}

// The mandated working-branch namespace (ADR047 — `bex-agent/*` push
// confinement). The create mutation's `branch` is the agent's work branch, not
// a base branch; the backend derives the base from the repo's default branch
// and rejects any work branch outside this namespace.
export const BRANCH_PREFIX = "bex-agent/";
const BRANCH_SLUG_MAX = 40;

// Mirror of the dashboard slugifier (dashboard/src/common/lib/utils/slug.ts) so
// both AI-SDK clients derive byte-identical branch names from the same task.
export function toSlug(value: string): string {
  return value
    .toLowerCase()
    .replace(/[^a-z0-9-]/g, "-")
    .replace(/-+/g, "-")
    .replace(/^-+|-+$/g, "");
}

// Auto-derive `bex-agent/<slug>` from the task, truncated sensibly. Falls back
// to `bex-agent/session` when the task has no slug-able characters so the result
// is always inside the namespace.
export function deriveBranch(task: string): string {
  const slug = toSlug(task).slice(0, BRANCH_SLUG_MAX).replace(/-+$/, "");
  return BRANCH_PREFIX + (slug || "session");
}

// True for a branch inside the mandated namespace (prefix plus a real name).
export function isBranchInNamespace(branch: string): boolean {
  return (
    branch.startsWith(BRANCH_PREFIX) && branch.length > BRANCH_PREFIX.length
  );
}

// Resolve the work branch actually sent to the backend. An empty override
// derives from the prompt; an in-namespace override is kept verbatim; any other
// override (e.g. "main") is slugged into the namespace so a phone entry can
// never produce the "branch must start with bex-agent/" rejection.
export function resolveWorkingBranch(fields: ComposerFields): string {
  const override = fields.branch.trim();
  if (override === "") return deriveBranch(fields.prompt);
  if (isBranchInNamespace(override)) return override;
  const slug = toSlug(override).slice(0, BRANCH_SLUG_MAX).replace(/-+$/, "");
  return slug ? BRANCH_PREFIX + slug : deriveBranch(fields.prompt);
}

// A submit is allowed only when the workspace is provisioned (ready), no submit
// is already in flight (the idempotency guard against double taps), a repo,
// prompt, and agent are present after trimming, and a valid `bex-agent/` work
// branch can be resolved (the prompt drives the default, so a present prompt is
// sufficient).
export function canSubmit({
  fields,
  ready,
  submitting,
}: ComposerGate): boolean {
  if (!ready || submitting) return false;
  if (
    fields.repo.trim() === "" ||
    fields.prompt.trim() === "" ||
    fields.agent.trim() === ""
  ) {
    return false;
  }
  return isBranchInNamespace(resolveWorkingBranch(fields));
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
export function buildCreateVariables(
  ownerId: string,
  fields: ComposerFields,
): CreateAgentSessionVariables {
  return {
    ownerId,
    repo: fields.repo.trim(),
    branch: resolveWorkingBranch(fields),
    agentConfig: { agent: fields.agent, task: fields.prompt.trim() },
  };
}

export function repositoryDisplayName(fullName: string): string {
  const segments = fullName.split("/");
  return segments.at(-1) || fullName;
}
