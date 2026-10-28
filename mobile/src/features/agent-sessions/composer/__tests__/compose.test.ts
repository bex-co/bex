import { buildCreateVariables, canSubmit, deriveBranch } from "../compose";

const full = {
  repo: "org/app",
  prompt: "fix it",
  agent: "claude",
};

describe("agent-session composer", () => {
  it("blocks submit until the workspace is provisioned (ready)", () => {
    expect(canSubmit({ fields: full, ready: false, submitting: false })).toBe(
      false,
    );
    expect(canSubmit({ fields: full, ready: true, submitting: false })).toBe(
      true,
    );
  });

  it("blocks submit while one is already in flight (idempotency guard)", () => {
    expect(canSubmit({ fields: full, ready: true, submitting: true })).toBe(
      false,
    );
  });

  it("requires every field, ignoring whitespace-only input", () => {
    for (const missing of ["repo", "prompt", "agent"] as const) {
      const fields = { ...full, [missing]: "   " };
      expect(canSubmit({ fields, ready: true, submitting: false })).toBe(false);
    }
  });

  it("builds a secret-free create payload: only agent id + prompt", () => {
    const vars = buildCreateVariables("tea-1", {
      repo: " org/app ",
      prompt: " fix it ",
      agent: "claude",
    });
    expect(vars).toEqual({
      ownerId: "tea-1",
      repo: "org/app",
      branch: "bex-agent/fix-it",
      agentConfig: { agent: "claude", task: "fix it" },
    });
    // The payload must not carry any endpoint/template/egress surface.
    const keys = Object.keys(vars.agentConfig);
    expect(keys.sort()).toEqual(["agent", "task"]);
  });

  it("derives the bex-agent/* branch from the task, never the repo default", () => {
    expect(deriveBranch("Fix the Login bug!")).toBe(
      "bex-agent/fix-the-login-bug",
    );
    expect(deriveBranch("x".repeat(60))).toBe("bex-agent/" + "x".repeat(40));
    // A task with no ASCII slug (e.g. Chinese) still yields a valid branch.
    expect(deriveBranch("修复登录问题", 1700000000000)).toBe(
      "bex-agent/session-" + (1700000000000).toString(36),
    );
    expect(deriveBranch("修复登录问题").startsWith("bex-agent/session-")).toBe(
      true,
    );
  });
});
