import { hasEvidence, isGitHubPrUrl, isGitHubUrl } from "../evidence";

describe("agent-session evidence helpers", () => {
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

  it("reports evidence only when a section is non-empty", () => {
    expect(hasEvidence(null)).toBe(false);
    expect(hasEvidence({})).toBe(false);
    expect(
      hasEvidence({ commandLog: [], testOutput: [], outputTail: "" }),
    ).toBe(false);
    expect(hasEvidence({ outputTail: "   " })).toBe(false);
    expect(hasEvidence({ commandLog: ["npm test"] })).toBe(true);
    expect(hasEvidence({ changedFiles: ["a.ts"] })).toBe(true);
    expect(hasEvidence({ outputTail: "ok" })).toBe(true);
  });
});
