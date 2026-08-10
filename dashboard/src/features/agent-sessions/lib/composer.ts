// The prompt-box composer's pure machinery — the Configuration form contract,
// the `bex-agent/*` branch rules, and the egress-textarea parser — kept out of
// the component files so react-refresh sees components-only modules.

import { toSlug } from "@/common/lib/utils/slug";

/** Max egress hostnames the backend accepts (ADR047 D9 — kept in sync as a
 *  friendly client-side pre-check; the backend re-validates per entry). */
export const MAX_EGRESS = 32;

/** The mandated working-branch namespace (ADR047 — `bex-agent/*` confinement). */
export const BRANCH_PREFIX = "bex-agent/";

const BRANCH_SLUG_MAX = 40;

export const AGENT_OPTIONS = ["claude", "gemini", "codex"] as const;
export type AgentOption = (typeof AGENT_OPTIONS)[number];

/** The Configuration popover's fields; the task text is plain component state. */
export interface ComposerValues {
  /** Empty means "derive from the task" — see `deriveBranch`. */
  branch: string;
  agent: AgentOption;
  model: string;
  modelEndpoint: string;
  /** Raw textarea — one hostname per line (also accepts commas). */
  egress: string;
  /**
   * Ask for a draft PR when the session finishes (w5/m65). Off by default: the
   * agent always pushes its `bex-agent/*` branch, and opening a pull request
   * writes to the user's repository, so it is an explicit choice.
   */
  openPr: boolean;
}

/** Split the egress textarea into trimmed, non-empty hostnames. */
export function parseEgress(raw: string): string[] {
  return raw
    .split(/[\n,]/)
    .map((h) => h.trim())
    .filter((h) => h.length > 0);
}

/** Auto-derive `bex-agent/<slug>` from the task text, truncated sensibly. */
export function deriveBranch(task: string): string {
  const slug = toSlug(task).slice(0, BRANCH_SLUG_MAX).replace(/-+$/, "");
  return BRANCH_PREFIX + slug;
}

/** True for a branch inside the mandated namespace (prefix plus a real name). */
export function isBranchInNamespace(branch: string): boolean {
  return (
    branch.startsWith(BRANCH_PREFIX) && branch.length > BRANCH_PREFIX.length
  );
}
