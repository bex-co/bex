// Pure composer logic for assigning a fire-and-forget agent session
// (w11/m6 t003). Kept render-free so the jest-lite runner can assert the submit
// gate and the secret-free create payload. The composer carries an agent
// profile id, the prompt and, where the agent offers a choice, a model from
// the platform catalog (w10/029); model endpoints and egress are desktop
// configuration and never collected here.

export interface ComposerFields {
  repo: string;
  prompt: string;
  agent: string;
  // The chosen catalog model; "" runs the agent's default.
  model: string;
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
  agentConfig: { agent: string; task: string; model?: string };
}

// Build the create variables. agentConfig carries ONLY the selected profile
// id, the prompt and a chosen model id — the server sources the model key from
// OpenBao and defaults the endpoint/template/egress, so no secret is ever
// assembled on the phone. No model is sent when none was chosen, so the
// session runs the agent's default, as the dashboard composer does.
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
    agentConfig: {
      agent: fields.agent,
      task,
      ...(fields.model ? { model: fields.model } : {}),
    },
  };
}

// An agent's model offer from the capabilities read (w10/m516): `model` is
// what a session that names none runs, `models` what it may name — the
// platform catalog narrowed by the workspace's admins (w10/m527), a fixed
// model's one entry, or empty where the workspace's own key takes any model.
export interface ModelOffer {
  model: string | null;
  models: readonly string[];
}

// What the composer's model control draws: nothing on the workspace's own key
// (its default from Settings → Agents applies), the one model a fixed-model
// agent runs, or the model that will run with the catalog to pick from.
export type ModelControl =
  | { kind: "none" }
  | { kind: "fixed"; model: string }
  | { kind: "pick"; model: string; default: string; models: readonly string[] };

export function modelControl(
  offer: ModelOffer | undefined,
  chosen: string,
): ModelControl {
  if (!offer || offer.models.length === 0) return { kind: "none" };
  const fallback = offer.model || offer.models[0];
  const model = chosen && offer.models.includes(chosen) ? chosen : fallback;
  if (offer.models.length === 1) return { kind: "fixed", model };
  return { kind: "pick", model, default: fallback, models: offer.models };
}

// The model the create names: the chosen one, or "" while the default (or a
// fixed or provider-chosen model) runs, so a session names a model only when
// the person picked another catalog one.
export function sentModel(control: ModelControl): string {
  return control.kind === "pick" && control.model !== control.default
    ? control.model
    : "";
}

// A model id without its provider prefix ("anthropic/claude-opus-5.5" →
// "claude-opus-5.5") for the compact pill; the picker lists the whole id.
// Mirrors the dashboard's shortModel (dashboard/src/features/agent-sessions/
// lib/models.ts).
export function shortModel(id: string): string {
  return id.slice(id.lastIndexOf("/") + 1).replace(/^~/, "");
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
