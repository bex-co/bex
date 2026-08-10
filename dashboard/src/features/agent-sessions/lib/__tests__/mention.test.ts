import { describe, expect, it } from "vitest";
import {
  fuzzyMatch,
  mentionOptions,
  mentionStateFromQuery,
} from "@/features/agent-sessions/lib/mention";
import type {
  MentionOption,
  MentionSource,
} from "@/features/agent-sessions/lib/mention";
import type { useTranslations } from "@/common/hooks/use-translations";
import type { RepoView } from "@/features/services/hooks/use-repos";

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
  };
}

function source(fullNames: string[]): MentionSource {
  return { repos: fullNames.map(repo), sessions: [] };
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
