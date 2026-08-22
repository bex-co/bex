import { bumpRepo, rankRepos, type RepoFrequency } from "../repo-frequency";

const available = ["org/a", "org/b", "org/c", "org/d"];

describe("composer repo frequency", () => {
  it("ranks recorded repos by count, then recency", () => {
    const freq: RepoFrequency = {
      "org/a": { count: 2, lastUsed: 10 },
      "org/b": { count: 5, lastUsed: 1 },
      "org/c": { count: 2, lastUsed: 30 },
    };
    // b (count 5) first; a and c tie on count 2 so newer lastUsed (c) wins.
    expect(rankRepos(freq, available, 3)).toEqual(["org/b", "org/c", "org/a"]);
  });

  it("fills remaining slots from available in list order", () => {
    const freq: RepoFrequency = { "org/c": { count: 3, lastUsed: 5 } };
    expect(rankRepos(freq, available, 3)).toEqual(["org/c", "org/a", "org/b"]);
  });

  it("never surfaces a repo missing from the available list", () => {
    const freq: RepoFrequency = { "org/gone": { count: 9, lastUsed: 9 } };
    expect(rankRepos(freq, available, 2)).toEqual(["org/a", "org/b"]);
  });

  it("honors the limit and a zero limit", () => {
    expect(rankRepos({}, available, 2)).toEqual(["org/a", "org/b"]);
    expect(rankRepos({}, available, 0)).toEqual([]);
  });

  it("bumpRepo increments count, stamps recency, and ignores blanks", () => {
    const once = bumpRepo({}, "org/a", 100);
    expect(once["org/a"]).toEqual({ count: 1, lastUsed: 100 });
    const twice = bumpRepo(once, "org/a", 200);
    expect(twice["org/a"]).toEqual({ count: 2, lastUsed: 200 });
    expect(bumpRepo({}, "   ", 1)).toEqual({});
  });
});
