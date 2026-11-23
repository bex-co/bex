import { describe, expect, it } from "vitest";
import { AgentNetworkTier } from "@/graphql/definitions";
import { codedGraphQLError, uncodedGraphQLError } from "@/test/mocks/apollo";
import {
  automationsGithubCliInput,
  buildPatch,
  defaultModelUnoffered,
  draftFrom,
  enabledModelsErrorKey,
  modelFilterOf,
  modelProvider,
  providerModels,
  providersOfEnabledModels,
  githubCliAutomationsErrorKey,
  githubCliErrorKey,
  githubCliMandatoryErrorKey,
  idleTimeoutErrorKey,
  modelKeyErrorKey,
  normalizeModelKey,
  parseHostLines,
  parseIdleTimeout,
  saveErrorFrom,
  validModel,
  validateCustomHosts,
  withModelToggled,
  withProviderToggled,
  type AgentEnvironmentView,
} from "@/features/agent-environment/lib/environment";
import { agentEnvironment } from "@/test/mocks/agent-environment";

const SAVED: AgentEnvironmentView = agentEnvironment({
  networkTier: AgentNetworkTier.Custom,
  customHosts: ["a.example.com", "b.example.com"],
  defaultModel: "claude-sonnet-4-5",
  registryHosts: ["registry.npmjs.org"],
});

describe("parseHostLines", () => {
  it("trims, lowercases and drops blank lines", () => {
    expect(parseHostLines(" A.example.com \n\n\r\nb.example.com\n")).toEqual([
      "a.example.com",
      "b.example.com",
    ]);
  });
});

describe("validateCustomHosts", () => {
  it.each([
    [["https://a.example.com"], "not_a_hostname"],
    [["a.example.com:443"], "not_a_hostname"],
    [["*.example.com"], "not_a_hostname"],
    [["localhost"], "not_a_hostname"],
    [["10.0.0.1"], "ip_literal"],
    [["a.example.com", "a.example.com"], "duplicate"],
    [
      Array.from({ length: 33 }, (_, i) => `h${i}.example.com`),
      "too_many_entries",
    ],
  ])("rejects %j as %s", (hosts, reason) => {
    expect(validateCustomHosts(hosts)?.reason).toBe(reason);
  });

  it("accepts exact multi-label names, up to 32", () => {
    expect(
      validateCustomHosts(
        Array.from({ length: 32 }, (_, i) => `h${i}.example.com`),
      ),
    ).toBeNull();
  });
});

describe("validModel", () => {
  it("accepts empty and provider/OpenRouter ids, rejects whitespace", () => {
    expect(validModel("")).toBe(true);
    expect(validModel("claude-sonnet-4-5[1m]")).toBe(true);
    expect(validModel("deepseek/deepseek-v4.1-flash")).toBe(true);
    expect(validModel("bad model")).toBe(false);
  });
});

describe("buildPatch", () => {
  it("is empty for an untouched draft, and ignores host reordering", () => {
    expect(buildPatch(SAVED, draftFrom(SAVED))).toEqual({});
    expect(
      buildPatch(SAVED, {
        ...draftFrom(SAVED),
        hostsText: "b.example.com\nA.example.com",
      }),
    ).toEqual({});
  });

  it("sends only the changed fields", () => {
    expect(
      buildPatch(SAVED, { ...draftFrom(SAVED), defaultModel: "  " }),
    ).toEqual({ defaultModel: "" });
    expect(
      buildPatch(SAVED, {
        ...draftFrom(SAVED),
        networkTier: AgentNetworkTier.Registries,
        hostsText: "",
      }),
    ).toEqual({ networkTier: AgentNetworkTier.Registries });
  });

  it("sends a catalog default on the platform route (w10/m516)", () => {
    const routed = {
      ...SAVED,
      defaultModel: null,
      models: [
        "deepseek/deepseek-v4.1-flash-1017",
        "anthropic/claude-opus-5.5",
      ],
    };
    expect(buildPatch(routed, draftFrom(routed))).toEqual({});
    expect(
      buildPatch(routed, {
        ...draftFrom(routed),
        defaultModel: "anthropic/claude-opus-5.5",
      }),
    ).toEqual({ defaultModel: "anthropic/claude-opus-5.5" });
  });

  it("sends a changed default sandbox template (w10/m519)", () => {
    const sized = {
      ...SAVED,
      defaultTemplate: "box-small",
      templates: [
        {
          __typename: "AgentSandboxTemplate" as const,
          name: "box-small",
          cpu: 1,
          memoryGb: 2,
        },
        {
          __typename: "AgentSandboxTemplate" as const,
          name: "box-large",
          cpu: 8,
          memoryGb: 16,
        },
      ],
    };
    expect(draftFrom(sized).defaultTemplate).toBe("box-small");
    expect(buildPatch(sized, draftFrom(sized))).toEqual({});
    expect(
      buildPatch(sized, { ...draftFrom(sized), defaultTemplate: "box-large" }),
    ).toEqual({ defaultTemplate: "box-large" });
  });
});

describe("saveErrorFrom", () => {
  it("anchors coded refusals to their field", () => {
    expect(
      saveErrorFrom(
        codedGraphQLError("AGENT_ENVIRONMENT_HOSTS_INVALID", {
          field: "customHosts",
          reason: "private_name",
          entry: "db.internal.example",
        }),
      ),
    ).toEqual({
      field: "hosts",
      problem: { reason: "private_name", entry: "db.internal.example" },
    });
    expect(
      saveErrorFrom(
        codedGraphQLError("AGENT_ENVIRONMENT_MODEL_NOT_OFFERED", {
          field: "defaultModel",
          model: "x/y",
          models: ["a/b"],
        }),
      ),
    ).toEqual({
      field: "model",
      key: "agentEnvironment.modelNotOfferedError",
      params: { model: "x/y" },
    });
    expect(
      saveErrorFrom(
        codedGraphQLError("AGENT_ENVIRONMENT_TEMPLATE_INVALID", {
          field: "defaultTemplate",
          template: "box-huge",
          templates: ["box-small", "box-large"],
        }),
      ),
    ).toEqual({
      field: "template",
      key: "agentEnvironment.templateInvalidError",
      params: { template: "box-huge", templates: "box-small, box-large" },
    });
    expect(saveErrorFrom(codedGraphQLError("FORBIDDEN")).field).toBe("form");
  });

  it("falls back to the server's message, else generic copy", () => {
    expect(saveErrorFrom(uncodedGraphQLError("nope"))).toEqual({
      field: "form",
      key: "agentEnvironment.saveFailedReason",
      params: { reason: "nope" },
    });
    expect(saveErrorFrom(new Error("Failed to fetch"))).toEqual({
      field: "form",
      key: "agentEnvironment.saveFailed",
    });
  });
});

describe("normalizeModelKey", () => {
  it("trims a pasted key and refuses what the server refuses", () => {
    expect(normalizeModelKey("  sk-ant-api03-x\n")).toBe("sk-ant-api03-x");
    expect(normalizeModelKey("")).toBeNull();
    expect(normalizeModelKey(" \t ")).toBeNull();
    expect(normalizeModelKey("sk ant")).toBeNull();
    expect(normalizeModelKey("sk\u0000ant")).toBeNull();
    expect(normalizeModelKey("k".repeat(4096))).toHaveLength(4096);
    expect(normalizeModelKey("k".repeat(4097))).toBeNull();
  });
});

describe("modelKeyErrorKey", () => {
  it("maps by code and never surfaces the server message", () => {
    expect(
      modelKeyErrorKey(
        codedGraphQLError("AGENT_ENVIRONMENT_MODEL_KEY_INVALID"),
      ),
    ).toBe("agentEnvironment.modelKeyInvalid");
    expect(modelKeyErrorKey(codedGraphQLError("FORBIDDEN"))).toBe(
      "agentEnvironment.forbidden",
    );
    expect(modelKeyErrorKey(uncodedGraphQLError("boom"))).toBe(
      "agentEnvironment.modelKeySaveFailed",
    );
  });
});

describe("githubCliErrorKey", () => {
  it("names the admin-only refusal and offers a retry for anything else", () => {
    expect(githubCliErrorKey(codedGraphQLError("FORBIDDEN"))).toBe(
      "agentEnvironment.forbidden",
    );
    expect(githubCliErrorKey(uncodedGraphQLError("boom"))).toBe(
      "agentEnvironment.githubCliSaveFailed",
    );
    expect(githubCliErrorKey(new Error("offline"))).toBe(
      "agentEnvironment.githubCliSaveFailed",
    );
  });
});

// w10/m527: the catalog models members may choose.
describe("the model list", () => {
  const CATALOG = [
    "deepseek/deepseek-v4.1-flash-1017",
    "anthropic/claude-haiku-5.5",
    "anthropic/claude-sonnet-5.5",
    "anthropic/claude-opus-5.5",
  ];
  const routed = (
    enabledModels: string[],
    defaultModel: string | null = null,
  ) =>
    // An allowlist (w10/m568): what the switches by model save.
    agentEnvironment({
      models: CATALOG,
      allowAllModels: false,
      enabledModels,
      defaultModel,
    });

  it("sends the whole list in catalog order with the catalog default kept on", () => {
    const all = routed(CATALOG);
    expect(withModelToggled(all, "anthropic/claude-opus-5.5", false)).toEqual(
      CATALOG.slice(0, 3),
    );
    const some = routed([CATALOG[0], CATALOG[3]]);
    // Turning one back on puts it in its catalog slot, not at the end.
    expect(withModelToggled(some, CATALOG[1], true)).toEqual([
      CATALOG[0],
      CATALOG[1],
      CATALOG[3],
    ]);
    // The catalog default can never leave the list.
    expect(withModelToggled(all, CATALOG[0], false)).toEqual(CATALOG);
    expect(withModelToggled(routed([]), CATALOG[2], true)).toEqual([
      CATALOG[0],
      CATALOG[2],
    ]);
  });

  it("says why a saved default does not take effect", () => {
    const env = routed([CATALOG[0], CATALOG[1]]);
    expect(defaultModelUnoffered(env, "")).toBeNull();
    expect(defaultModelUnoffered(env, CATALOG[1])).toBeNull();
    expect(defaultModelUnoffered(env, CATALOG[3])).toBe("off");
    expect(defaultModelUnoffered(env, "anthropic/claude-opus-4.1")).toBe(
      "retired",
    );
    // On the workspace's own key any model id stands.
    expect(
      defaultModelUnoffered(agentEnvironment(), "claude-sonnet-4-5"),
    ).toBeNull();
  });

  it("names the list refusal and the admin-only refusal, and offers a retry for anything else", () => {
    expect(
      enabledModelsErrorKey(
        codedGraphQLError("AGENT_ENVIRONMENT_MODELS_INVALID", {
          field: "enabledModels",
          model: CATALOG[0],
        }),
      ),
    ).toBe("agentEnvironment.modelsInvalid");
    expect(enabledModelsErrorKey(codedGraphQLError("FORBIDDEN"))).toBe(
      "agentEnvironment.forbidden",
    );
    expect(enabledModelsErrorKey(uncodedGraphQLError("boom"))).toBe(
      "agentEnvironment.modelsSaveFailed",
    );
    expect(enabledModelsErrorKey(new Error("offline"))).toBe(
      "agentEnvironment.modelsSaveFailed",
    );
    // w10/m573: the same code on the provider filter names the providers.
    expect(
      enabledModelsErrorKey(
        codedGraphQLError("AGENT_ENVIRONMENT_MODELS_INVALID", {
          field: "enabledProviders",
          provider: "openai",
        }),
      ),
    ).toBe("agentEnvironment.modelsProvidersInvalid");
    // w10/m592: on a switch to or from "All models", the model source.
    expect(
      enabledModelsErrorKey(
        codedGraphQLError("AGENT_ENVIRONMENT_MODELS_INVALID", {
          field: "allowAllModels",
        }),
      ),
    ).toBe("agentEnvironment.modelsAllowAllInvalid");
  });
});

// w10/m573: the catalog filtered by provider, Devin's other filter type.
describe("the filter by provider", () => {
  const CATALOG = [
    "deepseek/deepseek-v4.1-flash-1017",
    "anthropic/claude-sonnet-5.5",
    "anthropic/claude-opus-5.5",
    "moonshotai/kimi-k3",
  ];
  const PROVIDERS = ["deepseek", "anthropic", "moonshotai"];
  const env = (
    enabledModels: string[],
    enabledProviders: string[] | null = null,
  ) =>
    agentEnvironment({
      models: CATALOG,
      allowAllModels: false,
      enabledModels,
      providers: PROVIDERS,
      enabledProviders,
    });

  it("takes a model's provider from its id's prefix before the slash", () => {
    expect(modelProvider("anthropic/claude-opus-5.5")).toBe("anthropic");
    expect(modelProvider("~deepseek/deepseek-flash-latest")).toBe("~deepseek");
    expect(modelProvider("claude-opus-4-1")).toBe("");
    expect(providerModels(env(CATALOG), "anthropic")).toEqual(
      CATALOG.slice(1, 3),
    );
  });

  it("reads the mode in force from the environment", () => {
    // Devin's "Allow all models" (w10/m592), every workspace's default.
    expect(modelFilterOf({ ...env(CATALOG), allowAllModels: true })).toEqual({
      mode: "all",
      models: CATALOG,
      providers: [],
    });
    // An allowlist, even one listing every model.
    for (const listed of [CATALOG.slice(0, 2), CATALOG]) {
      expect(modelFilterOf(env(listed))).toEqual({
        mode: "model",
        models: listed,
        providers: [],
      });
    }
    expect(modelFilterOf(env(CATALOG.slice(0, 3), ["anthropic"]))).toEqual({
      mode: "provider",
      models: CATALOG.slice(0, 3),
      providers: ["anthropic"],
    });
  });

  it("sends the whole provider list in catalog order", () => {
    const saved = env(CATALOG.slice(0, 3), ["anthropic"]);
    expect(withProviderToggled(saved, "moonshotai", true)).toEqual([
      "anthropic",
      "moonshotai",
    ]);
    expect(withProviderToggled(saved, "deepseek", true)).toEqual([
      "deepseek",
      "anthropic",
    ]);
    expect(withProviderToggled(saved, "anthropic", false)).toEqual([]);
  });

  it("switches to the filter by provider with the providers of the models on now", () => {
    // Sonnet and the default on: anthropic and deepseek, not moonshotai.
    expect(providersOfEnabledModels(env([CATALOG[0], CATALOG[1]]))).toEqual([
      "deepseek",
      "anthropic",
    ]);
    expect(providersOfEnabledModels(env(CATALOG))).toEqual(PROVIDERS);
    // No model on has a provider: every provider.
    expect(
      providersOfEnabledModels(
        agentEnvironment({
          models: ["claude-opus-4-1", ...CATALOG.slice(1)],
          enabledModels: ["claude-opus-4-1"],
          providers: ["anthropic", "moonshotai"],
        }),
      ),
    ).toEqual(["anthropic", "moonshotai"]);
  });
});

// w10/m561: the automations default and the mandatory flag beside the
// workspace's switch.
describe("the GitHub CLI section's other writes", () => {
  it("sends inherit as the reset flag, on and off as the binding", () => {
    expect(automationsGithubCliInput("inherit")).toEqual({
      resetAutomationsGithubCli: true,
    });
    expect(automationsGithubCliInput("on")).toEqual({
      automationsGithubCliEnabled: true,
    });
    expect(automationsGithubCliInput("off")).toEqual({
      automationsGithubCliEnabled: false,
    });
  });

  it("names the mandatory lock, the admin-only refusal, and offers a retry", () => {
    expect(
      githubCliAutomationsErrorKey(
        codedGraphQLError("AGENT_ENVIRONMENT_GITHUB_CLI_LOCKED", {
          field: "automationsGithubCliEnabled",
          reason: "mandatory",
        }),
      ),
    ).toBe("agentEnvironment.githubCliAutomationsLocked");
    expect(githubCliAutomationsErrorKey(codedGraphQLError("FORBIDDEN"))).toBe(
      "agentEnvironment.forbidden",
    );
    expect(githubCliAutomationsErrorKey(new Error("offline"))).toBe(
      "agentEnvironment.githubCliAutomationsSaveFailed",
    );
    expect(githubCliMandatoryErrorKey(codedGraphQLError("FORBIDDEN"))).toBe(
      "agentEnvironment.forbidden",
    );
    expect(githubCliMandatoryErrorKey(uncodedGraphQLError("boom"))).toBe(
      "agentEnvironment.githubCliMandatorySaveFailed",
    );
  });
});

// w6/m563: the Enterprise idle timeout is whole minutes from 5 to 120.
describe("parseIdleTimeout", () => {
  it.each([
    ["5", 5],
    [" 30 ", 30],
    ["120", 120],
    ["4", null],
    ["121", null],
    ["7.5", null],
    ["-10", null],
    ["", null],
    ["1e2", null],
  ])("%j is %j", (text, want) => {
    expect(parseIdleTimeout(text)).toBe(want);
  });

  it("maps each refusal to its message", () => {
    expect(
      idleTimeoutErrorKey(
        codedGraphQLError("AGENT_ENVIRONMENT_IDLE_TIMEOUT_INVALID"),
      ),
    ).toBe("agentEnvironment.idleTimeoutInvalid");
    expect(
      idleTimeoutErrorKey(codedGraphQLError("PLAN_FEATURE_REQUIRED")),
    ).toBe("agentEnvironment.idleTimeoutEnterpriseOnly");
    expect(idleTimeoutErrorKey(codedGraphQLError("FORBIDDEN"))).toBe(
      "agentEnvironment.forbidden",
    );
    expect(idleTimeoutErrorKey(uncodedGraphQLError("boom"))).toBe(
      "agentEnvironment.idleTimeoutSaveFailed",
    );
  });
});
