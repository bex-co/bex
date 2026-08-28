import { describe, expect, it } from "vitest";
import {
  agentSessionDisplayName,
  sessionTitleShort,
  SESSION_TITLE_MAX,
} from "@/features/agent-sessions/lib/mapper";

const FALLBACK = "Untitled session";

function named(repo: string, task: string) {
  return { repo, agentConfig: { task } };
}

/**
 * w1/m90 t001 — the derivation the detail heading and the document title read.
 * Every assertion reads the returned STRING: the bug this milestone fixes was an
 * `<h1>` that existed and was empty, so "a value came back" proves nothing.
 */
describe("agentSessionDisplayName", () => {
  it("names a repo-backed session by its repository", () => {
    const name = agentSessionDisplayName(
      named("acme/widgets", "refactor the mapper"),
      FALLBACK,
    );
    expect(name.text).toBe("acme/widgets");
    expect(name.full).toBe("acme/widgets");
  });

  it("falls back to the prompt for a repo-less session", () => {
    const name = agentSessionDisplayName(
      named("", "explain the mapper"),
      FALLBACK,
    );
    expect(name.text).toBe("explain the mapper");
    expect(name.full).toBe("explain the mapper");
  });

  it("falls back to the localized last resort when neither is present", () => {
    const name = agentSessionDisplayName(named("", ""), FALLBACK);
    expect(name.text).toBe(FALLBACK);
    expect(name.full).toBe(FALLBACK);
  });

  it("treats whitespace-only values as absent", () => {
    expect(
      agentSessionDisplayName(named("   ", "  chat  "), FALLBACK).text,
    ).toBe("chat");
    expect(agentSessionDisplayName(named(" ", " "), FALLBACK).text).toBe(
      FALLBACK,
    );
  });

  it("truncates a long prompt on a word boundary and keeps the full text", () => {
    const task = `${"refactor the mapper ".repeat(20)}end`;
    expect(task.length).toBeGreaterThan(SESSION_TITLE_MAX);

    const name = agentSessionDisplayName(named("", task), FALLBACK);
    expect(name.text.length).toBeLessThanOrEqual(SESSION_TITLE_MAX + 1);
    expect(name.text.endsWith("…")).toBe(true);
    // Ends on a whole word, not mid-token, and never on a dangling space.
    expect(name.text).not.toMatch(/ …$/);
    expect(task.startsWith(name.text.slice(0, -1))).toBe(true);
    // The untruncated source stays reachable for a title/tooltip.
    expect(name.full).toBe(task);
  });

  it("hard-cuts a single token with no word boundary to cut on", () => {
    const task = "a".repeat(500);
    const name = agentSessionDisplayName(named("", task), FALLBACK);
    expect(name.text).toBe(`${"a".repeat(SESSION_TITLE_MAX)}…`);
    expect(name.full).toBe(task);
  });

  it("shares its truncation with the list-row title, which stays prompt-first", () => {
    const task = `${"refactor the mapper ".repeat(20)}end`;
    // Row titles tell sibling sessions apart, so the prompt leads there even
    // when a repo exists — the two orders differ on purpose, the cut does not.
    expect(sessionTitleShort({ id: "as-1", agentConfig: { task } })).toBe(
      agentSessionDisplayName(named("", task), FALLBACK).text,
    );
  });
});
