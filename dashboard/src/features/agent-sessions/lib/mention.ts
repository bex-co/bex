// The @ mention picker's non-component machinery: the typed token, the
// dependency-free fuzzy matcher, and the option model — kept out of the
// component file so react-refresh sees a components-only module.

import type { useTranslations } from "@/common/hooks/use-translations";
import type { RepoView } from "@/features/services/hooks/use-repos";
import { sessionTitle } from "@/features/agent-sessions/lib/mapper";
import type { AgentSessionView } from "@/features/agent-sessions/types";

type Translate = ReturnType<typeof useTranslations>["t"];
type TranslationKey = Parameters<Translate>[0];

/** The two v1 mention categories (Devin ships more; ADR047 D9 starts here). */
export type MentionCategory = "repos" | "sessions";

export const MENTION_CATEGORIES = ["repos", "sessions"] as const;

/** The one home for each category's copy — label, description, empty state. */
export const CATEGORY_META: Record<
  MentionCategory,
  {
    labelKey: TranslationKey;
    descKey: TranslationKey;
    emptyKey: TranslationKey;
  }
> = {
  repos: {
    labelKey: "agentSessions.mentionCategoryRepos",
    descKey: "agentSessions.mentionCategoryReposDesc",
    emptyKey: "agentSessions.mentionReposEmpty",
  },
  sessions: {
    labelKey: "agentSessions.mentionCategorySessions",
    descKey: "agentSessions.mentionCategorySessionsDesc",
    emptyKey: "agentSessions.mentionSessionsEmpty",
  },
};

/** The typed token a category selection inserts into the composer input. */
export function mentionToken(category: MentionCategory): string {
  return `@${category}:`;
}

/**
 * The open mention popup. `category: null` is the first level (choosing a
 * category); a set category is the second (choosing one of its items).
 */
export interface MentionState {
  category: MentionCategory | null;
  /** Index of the trigger `@` in the task text. */
  start: number;
  /** The filter text typed after the token — the caret sits at its end. */
  query: string;
  /** Index of the keyboard-highlighted option. */
  highlight: number;
}

/** Index just past the mention's trigger/token in the task text. */
export function mentionTokenEnd(state: MentionState): number {
  return (
    state.start +
    (state.category === null ? 1 : mentionToken(state.category).length)
  );
}

/** Index just past the whole mention (token + typed query) in the task text. */
export function mentionEnd(state: MentionState): number {
  return mentionTokenEnd(state) + state.query.length;
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

/** The rows a mention level offers, before filtering. */
export interface MentionSource {
  repos: RepoView[];
  sessions: AgentSessionView[];
}

/** Every text a row can be matched against (a category matches its id or label). */
function optionSearchTexts(option: MentionOption, t: Translate): string[] {
  switch (option.kind) {
    case "category":
      return [option.category, t(CATEGORY_META[option.category].labelKey)];
    case "repo":
      return [option.repo.fullName];
    case "session":
      return [sessionTitle(option.session)];
  }
}

/** The popup scrolls ~6 rows and an org installation can expose hundreds. */
const MAX_OPTIONS = 50;

/** The open level's rows, fuzzy-filtered by the typed query and capped. */
export function mentionOptions(
  state: MentionState,
  source: MentionSource,
  t: Translate,
): MentionOption[] {
  const rows: MentionOption[] =
    state.category === null
      ? MENTION_CATEGORIES.map((category) => ({ kind: "category", category }))
      : state.category === "repos"
        ? source.repos.map((repo) => ({ kind: "repo", repo }))
        : source.sessions.map((session) => ({ kind: "session", session }));
  return rows
    .filter((option) =>
      optionSearchTexts(option, t).some((text) =>
        fuzzyMatch(state.query, text),
      ),
    )
    .slice(0, MAX_OPTIONS);
}

/** What an empty list means: no source rows reads differently from no matches. */
export function mentionEmptyText(
  state: MentionState,
  source: MentionSource,
  reposLoading: boolean,
): TranslationKey {
  const category = state.category;
  if (category === null) return "agentSessions.mentionNoResults";
  const rows = category === "repos" ? source.repos : source.sessions;
  if (rows.length > 0) return "agentSessions.mentionNoResults";
  if (category === "repos" && reposLoading) return "common.loading";
  return CATEGORY_META[category].emptyKey;
}
