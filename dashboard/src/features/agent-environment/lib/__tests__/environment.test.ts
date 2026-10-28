import { describe, expect, it } from "vitest";
import { AgentNetworkTier } from "@/graphql/definitions";
import { codedGraphQLError, uncodedGraphQLError } from "@/test/mocks/apollo";
import {
  buildPatch,
  draftFrom,
  parseHostLines,
  saveErrorFrom,
  validModel,
  validateCustomHosts,
  type AgentEnvironmentView,
} from "@/features/agent-environment/lib/environment";

const SAVED: AgentEnvironmentView = {
  __typename: "AgentEnvironment",
  ownerId: "tea-1",
  networkTier: AgentNetworkTier.Custom,
  customHosts: ["a.example.com", "b.example.com"],
  defaultModel: "claude-sonnet-4-5",
  modelPinned: false,
  pinnedModel: null,
  registryHosts: ["registry.npmjs.org"],
  updatedAt: null,
};

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

  it("never sends the model while it is pinned", () => {
    const pinned = { ...SAVED, modelPinned: true, pinnedModel: "x/y" };
    expect(
      buildPatch(pinned, { ...draftFrom(pinned), defaultModel: "z" }),
    ).toEqual({});
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
        codedGraphQLError("AGENT_ENVIRONMENT_MODEL_PINNED", {
          pinnedModel: "x/y",
        }),
      ),
    ).toEqual({
      field: "model",
      key: "agentEnvironment.modelPinnedError",
      params: { model: "x/y" },
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
