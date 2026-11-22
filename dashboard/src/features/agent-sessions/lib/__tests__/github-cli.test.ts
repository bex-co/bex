import { describe, expect, it } from "vitest";
import { agentEnvironment } from "@/test/mocks/agent-environment";
import {
  githubCliBinding,
  githubCliChoice,
  githubCliMandatoryOff,
  githubCliOnLock,
  inheritedGithubCli,
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

  it("locks on under the floor for everyone and for a member always, unless kept", () => {
    const floor = agentEnvironment({
      githubCliEnabled: false,
      githubCliMandatory: true,
    });
    const open = agentEnvironment();
    expect(githubCliOnLock({ env: open, canManage: true })).toBeNull();
    expect(githubCliOnLock({ env: open, canManage: false })).toBe("admin");
    expect(githubCliOnLock({ env: floor, canManage: true })).toBe("mandatory");
    expect(githubCliOnLock({ env: floor, canManage: false })).toBe("mandatory");
    expect(githubCliOnLock({ env: null, canManage: true })).toBeNull();
    // An automation that already stores on keeps it, as the server does.
    expect(
      githubCliOnLock({ env: floor, canManage: false, kept: true }),
    ).toBeNull();
  });
});
