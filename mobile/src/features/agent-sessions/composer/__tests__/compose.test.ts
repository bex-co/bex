import {
  buildCreateVariables,
  canSubmit,
  deriveBranch,
  modelControl,
  sentChoice,
  shortModel,
  templateControl,
  templateLabel,
} from "../compose";

const full = {
  repo: "org/app",
  prompt: "fix it",
  agent: "claude",
  model: "",
  template: "",
};

// The capabilities read's sandbox offer (w10/m519): its templates smallest
// first, the workspace default among them.
const sandbox = {
  defaultTemplate: "agent",
  templates: [
    { name: "agent", cpu: 2, memoryGb: 4 },
    { name: "agent-4c-8g", cpu: 4, memoryGb: 8 },
  ],
};
const names = ["agent", "agent-4c-8g"];

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
      template: "",
    });
    expect(vars).toEqual({
      ownerId: "tea-1",
      repo: "org/app",
      branch: "bex-agent/fix-it",
      agentConfig: { agent: "claude", task: "fix it" },
    });
    // The payload must not carry any endpoint or egress surface, and names no
    // model or template when none was chosen (the agent's default model runs
    // on the workspace's default template).
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

  it("starts the session on a chosen sandbox template (agentConfig.template)", () => {
    const vars = buildCreateVariables("tea-1", {
      ...full,
      model: "anthropic/claude-sonnet-5.5",
      template: "agent-4c-8g",
    });
    expect(vars.agentConfig).toEqual({
      agent: "claude",
      task: "fix it",
      model: "anthropic/claude-sonnet-5.5",
      template: "agent-4c-8g",
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
      value: "deepseek/deepseek-v4.1-flash-1017",
    });
  });

  it("shows the default until another catalog model is chosen", () => {
    expect(modelControl(catalog, "")).toEqual({
      kind: "pick",
      value: catalog.model,
      default: catalog.model,
      options: catalog.models,
    });
    expect(modelControl(catalog, "anthropic/claude-haiku-5.5")).toEqual({
      kind: "pick",
      value: "anthropic/claude-haiku-5.5",
      default: catalog.model,
      options: catalog.models,
    });
  });

  it("falls back to the default when the chosen model is no longer offered", () => {
    // An admin turned it off (w10/m527) while the composer was open: the
    // refreshed offer drops it, so it is neither shown nor sent.
    const control = modelControl(catalog, "anthropic/claude-opus-5.5");
    expect(control).toEqual({
      kind: "pick",
      value: catalog.model,
      default: catalog.model,
      options: catalog.models,
    });
    expect(sentChoice(control)).toBe("");
  });

  it("names a model in the create only when another one was picked", () => {
    expect(sentChoice(modelControl(catalog, ""))).toBe("");
    // Picking the default back sends nothing, as the dashboard composer does.
    expect(sentChoice(modelControl(catalog, catalog.model))).toBe("");
    expect(
      sentChoice(modelControl(catalog, "anthropic/claude-sonnet-5.5")),
    ).toBe("anthropic/claude-sonnet-5.5");
    // A fixed or absent control never names one.
    const fixed = { model: "x/y", models: ["x/y"] };
    expect(sentChoice(modelControl(fixed, "x/y"))).toBe("");
    expect(sentChoice(modelControl(undefined, "x/y"))).toBe("");
  });

  it("shortens a model id to its name for the pill", () => {
    expect(shortModel("anthropic/claude-sonnet-5.5")).toBe("claude-sonnet-5.5");
    expect(shortModel("deepseek/deepseek-v4.1-flash-1017")).toBe(
      "deepseek-v4.1-flash-1017",
    );
    expect(shortModel("gpt-5")).toBe("gpt-5");
    expect(shortModel("openrouter/~anthropic-latest")).toBe("anthropic-latest");
  });
  it("offers no sandbox control when the read lists no template", () => {
    expect(templateControl(undefined, "")).toEqual({ kind: "none" });
    expect(
      templateControl({ defaultTemplate: "agent", templates: [] }, ""),
    ).toEqual({ kind: "none" });
  });

  it("shows a workspace's only template without a choice", () => {
    const offer = {
      defaultTemplate: "agent",
      templates: [{ name: "agent", cpu: 2, memoryGb: 4 }],
    };
    const control = templateControl(offer, "");
    expect(control).toEqual({ kind: "fixed", value: "agent" });
    expect(sentChoice(control)).toBe("");
  });

  it("shows the workspace default until another template is chosen", () => {
    expect(templateControl(sandbox, "")).toEqual({
      kind: "pick",
      value: "agent",
      default: "agent",
      options: names,
    });
    expect(templateControl(sandbox, "agent-4c-8g")).toEqual({
      kind: "pick",
      value: "agent-4c-8g",
      default: "agent",
      options: names,
    });
    // A workspace whose admin made the larger one the default (w10/m519)
    // starts there, and the smaller one is the pick.
    const larger = { ...sandbox, defaultTemplate: "agent-4c-8g" };
    expect(templateControl(larger, "")).toEqual({
      kind: "pick",
      value: "agent-4c-8g",
      default: "agent-4c-8g",
      options: names,
    });
    expect(sentChoice(templateControl(larger, "agent"))).toBe("agent");
  });

  it("falls back to the default when the chosen template is no longer listed", () => {
    const control = templateControl(sandbox, "agent-8c-16g");
    expect(control).toEqual({
      kind: "pick",
      value: "agent",
      default: "agent",
      options: names,
    });
    expect(sentChoice(control)).toBe("");
  });

  it("names a template in the create only when another one was picked", () => {
    expect(sentChoice(templateControl(sandbox, ""))).toBe("");
    // Picking the default back sends nothing, as the dashboard composer does.
    expect(sentChoice(templateControl(sandbox, "agent"))).toBe("");
    expect(sentChoice(templateControl(sandbox, "agent-4c-8g"))).toBe(
      "agent-4c-8g",
    );
    expect(sentChoice(templateControl(undefined, "agent-4c-8g"))).toBe("");
  });

  it("labels a template with its size from the read, in the app's language", () => {
    const label = (parts: { name: string; cpu: string; memory: string }) =>
      `${parts.name} · ${parts.cpu} CPU, ${parts.memory} GB`;
    expect(templateLabel("agent-4c-8g", sandbox.templates, "en", label)).toBe(
      "agent-4c-8g · 4 CPU, 8 GB",
    );
    const half = [{ name: "agent-tiny", cpu: 0.5, memoryGb: 1.5 }];
    expect(templateLabel("agent-tiny", half, "en", label)).toBe(
      "agent-tiny · 0.5 CPU, 1.5 GB",
    );
    expect(templateLabel("agent-tiny", half, "de", label)).toBe(
      "agent-tiny · 0,5 CPU, 1,5 GB",
    );
    // A name the read does not list is shown as it is, never sized.
    expect(templateLabel("agent-8c-16g", sandbox.templates, "en", label)).toBe(
      "agent-8c-16g",
    );
  });
});
