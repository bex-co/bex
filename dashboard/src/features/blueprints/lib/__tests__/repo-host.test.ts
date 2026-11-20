import { describe, expect, it } from "vitest";
import { blueprintRepoHost } from "../repo-host";

// w8/346: the review's failure copy names where the repository lives, as
// bex-api reads it: GitHub through its app, gitlab.com and bitbucket.org
// anonymously, any other host refused by name.
describe("blueprintRepoHost", () => {
  it.each([
    ["https://github.com/acme/app", { kind: "github" }],
    ["https://github.com/acme/app.git", { kind: "github" }],
    ["git@github.com:acme/app.git", { kind: "github" }],
    ["ssh://git@github.com/acme/app.git", { kind: "github" }],
    [
      "https://GitLab.com/group/sub/app",
      { kind: "public", host: "gitlab.com" },
    ],
    [
      "https://bitbucket.org/acme/app",
      { kind: "public", host: "bitbucket.org" },
    ],
    [
      "https://codeberg.org/forgejo/forgejo",
      { kind: "unsupported", host: "codeberg.org" },
    ],
    [
      "https://gitlab.example.com/a/b",
      { kind: "unsupported", host: "gitlab.example.com" },
    ],
    ["git@gitlab.com:acme/app.git", { kind: "public", host: "gitlab.com" }],
    // No host to name: the copy the page always showed.
    ["", { kind: "github" }],
  ])("reads %s", (repo, want) => {
    expect(blueprintRepoHost(repo)).toEqual(want);
  });
});
