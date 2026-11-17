import { describe, expect, it } from "vitest";
import { parseDeployReferrer, parseDeployRepo } from "../deploy-link";

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

// w33/003: a button with no `repo` deploys the page that linked to it.
describe("parseDeployReferrer", () => {
  const own = "https://dashboard.bex.co";

  it("reads a repository page as its repository", () => {
    expect(parseDeployReferrer("https://gitlab.com/a/b", own)).toEqual({
      repo: "https://gitlab.com/a/b",
    });
    expect(
      parseDeployReferrer("https://github.com/a/b/?tab=readme-ov-file#x", own),
    ).toEqual({ repo: "https://github.com/a/b" });
  });

  it("reads a branch page as that branch", () => {
    expect(
      parseDeployReferrer("https://github.com/a/b/tree/feat/x", own),
    ).toEqual({ repo: "https://github.com/a/b", branch: "feat/x" });
    expect(
      parseDeployReferrer("https://github.com/a/b/tree/f%C3%A9", own),
    ).toEqual({ repo: "https://github.com/a/b", branch: "fé" });
  });

  it("names nothing for no referrer, an origin alone, or any other page", () => {
    expect(parseDeployReferrer("", own)).toBeNull();
    expect(parseDeployReferrer("https://github.com/", own)).toBeNull();
    expect(parseDeployReferrer("https://github.com/a", own)).toBeNull();
    expect(
      parseDeployReferrer("https://github.com/a/b/issues", own),
    ).toBeNull();
    expect(parseDeployReferrer("https://github.com/a/b/tree/", own)).toBeNull();
    expect(parseDeployReferrer("http://example.com/a/b", own)).toBeNull();
  });

  it("names nothing for a page of this dashboard, whatever the scheme it sees", () => {
    expect(
      parseDeployReferrer("https://dashboard.bex.co/services/srv-x", own),
    ).toBeNull();
    expect(
      parseDeployReferrer(
        "https://dashboard.bex.co/services/srv-x",
        "http://dashboard.bex.co",
      ),
    ).toBeNull();
  });
});
