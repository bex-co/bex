import { describe, expect, it } from "vitest";

import { isValidGitUrl } from "./git-url";

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
