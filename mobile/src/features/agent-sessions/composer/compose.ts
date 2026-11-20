// Pure composer logic for assigning a fire-and-forget agent session
// (w11/m6 t003). Kept render-free so the jest-lite runner can assert the submit
// gate and the secret-free create payload. The composer carries an agent
// profile id, the prompt and, where offered, a model from the platform catalog
// (w10/029) and a sandbox template (w10/031); model endpoints and egress are
// desktop configuration and never collected here.

export interface ComposerFields {
  repo: string;
  prompt: string;
  agent: string;
  // The chosen catalog model; "" runs the agent's default.
  model: string;
  // The chosen sandbox template; "" runs the workspace default.
  template: string;
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
  agentConfig: {
    agent: string;
    task: string;
    model?: string;
    template?: string;
  };
}

// Build the create variables. agentConfig carries ONLY the selected profile
// id, the prompt, a chosen model id and a chosen sandbox template — the server
// sources the model key from OpenBao and defaults the endpoint and egress, so
// no secret is ever assembled on the phone. No model or template is sent when
// none was chosen, so the session runs the agent's default model on the
// workspace's default template, as the dashboard composer does.
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
      ...(fields.template ? { template: fields.template } : {}),
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

// What a composer control draws: nothing when nothing is offered, the one
// value there is, or the value that will run with the offer to pick from.
// `default` is what runs when the create names none.
export type Choice =
  | { kind: "none" }
  | { kind: "fixed"; value: string }
  | {
      kind: "pick";
      value: string;
      default: string;
      options: readonly string[];
    };

// The chosen value while it is still offered (an admin may turn a model off,
// w10/m527, while the composer is open), else the default.
function choose(
  options: readonly string[],
  preferred: string | null,
  chosen: string,
): Choice {
  if (options.length === 0) return { kind: "none" };
  const fallback = preferred || options[0];
  const value = chosen && options.includes(chosen) ? chosen : fallback;
  if (options.length === 1) return { kind: "fixed", value };
  return { kind: "pick", value, default: fallback, options };
}

// The model control: none on the workspace's own key (its default from
// Settings → Agents applies), a fixed-model agent's one model, or the catalog.
export function modelControl(
  offer: ModelOffer | undefined,
  chosen: string,
): Choice {
  return offer ? choose(offer.models, offer.model, chosen) : { kind: "none" };
}

// What the create names: the picked value, or "" while the default (or a
// fixed or provider-chosen value) runs, so a session names a model or a
// template only when the person picked another one.
export function sentChoice(control: Choice): string {
  return control.kind === "pick" && control.value !== control.default
    ? control.value
    : "";
}

// A model id without its provider prefix ("anthropic/claude-opus-5.5" →
// "claude-opus-5.5") for the compact pill; the picker lists the whole id.
// Mirrors the dashboard's shortModel (dashboard/src/features/agent-sessions/
// lib/models.ts).
export function shortModel(id: string): string {
  return id.slice(id.lastIndexOf("/") + 1).replace(/^~/, "");
}

// A sandbox template from the capabilities read (w10/m519): its name and size.
export interface SandboxTemplate {
  name: string;
  cpu: number;
  memoryGb: number;
}

// The workspace's sandbox offer: `defaultTemplate` is what a session that
// names none runs on, `templates` every one it may name, smallest first.
export interface TemplateOffer {
  defaultTemplate: string;
  templates: readonly SandboxTemplate[];
}

export function templateControl(
  offer: TemplateOffer | undefined,
  chosen: string,
): Choice {
  return offer
    ? choose(
        offer.templates.map((t) => t.name),
        offer.defaultTemplate,
        chosen,
      )
    : { kind: "none" };
}

// One number format per language: CPU cores and memory as the read carries
// them, at most two decimals, in the app's language.
const sizeFormats = new Map<string, Intl.NumberFormat>();
function sizeFormat(language: string): Intl.NumberFormat {
  let format = sizeFormats.get(language);
  if (!format) {
    format = new Intl.NumberFormat(language, { maximumFractionDigits: 2 });
    sizeFormats.set(language, format);
  }
  return format;
}

// The label of the template named `name`: `label` given its name and its
// formatted size when the list carries it, else the name alone (a template
// the read no longer lists). Mirrors the dashboard's namedTemplateLabel
// (dashboard/src/features/agent-sessions/lib/templates.ts).
export function templateLabel(
  name: string,
  templates: readonly SandboxTemplate[],
  language: string,
  label: (parts: { name: string; cpu: string; memory: string }) => string,
): string {
  const template = templates.find((t) => t.name === name);
  if (!template) return name;
  const number = sizeFormat(language);
  return label({
    name,
    cpu: number.format(template.cpu),
    memory: number.format(template.memoryGb),
  });
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
