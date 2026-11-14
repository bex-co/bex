import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";
import {
  AGENT_OPTIONS,
  DATA_USE_AGENTS,
  selectableAgents,
} from "@/features/agent-sessions/lib/composer";

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

describe("agent options and the release manifest", () => {
  const manifest = JSON.parse(
    readFileSync(
      resolve(
        __dirname,
        "../../../../../../lego/backend/internal/agentsession/agent-profiles.json",
      ),
      "utf8",
    ),
  ) as {
    profiles: { id: string; requiresDataUseAcknowledgement?: boolean }[];
  };

  it("offers exactly the manifest's profiles", () => {
    expect([...AGENT_OPTIONS].sort()).toEqual(
      manifest.profiles.map((p) => p.id).sort(),
    );
  });

  it("marks exactly the profiles that need a data-use acknowledgement", () => {
    expect([...DATA_USE_AGENTS].sort()).toEqual(
      manifest.profiles
        .filter((p) => p.requiresDataUseAcknowledgement)
        .map((p) => p.id)
        .sort(),
    );
  });
});
