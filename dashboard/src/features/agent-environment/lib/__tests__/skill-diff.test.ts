import { describe, expect, it } from "vitest";
import {
  parseSkillDiff,
  skillContentLines,
} from "@/features/agent-environment/lib/skill-diff";

describe("parseSkillDiff", () => {
  it("numbers context, removed and added lines from the hunk header", () => {
    const rows = parseSkillDiff("@@ -3,3 +3,3 @@\n keep\n-old\n+new\n tail\n");
    expect(rows).toEqual([
      { kind: "hunk", text: "@@ -3,3 +3,3 @@" },
      { kind: "context", oldLine: 3, newLine: 3, text: "keep" },
      { kind: "remove", oldLine: 4, newLine: null, text: "old" },
      { kind: "add", oldLine: null, newLine: 4, text: "new" },
      { kind: "context", oldLine: 5, newLine: 5, text: "tail" },
    ]);
  });

  it("renders the first version as all additions", () => {
    const rows = parseSkillDiff("@@ -0,0 +1,2 @@\n+---\n+name: a\n");
    expect(rows.filter((r) => r.kind === "add").map((r) => r.newLine)).toEqual([
      1, 2,
    ]);
    expect(rows.some((r) => r.kind === "remove")).toBe(false);
  });

  it("yields no rows when nothing changed", () => {
    expect(parseSkillDiff("")).toEqual([]);
    expect(parseSkillDiff(null)).toEqual([]);
    expect(parseSkillDiff(undefined)).toEqual([]);
  });

  it("restarts the numbering at each hunk", () => {
    const rows = parseSkillDiff(
      "@@ -1,1 +1,1 @@\n-a\n+b\n@@ -10 +10,2 @@\n x\n+y\n",
    );
    expect(rows.filter((r) => r.kind === "hunk")).toHaveLength(2);
    expect(rows.at(-2)).toMatchObject({
      kind: "context",
      oldLine: 10,
      newLine: 10,
    });
    expect(rows.at(-1)).toMatchObject({ kind: "add", newLine: 11 });
  });

  it("ignores no-newline markers and anything before the first hunk", () => {
    const rows = parseSkillDiff(
      "stray\n@@ -1 +1 @@\n-a\n\\ No newline at end of file\n+b\n\\ No newline at end of file\n",
    );
    expect(rows.map((r) => r.kind)).toEqual(["hunk", "remove", "add"]);
  });

  it("keeps an interior blank line as blank context", () => {
    const rows = parseSkillDiff("@@ -1,3 +1,3 @@\n a\n\n c\n");
    expect(rows[2]).toMatchObject({ kind: "context", text: "" });
  });
});

describe("parseSkillDiff edge cases", () => {
  it("never mistakes file content for a header or a marker", () => {
    // A removed "---" fence reads "----"; a removed "@@" line reads "-@@ ...".
    const rows = parseSkillDiff(
      "@@ -1,3 +1,2 @@\n----\n-@@ -9 +9 @@\n+++ b\n x\n",
    );
    expect(rows.map((r) => r.kind)).toEqual([
      "hunk",
      "remove",
      "remove",
      "add",
      "context",
    ]);
    expect(rows.map((r) => r.text).slice(1)).toEqual([
      "---",
      "@@ -9 +9 @@",
      "++ b",
      "x",
    ]);
  });

  it("keeps a carriage return as part of the line", () => {
    const rows = parseSkillDiff("@@ -1,1 +1,1 @@\n-a\r\n+b\r\n");
    expect(rows.map((r) => r.text).slice(1)).toEqual(["a\r", "b\r"]);
  });

  it("numbers an empty old file from line 1 and an emptied file with no new lines", () => {
    const added = parseSkillDiff("@@ -0,0 +1,2 @@\n+a\n+b\n");
    expect(added.slice(1)).toMatchObject([
      { kind: "add", oldLine: null, newLine: 1 },
      { kind: "add", oldLine: null, newLine: 2 },
    ]);
    const removed = parseSkillDiff("@@ -1,2 +0,0 @@\n-a\n-b\n");
    expect(removed.slice(1)).toMatchObject([
      { kind: "remove", oldLine: 1, newLine: null },
      { kind: "remove", oldLine: 2, newLine: null },
    ]);
  });

  it("gives a diff with no hunk header no rows", () => {
    expect(parseSkillDiff("+a\n-b\n")).toEqual([]);
  });
});

describe("skillContentLines", () => {
  it("splits a file without the empty tail", () => {
    expect(skillContentLines("a\n\nb\n")).toEqual(["a", "", "b"]);
    expect(skillContentLines(null)).toEqual([]);
    expect(skillContentLines("")).toEqual([]);
    expect(skillContentLines("no newline")).toEqual(["no newline"]);
    expect(skillContentLines("a\r\nb\r\n")).toEqual(["a\r", "b\r"]);
  });
});
