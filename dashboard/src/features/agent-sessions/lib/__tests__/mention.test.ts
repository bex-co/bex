import { describe, expect, it } from "vitest";
import {
  fuzzyMatch,
  mentionEmptyText,
  mentionOptionGroup,
  mentionOptions,
  mentionStateFromQuery,
} from "@/features/agent-sessions/lib/mention";
import type {
  MentionOption,
  MentionSource,
} from "@/features/agent-sessions/lib/mention";
import type { useTranslations } from "@/common/hooks/use-translations";
import type { RepoView } from "@/features/services/hooks/use-repos";
import { agentSessionView } from "@/test/mocks/agent-session";

type Translate = ReturnType<typeof useTranslations>["t"];

/** The picker only ever asks `t` for category copy; the key itself will do. */
const t = ((key: string) => key) as unknown as Translate;

function repo(fullName: string): RepoView {
  return {
    id: 0,
    fullName,
    private: false,
    defaultBranch: "main",
    htmlUrl: "",
    cloneUrl: "",
    accountLogin: fullName.split("/")[0] ?? "",
  };
}

/** A session whose title (its task) is all `mentionOptions` scores on. */
const sessionView = (id: string, task: string) =>
  agentSessionView({ id, task, phase: "completed" });

function source(fullNames: string[]): MentionSource {
  return { repos: fullNames.map(repo), sessions: [] };
}

/** A row's stable label — `repo:<full>` / `session:<id>` / `category:<name>`. */
function rowLabel(option: MentionOption): string {
  switch (option.kind) {
    case "repo":
      return `repo:${option.repo.fullName}`;
    case "session":
      return `session:${option.session.id}`;
    case "category":
      return `category:${option.category}`;
  }
}

/** The rows the universal (bare-`@`) level renders, as stable labels. */
function universalRows(query: string, src: MentionSource): string[] {
  return mentionOptions({ category: null, query }, src, t).map(rowLabel);
}

/** The repo rows a `@repos:<query>` picker renders, in render order. */
function repoRows(query: string, fullNames: string[]): string[] {
  return mentionOptions({ category: "repos", query }, source(fullNames), t).map(
    (option: MentionOption) =>
      option.kind === "repo" ? option.repo.fullName : option.kind,
  );
}

describe("fuzzyMatch", () => {
  it("matches on prefix, substring, subsequence, and the empty query", () => {
    expect(fuzzyMatch("acme", "acme/anvils")).toBe(true);
    expect(fuzzyMatch("anvil", "acme/anvils")).toBe(true);
    expect(fuzzyMatch("awg", "acme/widgets")).toBe(true);
    expect(fuzzyMatch("", "anything")).toBe(true);
    expect(fuzzyMatch("zzz", "acme/anvils")).toBe(false);
  });
});

describe("mentionOptions repo ranking", () => {
  it("ranks a repo-name match above an org-name match", () => {
    expect(
      repoRows("bex", ["bex-labs/website", "tianpanco/bex7", "bex-labs/infra"]),
    ).toEqual(["tianpanco/bex7", "bex-labs/website", "bex-labs/infra"]);
  });

  it("ranks a name substring above an org prefix", () => {
    expect(repoRows("bex", ["bex-labs/website", "acme/my-bex-app"])).toEqual([
      "acme/my-bex-app",
      "bex-labs/website",
    ]);
  });

  it("prefers a name prefix over a name substring", () => {
    expect(repoRows("anv", ["acme/my-anvils", "acme/anvils"])).toEqual([
      "acme/anvils",
      "acme/my-anvils",
    ]);
  });

  it("sinks subsequence-only matches below every real hit", () => {
    expect(repoRows("bex", ["blue/exports", "bex-labs/site"])).toEqual([
      "bex-labs/site",
      "blue/exports",
    ]);
  });

  it("keeps the source order for an empty query and for ties", () => {
    const all = ["zeta/one", "alpha/two", "mid/three"];
    expect(repoRows("", all)).toEqual(all);
    expect(repoRows("acme", ["acme/one", "acme/two"])).toEqual([
      "acme/one",
      "acme/two",
    ]);
  });

  it("drops non-matching repos", () => {
    expect(repoRows("bex", ["acme/anvils", "tianpanco/bex7"])).toEqual([
      "tianpanco/bex7",
    ]);
  });
});

describe("mentionStateFromQuery", () => {
  it("splits the category token from the filter text", () => {
    expect(mentionStateFromQuery("repos:bex")).toEqual({
      category: "repos",
      query: "bex",
    });
    expect(mentionStateFromQuery("re")).toEqual({
      category: null,
      query: "re",
    });
  });
});

describe("mentionOptions universal level", () => {
  it("surfaces a repo directly for a bare @query — no repos: prefix needed", () => {
    // The headline regression: before this, `@be` at the top level scored only
    // the category labels, so the repo never appeared without `@repos:` first.
    const rows = universalRows("be", source(["bex-co/bex", "acme/anvils"]));
    expect(rows).toContain("repo:bex-co/bex");
    expect(rows).not.toContain("repo:acme/anvils");
  });

  it("guards against the category-only top level ever returning", () => {
    // If the gate came back, `@be` would yield only (non-matching) category
    // rows and thus an empty list. Assert at least one real entity survives.
    const rows = mentionOptions(
      { category: null, query: "be" },
      source(["bex-co/bex"]),
      t,
    );
    expect(rows.some((row) => row.kind === "repo")).toBe(true);
  });

  it("orders categories, then repos, then sessions", () => {
    const rows = universalRows("", {
      repos: [repo("bex-co/bex")],
      sessions: [sessionView("as-1", "fix things")],
    });
    expect(rows).toEqual([
      "category:repos",
      "category:sessions",
      "repo:bex-co/bex",
      "session:as-1",
    ]);
  });

  it("surfaces a matching session by its task title", () => {
    const rows = universalRows("flaky", {
      repos: [repo("acme/anvils")],
      sessions: [
        sessionView("as-1", "Investigate flaky tests"),
        sessionView("as-2", "Ship the billing page"),
      ],
    });
    expect(rows).toContain("session:as-1");
    expect(rows).not.toContain("session:as-2");
    // "flaky" hits no repo, so the repo group drops out entirely.
    expect(rows.every((label) => !label.startsWith("repo:"))).toBe(true);
  });

  it("keeps a matching category row for discoverability", () => {
    const rows = universalRows("rep", source(["acme/anvils"]));
    expect(rows).toContain("category:repos");
    expect(rows).not.toContain("category:sessions");
  });

  it("drops category rows the query does not match", () => {
    const rows = universalRows("be", source(["bex-co/bex"]));
    expect(rows).not.toContain("category:repos");
    expect(rows).not.toContain("category:sessions");
  });

  it("previews only a bounded slice of each entity group on an empty query", () => {
    const repos = Array.from({ length: 20 }, (_, i) => `acme/repo-${i}`);
    const sessions = Array.from({ length: 20 }, (_, i) =>
      sessionView(`as-${i}`, `task ${i}`),
    );
    const rows = mentionOptions(
      { category: null, query: "" },
      { repos: repos.map(repo), sessions },
      t,
    );
    expect(rows.filter((r) => r.kind === "category").length).toBe(2);
    expect(rows.filter((r) => r.kind === "repo").length).toBe(5);
    expect(rows.filter((r) => r.kind === "session").length).toBe(5);
  });

  it("ranks the full entity set for a typed query, past the empty-query preview", () => {
    const repos = Array.from({ length: 8 }, (_, i) => `acme/svc-${i}`);
    const rows = mentionOptions(
      { category: null, query: "svc" },
      source(repos),
      t,
    ).filter((r) => r.kind === "repo");
    expect(rows.length).toBe(8);
  });

  it("leaves the explicit repos: narrowing path unchanged", () => {
    // The shortcut still drills straight to a repos-only ranked list.
    expect(repoRows("bex", ["bex-co/bex", "acme/anvils"])).toEqual([
      "bex-co/bex",
    ]);
  });
});

describe("mentionOptionGroup", () => {
  it("groups entities and leaves category rows ungrouped", () => {
    expect(mentionOptionGroup({ kind: "repo", repo: repo("a/b") })).toBe(
      "repos",
    );
    expect(
      mentionOptionGroup({
        kind: "session",
        session: sessionView("as-1", "x"),
      }),
    ).toBe("sessions");
    expect(mentionOptionGroup({ kind: "category", category: "repos" })).toBe(
      null,
    );
  });
});

describe("mentionEmptyText", () => {
  it("reads as loading at the universal level while repos are still loading", () => {
    const state = { category: null, query: "be" } as const;
    expect(mentionEmptyText(state, source([]), true)).toBe("common.loading");
    expect(mentionEmptyText(state, source([]), false)).toBe(
      "agentSessions.mentionNoResults",
    );
  });

  it("still distinguishes a loading repos second level from no matches", () => {
    const state = { category: "repos", query: "be" } as const;
    expect(mentionEmptyText(state, source([]), true)).toBe("common.loading");
    expect(
      mentionEmptyText(state, { repos: [repo("a/b")], sessions: [] }, true),
    ).toBe("agentSessions.mentionNoResults");
  });
});
