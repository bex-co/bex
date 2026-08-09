import { buildCreateVariables, canSubmit, defaultBranchFor } from "../compose";

const full = {
  repo: "org/app",
  branch: "main",
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
    for (const missing of ["repo", "branch", "prompt", "agent"] as const) {
      const fields = { ...full, [missing]: "   " };
      expect(canSubmit({ fields, ready: true, submitting: false })).toBe(false);
    }
  });

  it("builds a secret-free create payload: only agent id + prompt", () => {
    const vars = buildCreateVariables("tea-1", {
      repo: " org/app ",
      branch: " main ",
      prompt: " fix it ",
      agent: "claude",
    });
    expect(vars).toEqual({
      ownerId: "tea-1",
      repo: "org/app",
      branch: "main",
      agentConfig: { agent: "claude", task: "fix it" },
    });
    // The payload must not carry any endpoint/template/egress surface.
    const keys = Object.keys(vars.agentConfig);
    expect(keys.sort()).toEqual(["agent", "task"]);
  });

  it("defaults a missing branch to main", () => {
    expect(defaultBranchFor(null)).toBe("main");
    expect(defaultBranchFor("  ")).toBe("main");
    expect(defaultBranchFor("develop")).toBe("develop");
  });
});
