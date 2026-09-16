import { describe, expect, it } from "vitest";

import { formatRepoLabel, repoBrowseUrl, repoCommitUrl } from "../repo";

describe("repository display helpers", () => {
  it("keeps an arbitrary self-hosted forge visible in linked text", () => {
    const repo = "https://attacker.example/acme/web.git";

    expect(formatRepoLabel(repo)).toBe("attacker.example · acme / web");
    expect(repoBrowseUrl(repo, "main")).toBe(
      "https://attacker.example/acme/web/tree/main",
    );
  });

  it("shows the real authority when URL userinfo contains a trusted-looking host", () => {
    const repo = "https://github.com@attacker.example/acme/web.git";

    expect(formatRepoLabel(repo)).toBe("attacker.example · acme / web");
    expect(repoBrowseUrl(repo, "main")).toBe(
      "https://attacker.example/acme/web/tree/main",
    );
  });

  it("keeps the host visible for scp-style SSH clone URLs", () => {
    expect(formatRepoLabel("git@git.example:owner/repo.git")).toBe(
      "git.example · owner / repo",
    );
  });
});

describe("repoCommitUrl", () => {
  const sha = "abc1234def5678";

  it("points a repo-backed deploy's SHA at the commit page (the diff)", () => {
    expect(repoCommitUrl("https://github.com/acme/web.git", sha)).toBe(
      "https://github.com/acme/web/commit/abc1234def5678",
    );
    expect(repoCommitUrl("git@github.com:acme/web.git", sha)).toBe(
      "https://github.com/acme/web/commit/abc1234def5678",
    );
  });

  it("keeps a self-hosted forge's real host in the link", () => {
    expect(repoCommitUrl("https://git.example/acme/web", sha)).toBe(
      "https://git.example/acme/web/commit/abc1234def5678",
    );
  });

  it("refuses anything that isn't a hex commit id in the path", () => {
    expect(repoCommitUrl("https://github.com/acme/web.git", "")).toBeNull();
    expect(
      repoCommitUrl("https://github.com/acme/web.git", "../../settings"),
    ).toBeNull();
    expect(repoCommitUrl("https://github.com/acme/web.git", "abc")).toBeNull();
  });

  it("yields no link for a repo that can't be browsed", () => {
    expect(repoCommitUrl("acme/web", sha)).toBeNull();
    expect(repoCommitUrl("", sha)).toBeNull();
  });
});
