import { isGitHubPrUrl, isGitHubUrl } from "../github-links";

describe("agent-session GitHub link guards", () => {
  it("opens only https github.com hosts (shared link guard)", () => {
    expect(isGitHubUrl("https://github.com/apps/bex/installations/new")).toBe(
      true,
    );
    expect(isGitHubUrl("https://github.com/org/repo")).toBe(true);
    expect(isGitHubUrl("http://github.com/x")).toBe(false);
    expect(isGitHubUrl("https://github.com.evil.com/x")).toBe(false);
    expect(isGitHubUrl("https://evil.com/github.com")).toBe(false);
    expect(isGitHubUrl("javascript:alert(1)")).toBe(false);
    expect(isGitHubUrl(null)).toBe(false);
  });

  it("opens only canonical https GitHub pull URLs", () => {
    expect(isGitHubPrUrl("https://github.com/org/repo/pull/12")).toBe(true);
    expect(isGitHubPrUrl("https://github.com/org/repo/pull/12/files")).toBe(
      false,
    );
    expect(isGitHubPrUrl("http://github.com/org/repo/pull/12")).toBe(false);
    expect(isGitHubPrUrl("https://evil.com/org/repo/pull/12")).toBe(false);
    expect(isGitHubPrUrl("https://github.com.evil.com/o/r/pull/1")).toBe(false);
    expect(isGitHubPrUrl("javascript:alert(1)")).toBe(false);
    expect(isGitHubPrUrl(null)).toBe(false);
    expect(isGitHubPrUrl("")).toBe(false);
  });
});
