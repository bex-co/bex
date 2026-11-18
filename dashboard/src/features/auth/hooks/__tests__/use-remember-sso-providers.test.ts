import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { renderHook } from "@testing-library/react";
import type { LoginFlow, RegistrationFlow, UiNode } from "@ory/client-fetch";
import { memoryStorage } from "@/test/mocks/storage";
import {
  rememberedSsoProviders,
  resetSsoProvidersCache,
} from "@/common/lib/ory/sso-providers";
import { useRememberSsoProviders } from "../use-remember-sso-providers";

function providerNode(provider: string): UiNode {
  return {
    type: "input",
    group: "oidc",
    attributes: {
      name: "provider",
      value: provider,
      type: "submit",
      node_type: "input",
    },
    messages: [],
    meta: {},
  } as unknown as UiNode;
}

function flow(
  providers: string[],
  extra: Partial<LoginFlow> = {},
): LoginFlow | RegistrationFlow {
  return {
    id: "flow-1",
    ui: { action: "", method: "POST", nodes: providers.map(providerNode) },
    ...extra,
  } as LoginFlow;
}

beforeEach(() => {
  vi.stubGlobal("localStorage", memoryStorage());
});

afterEach(() => {
  resetSsoProvidersCache();
  vi.unstubAllGlobals();
});

describe("useRememberSsoProviders", () => {
  it("keeps the providers the page's flow offers once it arrives", () => {
    const { rerender } = renderHook(({ f }) => useRememberSsoProviders(f), {
      initialProps: { f: null as LoginFlow | RegistrationFlow | null },
    });
    expect(rememberedSsoProviders()).toBeNull();

    rerender({ f: flow(["bitbucket", "github", "gitlab", "google"]) });
    expect(rememberedSsoProviders()).toEqual([
      "bitbucket",
      "github",
      "gitlab",
      "google",
    ]);
  });

  it("keeps an installation's having no providers", () => {
    renderHook(() => useRememberSsoProviders(flow([])));
    expect(rememberedSsoProviders()).toEqual([]);
  });

  it.each([
    ["a re-authentication", { refresh: true }],
    ["a second-factor", { requested_aal: "aal2" as const }],
  ])(
    "does not read %s login flow, which lists only what it asks for",
    (_, extra) => {
      renderHook(() => useRememberSsoProviders(flow(["github"])));
      renderHook(() => useRememberSsoProviders(flow([], extra)));
      expect(rememberedSsoProviders()).toEqual(["github"]);
    },
  );
});
