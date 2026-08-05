// The @ mention picker's non-component machinery (w3/m45 t002): the typed
// token, the dependency-free fuzzy matcher, and the option model — kept out of
// the component file so react-refresh sees a components-only module.

import type { RepoView } from "@/features/services/hooks/use-repos";
import type { AgentSessionView } from "@/features/agent-sessions/types";

/** The two v1 mention categories (Devin ships more; ADR047 D9 starts here). */
export type MentionCategory = "repos" | "sessions";

/** The typed token a category selection inserts into the composer input. */
export function mentionToken(category: MentionCategory): string {
  return `@${category}:`;
}

/**
 * Dependency-free fuzzy match: case-insensitive `includes` first, then an
 * in-order subsequence scan (`tianpan` matches `android-tianpanco-release`;
 * `awg` matches `acme/widgets`). An empty query matches everything.
 */
export function fuzzyMatch(query: string, candidate: string): boolean {
  const q = query.trim().toLowerCase();
  if (!q) return true;
  const c = candidate.toLowerCase();
  if (c.includes(q)) return true;
  let i = 0;
  for (const ch of c) {
    if (ch === q[i]) i += 1;
    if (i === q.length) return true;
  }
  return false;
}

/** One row of the open picker — a category, a repo, or a prior session. */
export type MentionOption =
  | { kind: "category"; category: MentionCategory }
  | { kind: "repo"; repo: RepoView }
  | { kind: "session"; session: AgentSessionView };

/** Stable per-option DOM id, referenced by the textarea's aria-activedescendant. */
export function mentionOptionId(idBase: string, index: number): string {
  return `${idBase}-option-${index}`;
}
