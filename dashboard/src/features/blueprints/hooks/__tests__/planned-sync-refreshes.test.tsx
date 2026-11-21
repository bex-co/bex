import { describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";
import { act, renderHook, waitFor } from "@testing-library/react";
import { ApolloClient, InMemoryCache } from "@apollo/client";
import { ApolloProvider } from "@apollo/client/react";
import { MockLink } from "@apollo/client/testing";
import {
  ApproveBlueprintDocument,
  BlueprintDocument,
  BlueprintPlannedSyncDocument,
  PlanBlueprintSyncDocument,
} from "@/graphql/definitions";
import { useBlueprint } from "../use-blueprint";
import { usePlannedSync } from "../use-planned-sync";

vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}));

const ID = "exs-db136288mmqc73d4hpug";

function blueprint(status: string) {
  return {
    __typename: "Blueprint",
    id: ID,
    name: "qa-bp",
    ownerId: "tea-1",
    resourceSuffix: "",
    repo: "https://github.com/render-examples/go-web-server",
    branch: "main",
    path: "render.yaml",
    autoSync: true,
    manifest: "services: []",
    status,
    lastSync: "2026-11-21T00:00:00Z",
    resources: [],
    createdAt: "2026-11-21T00:00:00Z",
    updatedAt: "2026-11-21T00:00:00Z",
  };
}

const plan = {
  __typename: "BlueprintPlannedSync",
  id: "exe-waiting",
  commitId: "0123456789abcdef",
  state: "created",
  actions: [],
  requiredManualEnvVars: [],
};

const blueprintQuery = { query: BlueprintDocument, variables: { id: ID } };
const plannedQuery = {
  query: BlueprintPlannedSyncDocument,
  variables: { id: ID },
};

function renderPage(link: MockLink) {
  const client = new ApolloClient({ link, cache: new InMemoryCache() });
  const wrapper = ({ children }: { children: ReactNode }) => (
    <ApolloProvider client={client}>{children}</ApolloProvider>
  );
  return renderHook(
    () => ({ page: useBlueprint(ID), plan: usePlannedSync(ID, true) }),
    { wrapper },
  );
}

// w32/076: the Blueprint page shows a waiting plan whatever the Blueprint's
// status, so leaving `created` no longer unmounts the card. Real client, real
// hooks; only the network is scripted.
describe("planned sync → Blueprint page", () => {
  it("re-reads the plan after approving it, so the applied plan leaves the page", async () => {
    const link = new MockLink([
      {
        request: blueprintQuery,
        result: { data: { blueprint: blueprint("in_sync") } },
      },
      {
        request: plannedQuery,
        result: { data: { blueprintPlannedSync: plan } },
      },
      {
        request: {
          query: ApproveBlueprintDocument,
          variables: { id: ID, syncId: "exe-waiting", manualEnvVars: [] },
        },
        result: {
          data: {
            approveBlueprint: {
              __typename: "BlueprintApprovedSync",
              id: "exe-waiting",
              state: "pending",
            },
          },
        },
      },
      {
        request: plannedQuery,
        result: { data: { blueprintPlannedSync: null } },
      },
      {
        request: blueprintQuery,
        result: { data: { blueprint: blueprint("syncing") } },
      },
    ]);
    const { result } = renderPage(link);
    await waitFor(() =>
      expect(result.current.plan.planned?.id).toBe("exe-waiting"),
    );

    let approved: boolean | undefined;
    await act(async () => {
      approved = await result.current.plan.approve({});
    });

    expect(approved).toBe(true);
    await waitFor(() => expect(result.current.plan.planned).toBeNull());
    await waitFor(() =>
      expect(result.current.page.blueprint?.status).toBe("syncing"),
    );
  });

  it("re-reads the Blueprint after a no-changes plan takes it out of created (w32/073)", async () => {
    const link = new MockLink([
      {
        request: blueprintQuery,
        result: { data: { blueprint: blueprint("created") } },
      },
      {
        request: plannedQuery,
        result: { data: { blueprintPlannedSync: plan } },
      },
      {
        request: { query: PlanBlueprintSyncDocument, variables: { id: ID } },
        result: {
          data: {
            planBlueprintSync: {
              __typename: "BlueprintPlanResult",
              planningOutcome: "no_changes",
              notices: [],
            },
          },
        },
      },
      // The waiting first plan stays; the Blueprint is now in sync.
      {
        request: plannedQuery,
        result: { data: { blueprintPlannedSync: plan } },
      },
      {
        request: blueprintQuery,
        result: { data: { blueprint: blueprint("in_sync") } },
      },
    ]);
    const { result } = renderPage(link);
    await waitFor(() =>
      expect(result.current.page.blueprint?.status).toBe("created"),
    );
    await waitFor(() =>
      expect(result.current.plan.planned?.id).toBe("exe-waiting"),
    );

    await act(async () => {
      await result.current.plan.replan();
    });

    await waitFor(() =>
      expect(result.current.page.blueprint?.status).toBe("in_sync"),
    );
    expect(result.current.plan.planned?.id).toBe("exe-waiting");
  });
});
