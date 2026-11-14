import { ApolloClient, InMemoryCache, gql } from "@apollo/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { createApolloCsrClient } from "@/common/apollo/factory.client";
import { apolloCacheConfig } from "@/common/apollo/cache";
import { apolloDefaultOptions } from "@/common/apollo/default-options";
import {
  isWorkspaceTwoFactorBlocked,
  clearWorkspaceTwoFactorBlocks,
} from "@/features/workspaces/lib/two-factor-block";

const { mockFetch } = vi.hoisted(() => ({ mockFetch: vi.fn<typeof fetch>() }));

vi.mock("@/common/apollo/auth-redirect", () => ({
  handleUnauthenticated: vi.fn(),
  handleEmailVerificationRequired: vi.fn(),
}));

const SERVICES = gql`
  query Services {
    services {
      id
    }
  }
`;

let client: ApolloClient;

/** bex-api answers GraphQL failures 200 with `errors`; a batch answers an array. */
function answer(body: unknown, status = 200) {
  mockFetch.mockImplementation(async (_input, options) => {
    const request: unknown = JSON.parse(String(options?.body));
    const payload = Array.isArray(request) ? [body] : body;
    return new Response(JSON.stringify(payload), {
      status,
      headers: { "content-type": "application/json" },
    });
  });
}

async function query() {
  return client.query({
    query: SERVICES,
    fetchPolicy: "network-only",
    errorPolicy: "all",
  });
}

beforeEach(() => {
  vi.stubGlobal("fetch", mockFetch);
  mockFetch.mockReset();
  clearWorkspaceTwoFactorBlocks();
  // The production transport and link chain, with a fresh cache and query
  // manager per test: the wiring under test is the real client's.
  client = new ApolloClient({
    link: createApolloCsrClient().link,
    cache: new InMemoryCache(apolloCacheConfig),
    defaultOptions: apolloDefaultOptions,
  });
});

afterEach(() => {
  vi.unstubAllGlobals();
  clearWorkspaceTwoFactorBlocks();
});

// The ONE place WORKSPACE_2FA_REQUIRED is read (w22/m402): the production
// client's auth error link feeds the block record the gate walls on.
describe("the browser Apollo client and WORKSPACE_2FA_REQUIRED (w22/m402)", () => {
  it("records the refused workspace, and the query still sees the error", async () => {
    answer({
      data: null,
      errors: [
        {
          message: "this workspace requires two-factor authentication",
          path: ["services"],
          extensions: {
            code: "WORKSPACE_2FA_REQUIRED",
            workspaceId: "tea-acme",
          },
        },
      ],
    });

    const result = await query();

    expect(isWorkspaceTwoFactorBlocked("tea-acme")).toBe(true);
    expect(result.error).toBeDefined();
  });

  it("records nothing for any other refusal", async () => {
    answer({
      data: null,
      errors: [
        {
          message: "forbidden",
          extensions: { code: "FORBIDDEN", workspaceId: "tea-acme" },
        },
      ],
    });
    await query();
    expect(isWorkspaceTwoFactorBlocked("tea-acme")).toBe(false);

    // A transport 403, as the auth gate writes for a REST-shaped refusal.
    answer(
      {
        error: "forbidden",
        message: "not allowed",
        code: "ACCOUNT_DELETION_PENDING",
        params: {},
      },
      403,
    );
    await query();
    expect(isWorkspaceTwoFactorBlocked("tea-acme")).toBe(false);
  });

  it("records nothing for a successful read", async () => {
    answer({ data: { services: [{ __typename: "Service", id: "srv-1" }] } });
    await query();
    expect(isWorkspaceTwoFactorBlocked("tea-acme")).toBe(false);
  });
});
