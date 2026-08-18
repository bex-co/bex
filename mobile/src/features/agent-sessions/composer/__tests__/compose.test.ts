import {
  buildCreateVariables,
  canSubmit,
  deriveBranch,
  isBranchInNamespace,
  repositoryDisplayName,
  resolveWorkingBranch,
} from "../compose";

const full = {
  repo: "org/app",
  branch: "",
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

  it("requires a repo, prompt, and agent, ignoring whitespace-only input", () => {
    for (const missing of ["repo", "prompt", "agent"] as const) {
      const fields = { ...full, [missing]: "   " };
      expect(canSubmit({ fields, ready: true, submitting: false })).toBe(false);
    }
  });

  it("does not require a branch: it derives from the prompt", () => {
    // An empty branch still submits because the working branch is derived.
    expect(
      canSubmit({
        fields: { ...full, branch: "" },
        ready: true,
        submitting: false,
      }),
    ).toBe(true);
  });

  it("builds a secret-free create payload with a bex-agent/ work branch", () => {
    const vars = buildCreateVariables("tea-1", {
      repo: " org/app ",
      branch: "",
      prompt: " Fix the flaky test ",
      agent: "claude",
    });
    expect(vars).toEqual({
      ownerId: "tea-1",
      repo: "org/app",
      branch: "bex-agent/fix-the-flaky-test",
      agentConfig: { agent: "claude", task: "Fix the flaky test" },
    });
    // The payload must not carry any endpoint/template/egress surface.
    const keys = Object.keys(vars.agentConfig);
    expect(keys.sort()).toEqual(["agent", "task"]);
  });

  it("resolves the work branch into the mandated namespace", () => {
    // Empty override ⇒ derive from the prompt.
    expect(resolveWorkingBranch({ ...full, branch: "" })).toBe(
      "bex-agent/fix-it",
    );
    // A base-branch-shaped entry ("main") is slugged into the namespace, never
    // sent verbatim (the backend rejects branches outside bex-agent/).
    expect(resolveWorkingBranch({ ...full, branch: "main" })).toBe(
      "bex-agent/main",
    );
    // An in-namespace override is kept verbatim.
    expect(resolveWorkingBranch({ ...full, branch: "bex-agent/my-fix" })).toBe(
      "bex-agent/my-fix",
    );
    // Unsluggable override falls back to the prompt-derived branch.
    expect(resolveWorkingBranch({ ...full, branch: "///" })).toBe(
      "bex-agent/fix-it",
    );
  });

  it("derives a namespaced branch even from an empty task", () => {
    expect(deriveBranch("")).toBe("bex-agent/session");
    expect(isBranchInNamespace(deriveBranch(""))).toBe(true);
    expect(isBranchInNamespace("main")).toBe(false);
    expect(isBranchInNamespace("bex-agent/")).toBe(false);
  });

  it("uses the repository leaf name in the compact composer chip", () => {
    expect(repositoryDisplayName("bex-co/bex")).toBe("bex");
    expect(repositoryDisplayName("standalone")).toBe("standalone");
  });
});
