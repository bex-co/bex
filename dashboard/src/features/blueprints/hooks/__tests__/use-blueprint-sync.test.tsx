import { describe, expect, it } from "vitest";
import type { ReactNode } from "react";
import { act, renderHook, waitFor } from "@testing-library/react";
import { ApolloClient, InMemoryCache } from "@apollo/client";
import { ApolloProvider } from "@apollo/client/react";
import { MockLink } from "@apollo/client/testing";
import { GraphQLError } from "graphql";
import {
  ApproveBlueprintDocument,
  BlueprintSyncDocument,
  PlanBlueprintSyncDocument,
} from "@/graphql/definitions";
import { useBlueprintSync } from "../use-blueprint-sync";
import { usePlanBlueprintSync } from "../use-plan-blueprint-sync";

const ID = "exs-db136288mmqc73d4hpug";
const SYNC = "exe-waiting";

function sync(state: string, overrides: Record<string, unknown> = {}) {
  return {
    __typename: "BlueprintSyncDetail",
    id: SYNC,
    commitId: "0123456789abcdef",
    commitMessage: "Serve the api",
    state,
    startedAt: "2026-11-26T10:00:00Z",
    completedAt: null,
    errorMessage: null,
    note: null,
    actionsRecorded: true,
    actions: [
      {
        __typename: "BlueprintPlanAction",
        operation: "create",
        kind: "service",
        name: "api",
        sourcePath: "#/services/0",
        resourceId: null,
        changedFields: [],
        message: null,
      },
      {
        __typename: "BlueprintPlanAction",
        operation: "noop",
        kind: "service",
        name: "web",
        sourcePath: "#/services/1",
        resourceId: "srv-web",
        changedFields: [],
        message: null,
      },
    ],
    actionStates: [
      { __typename: "BlueprintSyncActionState", state },
      { __typename: "BlueprintSyncActionState", state },
    ],
    errors: [],
    requiredManualEnvVars:
      state === "created"
        ? [
            {
              __typename: "BlueprintManualEnvVar",
              serviceType: "web",
              serviceName: "api",
              key: "API_TOKEN",
            },
          ]
        : [],
    ...overrides,
  };
}

const syncQuery = {
  query: BlueprintSyncDocument,
  variables: { id: ID, syncId: SYNC },
};
const approveRequest = {
  query: ApproveBlueprintDocument,
  variables: {
    id: ID,
    syncId: SYNC,
    manualEnvVars: [
      {
        serviceType: "web",
        serviceName: "api",
        key: "API_TOKEN",
        value: "s3cret",
      },
    ],
  },
};

function render<T>(link: MockLink, hook: () => T) {
  const client = new ApolloClient({ link, cache: new InMemoryCache() });
  const wrapper = ({ children }: { children: ReactNode }) => (
    <ApolloProvider client={client}>{children}</ApolloProvider>
  );
  return renderHook(hook, { wrapper });
}

// w3/m678: a sync's page reads its sync, each action with the state the
// server gives it, and approves a waiting plan with its sync:false values,
// bound to this sync's id. Real client and hooks; only the network is
// scripted.
describe("useBlueprintSync", () => {
  it("reads each action with its state and the values approving needs", async () => {
    const link = new MockLink([
      {
        request: syncQuery,
        result: { data: { blueprintSync: sync("created") } },
      },
    ]);
    const { result } = render(link, () => useBlueprintSync(ID, SYNC));

    await waitFor(() => expect(result.current.sync?.id).toBe(SYNC));
    const view = result.current.sync!;
    expect(view.commitMessage).toBe("Serve the api");
    expect(
      view.actions.map(
        (a) => `${a.action.operation} ${a.action.name} ${a.state}`,
      ),
    ).toEqual(["create api created", "noop web created"]);
    expect(view.requiredManualEnvVars).toEqual([
      { serviceType: "web", serviceName: "api", key: "API_TOKEN" },
    ]);
  });

  it("approves the plan on screen with its values, then reads it again", async () => {
    const link = new MockLink([
      {
        request: syncQuery,
        result: { data: { blueprintSync: sync("created") } },
      },
      {
        request: approveRequest,
        result: {
          data: {
            approveBlueprint: {
              __typename: "BlueprintApprovedSync",
              id: SYNC,
              state: "success",
            },
          },
        },
      },
      {
        request: syncQuery,
        result: { data: { blueprintSync: sync("success") } },
      },
    ]);
    const { result } = render(link, () => useBlueprintSync(ID, SYNC));
    await waitFor(() => expect(result.current.sync?.state).toBe("created"));

    await act(() => result.current.approve({ "api/API_TOKEN": "s3cret" }));

    expect(result.current.approveError).toBeNull();
    await waitFor(() => expect(result.current.sync?.state).toBe("success"));
    expect(result.current.sync?.actions[0].state).toBe("success");
  });

  it("says when someone planned again before the approval", async () => {
    const link = new MockLink([
      {
        request: syncQuery,
        result: { data: { blueprintSync: sync("created") } },
      },
      {
        request: approveRequest,
        result: {
          errors: [
            new GraphQLError(
              "sync exe-waiting is no longer this Blueprint's newest plan",
              {
                extensions: { code: "BLUEPRINT_PLAN_CHANGED" },
              },
            ),
          ],
        },
      },
    ]);
    const { result } = render(link, () => useBlueprintSync(ID, SYNC));
    await waitFor(() => expect(result.current.sync?.state).toBe("created"));

    await act(() => result.current.approve({ "api/API_TOKEN": "s3cret" }));

    expect(result.current.approveError).toBe(
      "Someone planned this Blueprint again. Review the newest plan, then approve it.",
    );
    expect(result.current.approving).toBe(false);
  });

  it("relays the server's reason, else Render's retry line", async () => {
    const link = new MockLink([
      {
        request: syncQuery,
        result: { data: { blueprintSync: sync("created") } },
      },
      {
        request: approveRequest,
        result: {
          errors: [
            new GraphQLError(
              'conflict: another Blueprint named "app" was approved in this workspace',
              {
                extensions: { code: "BLUEPRINT_NAME_TAKEN" },
              },
            ),
          ],
        },
      },
      { request: approveRequest, error: new Error("Failed to fetch") },
    ]);
    const { result } = render(link, () => useBlueprintSync(ID, SYNC));
    await waitFor(() => expect(result.current.sync?.state).toBe("created"));

    await act(() => result.current.approve({ "api/API_TOKEN": "s3cret" }));
    expect(result.current.approveError).toBe(
      'Another Blueprint named "app" was approved in this workspace',
    );
    await act(() => result.current.approve({ "api/API_TOKEN": "s3cret" }));
    expect(result.current.approveError).toBe(
      "Couldn't approve this sync. Try again.",
    );
  });

  it("reads a sync that is not this Blueprint's as none", async () => {
    const link = new MockLink([
      {
        request: syncQuery,
        result: {
          data: { blueprintSync: null },
          errors: [
            new GraphQLError("not found: blueprint_sync", {
              extensions: { code: "NOT_FOUND" },
            }),
          ],
        },
      },
    ]);
    const { result } = render(link, () => useBlueprintSync(ID, SYNC));

    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.sync).toBeNull();
    expect(result.current.error).toBeDefined();
  });
});

// Manual sync's plan (w3/m678): a plan that saved a sync names it, one with
// no changes saved none, and a refusal carries the server's reason.
describe("usePlanBlueprintSync", () => {
  const planRequest = {
    query: PlanBlueprintSyncDocument,
    variables: { id: ID },
  };
  const planned = (outcome: string, syncId: string | null) => ({
    data: {
      planBlueprintSync: {
        __typename: "BlueprintPlanResult",
        planningOutcome: outcome,
        notices: [],
        sync: syncId
          ? { __typename: "BlueprintPlannedSync", id: syncId, state: "created" }
          : null,
      },
    },
  });

  it.each([
    [planned("planned", "exe-new"), { status: "planned", syncId: "exe-new" }],
    [
      planned("failed", "exe-failed"),
      { status: "planned", syncId: "exe-failed" },
    ],
    [planned("no_changes", null), { status: "no_changes" }],
    [
      {
        errors: [
          new GraphQLError(
            "bad request: Blueprint file not found on branch main",
            {
              extensions: { code: "BAD_REQUEST" },
            },
          ),
        ],
      },
      { status: "error", reason: "Blueprint file not found on branch main" },
    ],
  ])("answers %j as %j", async (answer, want) => {
    const link = new MockLink([{ request: planRequest, result: answer }]);
    const { result } = render(link, () => usePlanBlueprintSync());

    let got: unknown;
    await act(async () => {
      got = await result.current.plan(ID);
    });
    expect(got).toEqual(want);
  });
});
