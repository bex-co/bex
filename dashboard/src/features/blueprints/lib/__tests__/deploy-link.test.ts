import { describe, expect, it } from "vitest";
import { parseDeployRepo } from "../deploy-link";

describe("parseDeployRepo", () => {
  it("splits a trailing /tree/<branch>", () => {
    expect(parseDeployRepo("https://github.com/a/b/tree/dev")).toEqual({
      repo: "https://github.com/a/b",
      branch: "dev",
    });
    expect(parseDeployRepo("https://github.com/a/b/tree/feat/x/")).toEqual({
      repo: "https://github.com/a/b",
      branch: "feat/x",
    });
  });

  it("leaves the branch to the repository default otherwise", () => {
    expect(parseDeployRepo(" https://github.com/a/b/ ")).toEqual({
      repo: "https://github.com/a/b",
    });
  });

  it("refuses anything that is not a clone URL", () => {
    expect(parseDeployRepo(undefined)).toBeNull();
    expect(parseDeployRepo("")).toBeNull();
    expect(parseDeployRepo("javascript:alert(1)")).toBeNull();
    expect(parseDeployRepo("http://github.com/a/b")).toBeNull();
  });
});
