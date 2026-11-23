import { describe, expect, it } from "vitest";
import { agentEnvironment } from "@/test/mocks/agent-environment";
import {
  GITHUB_CLI_CHOICES,
  githubCliBinding,
  githubCliChoice,
  githubCliLock,
  githubCliMandatoryOff,
  inheritedGithubCli,
  type GithubCliChoice,
} from "@/features/agent-sessions/lib/github-cli";

// w10/m561: the gh binding chain as the dashboard mirrors it, so a choice
// bex-api would refuse is shown disabled with its reason.
describe("the gh binding", () => {
  it("round-trips a choice through the binding it sends", () => {
    for (const binding of [null, true, false] as const) {
      expect(githubCliBinding(githubCliChoice(binding))).toBe(
        binding ?? undefined,
      );
    }
    expect(githubCliChoice(undefined)).toBe("inherit");
  });

  it("finds the floor only while gh is off and mandatory", () => {
    expect(githubCliMandatoryOff(null)).toBe(false);
    expect(
      githubCliMandatoryOff(
        agentEnvironment({ githubCliEnabled: true, githubCliMandatory: true }),
      ),
    ).toBe(false);
    expect(
      githubCliMandatoryOff(
        agentEnvironment({
          githubCliEnabled: false,
          githubCliMandatory: false,
        }),
      ),
    ).toBe(false);
    expect(
      githubCliMandatoryOff(
        agentEnvironment({ githubCliEnabled: false, githubCliMandatory: true }),
      ),
    ).toBe(true);
  });

  // [workspace switch, automations default, mandatory] → [session, automation]
  it.each([
    [true, null, false, true, true],
    [false, null, false, false, false],
    [true, false, false, true, false],
    [false, true, false, false, true],
    // The floor wins over the automations default.
    [false, true, true, false, false],
    // No effect while the switch is on.
    [true, false, true, true, false],
  ] as const)(
    "inherits: switch %s, automations %s, mandatory %s → session %s, automation %s",
    (
      githubCliEnabled,
      automationsGithubCliEnabled,
      githubCliMandatory,
      session,
      automation,
    ) => {
      const env = agentEnvironment({
        githubCliEnabled,
        automationsGithubCliEnabled,
        githubCliMandatory,
      });
      expect(inheritedGithubCli(env, "session")).toBe(session);
      expect(inheritedGithubCli(env, "automation")).toBe(automation);
    },
  );

  it("knows nothing before the environment loads", () => {
    expect(inheritedGithubCli(null, "session")).toBeNull();
    expect(inheritedGithubCli(null, "automation")).toBeNull();
  });

  it("locks every change for a member and on under the floor, never the stored choice (w10/044)", () => {
    const floor = agentEnvironment({
      githubCliEnabled: false,
      githubCliMandatory: true,
    });
    const open = agentEnvironment();
    const locks = (
      env: typeof open | null,
      canManage: boolean,
      stored?: GithubCliChoice,
    ) =>
      GITHUB_CLI_CHOICES.map((choice) =>
        githubCliLock({ choice, stored, env, canManage }),
      );
    // An admin chooses anything, except on under the floor.
    expect(locks(open, true)).toEqual([null, null, null]);
    expect(locks(null, true)).toEqual([null, null, null]);
    expect(locks(floor, true)).toEqual([null, "mandatory", null]);
    // A member keeps inherit on a create, whatever the environment says.
    expect(locks(open, false)).toEqual([null, "admin", "admin"]);
    expect(locks(floor, false)).toEqual([null, "admin", "admin"]);
    // An edit keeps its stored binding, on under the floor included, as the
    // server keeps an unchanged one; a member changes nothing.
    expect(locks(floor, true, "on")).toEqual([null, null, null]);
    expect(locks(floor, false, "on")).toEqual(["admin", null, "admin"]);
    expect(locks(open, false, "off")).toEqual(["admin", "admin", null]);
  });
});
