import { describe, expect, it } from "vitest";

import { isValidGitUrl, parseGitHubPage } from "./git-url";

// The client accepts exactly the repo URLs bex-api's store.RepoRule admits
// (w8/130): https://, or one of GitHub's SSH spellings, which the build fetches
// over HTTPS. Another host's SSH URL, http:// and git:// would be refused there.
describe("isValidGitUrl", () => {
  it.each([
    "https://github.com/you/your-repo",
    "https://gitlab.com/you/your-repo.git",
    "git@github.com:you/your-repo.git",
    "ssh://git@github.com/you/your-repo.git",
    "ssh://github.com/you/your-repo",
    "  https://github.com/you/your-repo  ",
  ])("accepts %s", (url) => {
    expect(isValidGitUrl(url)).toBe(true);
  });

  it.each([
    "http://github.com/you/your-repo",
    "git://github.com/you/your-repo",
    "git@gitlab.com:you/your-repo.git",
    "ssh://git@gitlab.com/you/your-repo.git",
    "github.com/you/your-repo",
    "",
  ])("refuses %s", (url) => {
    expect(isValidGitUrl(url)).toBe(false);
  });
});

// w4/576: the URL a browser shows for a GitHub folder or file is no clone URL.
describe("parseGitHubPage", () => {
  it.each([
    [
      "https://github.com/bex-co/bex/tree/main/examples/hello-go",
      {
        repo: "https://github.com/bex-co/bex",
        branch: "main",
        rootDir: "examples/hello-go",
        file: false,
      },
    ],
    [
      "https://github.com/bex-co/bex/tree/main/",
      {
        repo: "https://github.com/bex-co/bex",
        branch: "main",
        rootDir: "",
        file: false,
      },
    ],
    [
      "https://github.com/bex-co/bex.git/blob/v1/README.md",
      {
        repo: "https://github.com/bex-co/bex",
        branch: "v1",
        rootDir: "README.md",
        file: true,
      },
    ],
    [
      "https://GitHub.com/bex-co/bex/tree/main/my%20dir?tab=readme#top",
      {
        repo: "https://github.com/bex-co/bex",
        branch: "main",
        rootDir: "my dir",
        file: false,
      },
    ],
  ])("splits %s", (url, want) => {
    expect(parseGitHubPage(url)).toEqual(want);
  });

  it.each([
    "https://github.com/bex-co/bex",
    "https://github.com/bex-co/tree",
    "https://gitlab.com/acme/app/-/tree/main",
    "git@github.com:bex-co/bex.git",
  ])("leaves %s alone", (url) => {
    expect(parseGitHubPage(url)).toBeNull();
  });
});
