// Pure composer logic for assigning a fire-and-forget agent session
// (w11/m6 t003). Kept render-free so the jest-lite runner can assert the submit
// gate and the secret-free create payload. The composer only ever carries an
// agent profile id + the prompt; model endpoints, templates, and egress are
// desktop configuration and never collected here.

export interface ComposerFields {
  repo: string;
  branch: string;
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
    fields.branch.trim() !== "" &&
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
export function buildCreateVariables(
  ownerId: string,
  fields: ComposerFields,
): CreateAgentSessionVariables {
  return {
    ownerId,
    repo: fields.repo.trim(),
    branch: fields.branch.trim(),
    agentConfig: { agent: fields.agent, task: fields.prompt.trim() },
  };
}

// The default branch for a freshly picked repo, falling back to "main" when the
// repo has no reported default (a non-GitHub or unresolved repo).
export function defaultBranchFor(
  defaultBranch: string | null | undefined,
): string {
  const trimmed = (defaultBranch ?? "").trim();
  return trimmed === "" ? "main" : trimmed;
}
