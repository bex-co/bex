import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import type { SettingsFlow } from "@ory/client-fetch";
import { KRATOS_PUBLIC_URL } from "@/common/lib/ory/config";
import { settingsTransientPayload } from "@/features/auth/lib/settings-transient-payload";
import { AccountSettingsFlow } from "../account-settings-flow";

// w7/085: Render refuses only disconnecting Google while a member belongs to a
// workspace that requires Google login. bex-api's settings guard learns which
// provider an unlink removes only from the settings flow's transient payload,
// so the account settings page must send it with every unlink.

const FLOW_ID = "settings-1";
const SUBMIT = `${KRATOS_PUBLIC_URL}/self-service/settings?flow=${FLOW_ID}`;

function input(
  group: string,
  name: string,
  type: string,
  value: string,
  label?: string,
) {
  return {
    type: "input",
    group,
    attributes: { name, type, value, node_type: "input", disabled: false },
    messages: [],
    meta: label
      ? {
          label: {
            id: name === "unlink" ? 1050003 : 1050002,
            type: "info",
            text: `${name === "unlink" ? "Unlink" : "Link"} ${label}`,
            context: { provider: label, provider_id: value },
          },
        }
      : {},
  };
}

/** A settings flow as Kratos v26.2.0 sends it to a member with GitHub and
 *  Google linked: the profile form and the connected accounts. */
function settingsFlow(): SettingsFlow {
  return {
    id: FLOW_ID,
    type: "browser",
    state: "show_form",
    expires_at: new Date("2030-01-01T00:00:00Z"),
    issued_at: new Date("2026-11-17T00:00:00Z"),
    request_url: `${KRATOS_PUBLIC_URL}/self-service/settings/browser`,
    identity: {
      id: "identity-1",
      schema_id: "default",
      schema_url: "",
      traits: { email: "person@acme.example" },
    },
    ui: {
      action: SUBMIT,
      method: "POST",
      nodes: [
        input("default", "csrf_token", "hidden", "csrf-1"),
        input("profile", "traits.email", "email", "person@acme.example"),
        input("profile", "method", "submit", "profile"),
        input("oidc", "link", "submit", "gitlab", "GitLab"),
        input("oidc", "unlink", "submit", "github", "GitHub"),
        input("oidc", "unlink", "submit", "google", "Google"),
      ],
    },
  } as unknown as SettingsFlow;
}

/** The JSON bodies the page POSTs to the settings flow. */
let submits: Record<string, unknown>[] = [];

beforeEach(() => {
  submits = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = input instanceof Request ? input.url : String(input);
      if (url === SUBMIT && init?.method === "POST") {
        submits.push(JSON.parse(String(init.body)));
        return new Response(JSON.stringify(settingsFlow()), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        });
      }
      // The session Ory's SessionProvider reads; nothing here needs it.
      return new Response("{}", {
        status: 401,
        headers: { "Content-Type": "application/json" },
      });
    }),
  );
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("the account settings flow names the provider an unlink removes", () => {
  it.each(["github", "google"])(
    "unlinking %s sends it in the flow's transient payload",
    async (provider) => {
      render(<AccountSettingsFlow flow={settingsFlow()} />);

      const label = provider === "github" ? "GitHub" : "Google";
      fireEvent.click(
        await screen.findByRole("button", { name: `Unlink ${label}` }),
      );

      await waitFor(() => expect(submits).toHaveLength(1));
      expect(submits[0]).toMatchObject({
        method: "oidc",
        unlink: provider,
        transient_payload: { unlink: provider },
      });
    },
  );
});

describe("settingsTransientPayload", () => {
  it("names the provider of an oidc unlink", () => {
    expect(
      settingsTransientPayload({ method: "oidc", unlink: "github" }),
    ).toEqual({ unlink: "github" });
  });

  it.each([
    ["a link", { method: "oidc", link: "gitlab" }],
    ["a profile change", { method: "profile", "traits.email": "x@y.z" }],
    ["an empty unlink", { method: "oidc", unlink: "" }],
    ["an unlink under another method", { method: "totp", unlink: "github" }],
  ])("sends nothing for %s", (_, values) => {
    expect(settingsTransientPayload(values)).toEqual({});
  });
});
