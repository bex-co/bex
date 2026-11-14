import { describe, expect, it } from "vitest";
import { selectableAgents } from "@/features/agent-sessions/lib/composer";

describe("selectableAgents", () => {
  it("offers exactly the agents the workspace's capabilities list", () => {
    expect(selectableAgents(["muse", "claude"])).toEqual(["claude", "muse"]);
    expect(selectableAgents([], "codex")).toEqual(["codex"]);
  });

  it("offers only the current agent until capabilities load", () => {
    expect(selectableAgents(undefined)).toEqual(["claude"]);
    expect(selectableAgents(undefined, "gemini")).toEqual(["gemini"]);
  });

  it("keeps an already-saved agent listed so its value still renders", () => {
    expect(selectableAgents(["claude"], "muse")).toEqual(["claude", "muse"]);
    expect(selectableAgents(undefined, "muse")).toContain("muse");
  });
});
