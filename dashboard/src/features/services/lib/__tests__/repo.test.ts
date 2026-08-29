import { describe, expect, it } from "vitest";

import { formatRepoLabel, repoBrowseUrl } from "../repo";

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
