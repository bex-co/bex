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

/** The typed token a category selection inserts into the composer editor. */
export function mentionToken(category: MentionCategory): string {
  return `@${category}:`;
}

/**
 * The open mention popup. `category: null` is the first level (choosing a
 * category); a set category is the second (choosing one of its items).
 */
export interface MentionState {
  category: MentionCategory | null;
  /** The filter text typed after `@` or the selected category token. */
  query: string;
}

/** Decode Tiptap Suggestion's query into the picker's two levels. */
export function mentionStateFromQuery(query: string): MentionState {
  for (const category of MENTION_CATEGORIES) {
    const prefix = `${category}:`;
    if (query.startsWith(prefix)) {
      return { category, query: query.slice(prefix.length) };
    }
  }
  return { category: null, query };
}

/** How well a query hit a candidate — the ranking's primary signal. */
const MATCH_NONE = 0;
const MATCH_SUBSEQUENCE = 1;
const MATCH_SUBSTRING = 2;
const MATCH_PREFIX = 3;

/**
 * Dependency-free fuzzy match quality: prefix, then case-insensitive
 * `includes`, then an in-order subsequence scan (`tianpan` matches
 * `android-tianpanco-release`; `awg` matches `acme/widgets`). An empty query
 * matches everything at `MATCH_PREFIX`, so it leaves the source order intact.
 */
function matchQuality(query: string, candidate: string): number {
  const q = query.trim().toLowerCase();
  const c = candidate.toLowerCase();
  if (c.startsWith(q)) return MATCH_PREFIX;
  if (c.includes(q)) return MATCH_SUBSTRING;
  let i = 0;
  for (const ch of c) {
    if (ch === q[i]) i += 1;
    if (i === q.length) return MATCH_SUBSEQUENCE;
  }
  return MATCH_NONE;
}

/** Whether the query hits the candidate at all (any quality). */
export function fuzzyMatch(query: string, candidate: string): boolean {
  return matchQuality(query, candidate) !== MATCH_NONE;
}

/** One row of the open picker — a category, a repo, or a prior session. */
export type MentionOption =
  | { kind: "category"; category: MentionCategory }
  | { kind: "repo"; repo: RepoView }
  | { kind: "session"; session: AgentSessionView };

/** Stable per-option DOM id, referenced by the editor's aria-activedescendant. */
export function mentionOptionId(idBase: string, index: number): string {
  return `${idBase}-option-${index}`;
}

/** The rows a mention level offers, before filtering. */
export interface MentionSource {
  repos: RepoView[];
  sessions: AgentSessionView[];
}

/** The bare repo name — `acme/widgets` → `widgets`; no owner ⇒ the whole name. */
function repoName(fullName: string): string {
  const slash = fullName.lastIndexOf("/");
  return slash === -1 ? fullName : fullName.slice(slash + 1);
}

/**
 * Every text a row can be matched against, **most significant first** — a repo
 * leads with its bare name so `@repos:bex` ranks `someone/bex7` above every
 * repo that merely lives under a `bex…` org (a category matches its id or
 * label; the repo's `fullName` still carries the owner match).
 */
function optionSearchTexts(option: MentionOption, t: Translate): string[] {
  switch (option.kind) {
    case "category":
      return [option.category, t(CATEGORY_META[option.category].labelKey)];
    case "repo":
      return [repoName(option.repo.fullName), option.repo.fullName];
    case "session":
      return [sessionTitle(option.session)];
  }
}

/**
 * A row's relevance: the best (field priority, match quality) pair it offers.
 * Prefix/substring hits form a strong band ordered by field first — a repo-name
 * hit outranks an owner-only one — while the noisier subsequence hits all sink
 * below it. `0` means the row does not match at all.
 */
function optionScore(
  query: string,
  option: MentionOption,
  t: Translate,
): number {
  let best = 0;
  optionSearchTexts(option, t).forEach((text, field) => {
    const quality = matchQuality(query, text);
    if (quality === MATCH_NONE) return;
    const score =
      quality === MATCH_SUBSEQUENCE ? quality : 100 - field * 10 + quality;
    if (score > best) best = score;
  });
  return best;
}

/** The popup scrolls ~6 rows and an org installation can expose hundreds. */
const MAX_OPTIONS = 50;

/**
 * How many rows of each entity group the universal level previews when the
 * query is empty — opening `@` browses a bounded slice of repos and sessions
 * rather than dumping every one an installation exposes.
 */
const MAX_PREVIEW = 5;

/**
 * Fuzzy-filter rows by the typed query, rank by relevance, and cap. The sort is
 * stable, so an empty query (every row scores alike) and same-relevance ties
 * both keep the source order — and the cap keeps the best matches rather than
 * the first `limit`.
 */
function rankAndCap(
  rows: MentionOption[],
  query: string,
  t: Translate,
  limit: number,
): MentionOption[] {
  return rows
    .map((option) => ({ option, score: optionScore(query, option, t) }))
    .filter((scored) => scored.score > 0)
    .sort((a, b) => b.score - a.score)
    .slice(0, limit)
    .map((scored) => scored.option);
}

/**
 * The open level's rows. The two second levels (`@repos:`/`@sessions:`) stay a
 * single ranked entity list, unchanged. The first level is now **universal**:
 * category rows (the drill-down affordance), repos, and sessions are scored
 * against one query and returned grouped by type, so `@be` surfaces `owner/bex`
 * directly without the `repos:` hop (Devin/Cursor's model, docs/ADR047 D9). An
 * empty query previews a bounded slice of each entity group; a typed query
 * ranks the full set and keeps a matching category row for discoverability.
 */
export function mentionOptions(
  state: MentionState,
  source: MentionSource,
  t: Translate,
): MentionOption[] {
  const repoRows: MentionOption[] = source.repos.map((repo) => ({
    kind: "repo",
    repo,
  }));
  const sessionRows: MentionOption[] = source.sessions.map((session) => ({
    kind: "session",
    session,
  }));
  if (state.category === "repos") {
    return rankAndCap(repoRows, state.query, t, MAX_OPTIONS);
  }
  if (state.category === "sessions") {
    return rankAndCap(sessionRows, state.query, t, MAX_OPTIONS);
  }

  const entityLimit = state.query.trim() === "" ? MAX_PREVIEW : MAX_OPTIONS;
  const categoryRows: MentionOption[] = MENTION_CATEGORIES.map((category) => ({
    kind: "category",
    category,
  }));
  // Groups must stay contiguous: the picker derives section headers from
  // group changes between adjacent rows, so interleaving kinds here would
  // emit duplicate/misplaced headers.
  return [
    ...rankAndCap(categoryRows, state.query, t, MENTION_CATEGORIES.length),
    ...rankAndCap(repoRows, state.query, t, entityLimit),
    ...rankAndCap(sessionRows, state.query, t, entityLimit),
  ];
}

/**
 * The section a row renders under — `null` for the ungrouped category rows,
 * which sit above the entity groups as the drill-down affordance.
 */
export function mentionOptionGroup(
  option: MentionOption,
): MentionCategory | null {
  switch (option.kind) {
    case "repo":
      return "repos";
    case "session":
      return "sessions";
    case "category":
      return null;
  }
}

/** What an empty list means: no source rows reads differently from no matches. */
export function mentionEmptyText(
  state: MentionState,
  source: MentionSource,
  reposLoading: boolean,
): TranslationKey {
  const category = state.category;
  if (category === null) {
    // The universal level is only ever empty for a typed query that matched
    // nothing (an empty query always keeps the category rows). While the repo
    // list is still arriving, that reads as loading, not "nothing matches".
    return reposLoading ? "common.loading" : "agentSessions.mentionNoResults";
  }
  const rows = category === "repos" ? source.repos : source.sessions;
  if (rows.length > 0) return "agentSessions.mentionNoResults";
  if (category === "repos" && reposLoading) return "common.loading";
  return CATEGORY_META[category].emptyKey;
}
