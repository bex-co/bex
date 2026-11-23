import { describe, expect, it } from "vitest";
import { agentEnvironment } from "@/test/mocks/agent-environment";
import {
  GITHUB_CLI_CHOICES,
  githubCliBinding,
  githubCliChoice,
  githubCliLock,
  githubCliMandatoryOff,
  githubCliChainOf,
  inheritedGithubCli,
  inheritedSessionGithubCli,
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

// w10/m599: what a started session's inherit resolves to in its settings,
// through the chain bex-api walks.
describe("a started session's inherit", () => {
  type Node = Parameters<typeof githubCliChainOf>[0];
  const node = (
    id: string,
    githubCli: boolean | null,
    parentSessionId: string | null = null,
    automationId: string | null = null,
  ): Node => ({
    id,
    automationId,
    parentSessionId,
    agentConfig: { githubCli },
  });

  it("walks a child's tree up to its root, at most three sessions", () => {
    const tree = [
      node("ags-0", true, null, "aut-1"),
      node("ags-1", false, "ags-0"),
      node("ags-2", null, "ags-1"),
      node("ags-3", null, "ags-2"),
    ];
    expect(githubCliChainOf(node("ags-x", null), null)).toEqual({
      ancestors: [],
      rootAutomationId: null,
    });
    expect(githubCliChainOf(node("ags-2", null, "ags-1"), tree)).toEqual({
      ancestors: [true, false],
      rootAutomationId: "aut-1",
    });
    // Three ancestors deep, the walk stops at ags-1: ags-0 is not read.
    expect(githubCliChainOf(node("ags-4", null, "ags-3"), tree)).toEqual({
      ancestors: [false, null, null],
      rootAutomationId: null,
    });
    // A deleted parent makes the child a top-level session.
    expect(githubCliChainOf(node("ags-y", false, "ags-gone"), tree)).toEqual({
      ancestors: [],
      rootAutomationId: null,
    });
    expect(githubCliChainOf(node("ags-2", null, "ags-1"), null)).toBeNull();
  });

  const person = { ancestors: [], rootAutomationId: null };
  const automated = { ancestors: [], rootAutomationId: "aut-1" };

  it("follows the workspace's switch for a session a person started", () => {
    expect(inheritedSessionGithubCli(agentEnvironment(), person)).toBe(true);
    expect(
      inheritedSessionGithubCli(
        agentEnvironment({
          githubCliEnabled: false,
          automationsGithubCliEnabled: true,
        }),
        person,
      ),
    ).toBe(false);
  });

  it("follows the automation, then the automations default, for an automation's session", () => {
    const env = agentEnvironment({ automationsGithubCliEnabled: false });
    expect(inheritedSessionGithubCli(env, automated, true)).toBe(true);
    expect(inheritedSessionGithubCli(env, automated, null)).toBe(false);
    expect(inheritedSessionGithubCli(agentEnvironment(), automated, null)).toBe(
      true,
    );
    // Unknown until the automation is read.
    expect(inheritedSessionGithubCli(env, automated)).toBeNull();
  });

  it("lets a child's root turn gh on or off, and any off below it win", () => {
    const off = agentEnvironment({ githubCliEnabled: false });
    expect(
      inheritedSessionGithubCli(off, {
        ancestors: [true],
        rootAutomationId: null,
      }),
    ).toBe(true);
    expect(
      inheritedSessionGithubCli(agentEnvironment(), {
        ancestors: [false],
        rootAutomationId: null,
      }),
    ).toBe(false);
    // Below the root only an off counts: a middle session's on does not.
    expect(
      inheritedSessionGithubCli(off, {
        ancestors: [null, true],
        rootAutomationId: null,
      }),
    ).toBe(false);
    expect(
      inheritedSessionGithubCli(agentEnvironment(), {
        ancestors: [true, false],
        rootAutomationId: null,
      }),
    ).toBe(false);
  });

  it("knows nothing before the environment or the tree loads, and off under a mandatory off", () => {
    expect(inheritedSessionGithubCli(null, person)).toBeNull();
    expect(inheritedSessionGithubCli(agentEnvironment(), null)).toBeNull();
    const floor = agentEnvironment({
      githubCliEnabled: false,
      githubCliMandatory: true,
      automationsGithubCliEnabled: true,
    });
    for (const chain of [person, automated, null]) {
      expect(inheritedSessionGithubCli(floor, chain, true)).toBe(false);
    }
    expect(
      inheritedSessionGithubCli(floor, {
        ancestors: [true],
        rootAutomationId: null,
      }),
    ).toBe(false);
  });
});
