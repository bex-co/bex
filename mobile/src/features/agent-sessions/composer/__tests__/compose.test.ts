import {
  buildCreateVariables,
  canSubmit,
  deriveBranch,
  modelControl,
  sentModel,
  shortModel,
} from "../compose";

const full = {
  repo: "org/app",
  prompt: "fix it",
  agent: "claude",
  model: "",
};

// The capabilities read's offer for the claude agent on the platform route:
// the catalog, its default first (w10/m516).
const catalog = {
  model: "deepseek/deepseek-v4.1-flash-1017",
  models: [
    "deepseek/deepseek-v4.1-flash-1017",
    "anthropic/claude-haiku-5.5",
    "anthropic/claude-sonnet-5.5",
  ],
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
      model: "",
    });
    expect(vars).toEqual({
      ownerId: "tea-1",
      repo: "org/app",
      branch: "bex-agent/fix-it",
      agentConfig: { agent: "claude", task: "fix it" },
    });
    // The payload must not carry any endpoint/template/egress surface, and
    // names no model when none was chosen (the agent's default runs).
    const keys = Object.keys(vars.agentConfig);
    expect(keys.sort()).toEqual(["agent", "task"]);
  });

  it("starts the session on a chosen model (agentConfig.model)", () => {
    const vars = buildCreateVariables("tea-1", {
      ...full,
      model: "anthropic/claude-sonnet-5.5",
    });
    expect(vars.agentConfig).toEqual({
      agent: "claude",
      task: "fix it",
      model: "anthropic/claude-sonnet-5.5",
    });
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

  it("offers no model control on the workspace's own key", () => {
    expect(modelControl(undefined, "")).toEqual({ kind: "none" });
    expect(modelControl({ model: null, models: [] }, "")).toEqual({
      kind: "none",
    });
    // A default from Settings → Agents still draws nothing: any model runs.
    expect(modelControl({ model: "gpt-5", models: [] }, "")).toEqual({
      kind: "none",
    });
  });

  it("shows a fixed-model agent's one model without a choice", () => {
    const offer = {
      model: "deepseek/deepseek-v4.1-flash-1017",
      models: ["deepseek/deepseek-v4.1-flash-1017"],
    };
    expect(modelControl(offer, "")).toEqual({
      kind: "fixed",
      model: "deepseek/deepseek-v4.1-flash-1017",
    });
  });

  it("shows the default until another catalog model is chosen", () => {
    expect(modelControl(catalog, "")).toEqual({
      kind: "pick",
      model: catalog.model,
      default: catalog.model,
      models: catalog.models,
    });
    expect(modelControl(catalog, "anthropic/claude-haiku-5.5")).toEqual({
      kind: "pick",
      model: "anthropic/claude-haiku-5.5",
      default: catalog.model,
      models: catalog.models,
    });
  });

  it("falls back to the default when the chosen model is no longer offered", () => {
    // An admin turned it off (w10/m527) while the composer was open: the
    // refreshed offer drops it, so it is neither shown nor sent.
    const control = modelControl(catalog, "anthropic/claude-opus-5.5");
    expect(control).toEqual({
      kind: "pick",
      model: catalog.model,
      default: catalog.model,
      models: catalog.models,
    });
    expect(sentModel(control)).toBe("");
  });

  it("names a model in the create only when another one was picked", () => {
    expect(sentModel(modelControl(catalog, ""))).toBe("");
    // Picking the default back sends nothing, as the dashboard composer does.
    expect(sentModel(modelControl(catalog, catalog.model))).toBe("");
    expect(
      sentModel(modelControl(catalog, "anthropic/claude-sonnet-5.5")),
    ).toBe("anthropic/claude-sonnet-5.5");
    // A fixed or absent control never names one.
    const fixed = { model: "x/y", models: ["x/y"] };
    expect(sentModel(modelControl(fixed, "x/y"))).toBe("");
    expect(sentModel(modelControl(undefined, "x/y"))).toBe("");
  });

  it("shortens a model id to its name for the pill", () => {
    expect(shortModel("anthropic/claude-sonnet-5.5")).toBe("claude-sonnet-5.5");
    expect(shortModel("deepseek/deepseek-v4.1-flash-1017")).toBe(
      "deepseek-v4.1-flash-1017",
    );
    expect(shortModel("gpt-5")).toBe("gpt-5");
    expect(shortModel("openrouter/~anthropic-latest")).toBe("anthropic-latest");
  });
});
