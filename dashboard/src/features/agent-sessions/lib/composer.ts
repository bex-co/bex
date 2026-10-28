// The prompt-box composer's pure machinery — the form contract and the
// `bex-agent/*` branch derivation — kept out of the component files so
// react-refresh sees components-only modules.

import { toSlug } from "@/common/lib/utils/slug";

/** The mandated working-branch namespace (ADR047 — `bex-agent/*` confinement). */
const BRANCH_PREFIX = "bex-agent/";

const BRANCH_SLUG_MAX = 40;

export const AGENT_OPTIONS = ["claude", "gemini", "codex"] as const;
export type AgentOption = (typeof AGENT_OPTIONS)[number];

/** The composer's form fields; the task text is plain component state. */
export interface ComposerValues {
  agent: AgentOption;
}

/**
 * Derive the `bex-agent/<slug>` working branch from the task text, truncated
 * sensibly. The composer has no branch field, so this must always produce a
 * valid branch: a task with no ASCII letters or digits (e.g. one written in
 * Chinese) falls back to a time-based `session-<base36>` name.
 */
export function deriveBranch(task: string, now: number = Date.now()): string {
  const slug = toSlug(task).slice(0, BRANCH_SLUG_MAX).replace(/-+$/, "");
  return BRANCH_PREFIX + (slug || `session-${now.toString(36)}`);
}
