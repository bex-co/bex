import type { ReactNode } from "react";
import { ApolloClient, ApolloLink, InMemoryCache } from "@apollo/client";
import { ApolloProvider } from "@apollo/client/react";
import { act, renderHook, waitFor } from "@testing-library/react";
import { Observable } from "rxjs";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { apolloCacheConfig } from "@/common/apollo/cache";
import { apolloDefaultOptions } from "@/common/apollo/default-options";
import type { ServerQuery } from "@/graphql/definitions";
import { useEnvironmentDraftSave } from "../use-environment-draft-save";
import { useEnvVarKeys } from "../use-env-vars";
import { useSecretFileNames } from "../use-secret-files";
import { useServer } from "../use-server";

function service(undeployedChanges: boolean): ServerQuery["server"] {
  return {
    __typename: "Service",
    id: "srv-save",
    name: "save-only",
    slug: null,
    immutableName: null,
    displayName: null,
    type: "web_service",
    suspended: "not_suspended",
    dashboardUrl: null,
    region: null,
    url: null,
    publicRoutingNotice: null,
    undeployedChanges,
    internalAddress: null,
    createdAt: null,
    sshAddress: null,
    phase: "Running",
    replicas: 1,
    revision: "live-v1",
    plan: null,
    idleTTLSeconds: 0,
    repo: null,
    branch: null,
    imagePath: null,
    rootDir: null,
    runtime: null,
    builder: null,
    buildCommand: null,
    startCommand: null,
    dockerfilePath: null,
    registryCredentialId: null,
    autoDeploy: null,
    linkedEnvGroupIds: null,
    pushDeliveryMethod: null,
    notifyOnFail: null,
    notificationsToSend: null,
    renderSubdomainPolicy: null,
    healthCheckPath: null,
    port: 3000,
    maxShutdownDelaySeconds: null,
    preDeployCommand: null,
    schedule: null,
    command: null,
    lastSuccessfulRunAt: null,
    nextRunAt: null,
    publishPath: null,
    ipAllowList: null,
    maintenanceMode: null,
    buildFilter: null,
    runs: null,
    routes: null,
    headers: null,
    ipAllowListEntries: null,
    ipAllowListProxiedDomains: null,
    outboundIps: null,
  };
}

// Use the real documents, mutation, cache and two Server observers: the layout
// polls, and the Environment page reads that same cache without another timer.
function harness(initialPending = false) {
  const state = {
    pending: initialPending,
    failedRead: "",
    failedWrite: false,
    writes: 0,
    requests: [] as string[],
  };
  const client = new ApolloClient({
    cache: new InMemoryCache(apolloCacheConfig),
    defaultOptions: apolloDefaultOptions,
    link: new ApolloLink(
      (operation) =>
        new Observable((observer) => {
          const timer = setTimeout(() => {
            const name = operation.operationName ?? "";
            state.requests.push(name);
            if (
              state.failedRead === name ||
              (state.failedWrite && name === "PatchServiceEnvironment")
            ) {
              observer.error(new Error(`${name} unavailable`));
              return;
            }
            switch (name) {
              case "Server":
                observer.next({ data: { server: service(state.pending) } });
                break;
              case "EnvVarKeys":
                observer.next({ data: { envVars: [] } });
                break;
              case "SecretFileNames":
                observer.next({
                  data: {
                    secretFiles: [
                      {
                        __typename: "SecretFileWithCursor",
                        cursor: "message.txt",
                        secretFile: {
                          __typename: "SecretFileListValue",
                          id: "message.txt",
                          name: "message.txt",
                        },
                      },
                    ],
                  },
                });
                break;
              case "PatchServiceEnvironment":
                state.writes++;
                observer.next({
                  data: {
                    patchServiceEnvironment: {
                      __typename: "EnvironmentPatchResult",
                      envVarKeys: [],
                      secretFileNames: ["message.txt"],
                      rolledOut: false,
                    },
                  },
                });
                break;
              default:
                observer.error(new Error(`unexpected operation ${name}`));
                return;
            }
            observer.complete();
          }, 1);
          return () => clearTimeout(timer);
        }),
    ),
  });
  const hook = renderHook(
    () => ({
      header: useServer("srv-save"),
      page: useServer("srv-save", { poll: false }),
      env: useEnvVarKeys("srv-save"),
      files: useSecretFileNames("srv-save"),
      editor: useEnvironmentDraftSave(),
    }),
    {
      wrapper: ({ children }: { children: ReactNode }) => (
        <ApolloProvider client={client}>{children}</ApolloProvider>
      ),
    },
  );
  return { ...hook, state };
}

const patch = {
  envVars: [],
  secretFiles: [{ name: "message.txt", content: "saved-v2" }],
};

beforeEach(() => vi.useFakeTimers({ shouldAdvanceTime: true }));
afterEach(() => vi.useRealTimers());

describe("service environment save/refetch", () => {
  it("keeps status authoritative through a stale first read, a later poll and a revert", async () => {
    const { result, state } = harness();
    await waitFor(() =>
      expect(result.current.header.service?.undeployedChanges).toBe(false),
    );
    await waitFor(() => expect(result.current.files.names).toHaveLength(1));
    state.requests = [];

    await act(async () => {
      expect(
        await result.current.editor.save("srv-save", patch, "save_only"),
      ).toMatchObject({ rolledOut: false, refreshFailed: false });
    });
    // The accepted write does not prove that saved bytes differ from live
    // bytes. The first response may precede reconciliation or be a true no-op.
    expect(result.current.header.service?.undeployedChanges).toBe(false);
    expect(result.current.page.service?.undeployedChanges).toBe(false);
    expect(state.requests.sort()).toEqual([
      "EnvVarKeys",
      "PatchServiceEnvironment",
      "SecretFileNames",
      "Server",
    ]);
    expect(state.writes).toBe(1);

    state.pending = true;
    await act(async () => {
      await vi.advanceTimersByTimeAsync(31_000);
    });
    expect(result.current.header.service?.undeployedChanges).toBe(true);
    expect(result.current.page.service?.undeployedChanges).toBe(true);
    expect(result.current.header.service?.revision).toBe("live-v1");

    // Saving the live value again can clear pending without a deployment.
    state.pending = false;
    await act(async () => {
      await result.current.editor.save(
        "srv-save",
        {
          ...patch,
          secretFiles: [{ name: "message.txt", content: "live-v1" }],
        },
        "save_only",
      );
    });
    expect(result.current.header.service?.undeployedChanges).toBe(false);
    expect(result.current.page.service?.undeployedChanges).toBe(false);
    expect(state.writes).toBe(2);
  });

  it.each(["Server", "EnvVarKeys", "SecretFileNames"])(
    "reports a committed save when the %s refetch fails",
    async (failedRead) => {
      const { result, state } = harness(true);
      await waitFor(() =>
        expect(result.current.header.service?.undeployedChanges).toBe(true),
      );
      await waitFor(() => expect(result.current.files.names).toHaveLength(1));
      state.failedRead = failedRead;

      await act(async () => {
        expect(
          await result.current.editor.save("srv-save", patch, "save_only"),
        ).toMatchObject({ rolledOut: false, refreshFailed: true });
      });
      expect(state.writes).toBe(1);
      expect(result.current.editor.saving).toBe(false);
      if (failedRead === "Server") {
        // The existing detail read exposes its error instead of a fresh view;
        // neither an error nor the save response writes an invented false.
        expect(result.current.header.error?.message).toBe("Server unavailable");
        expect(result.current.header.service).toBeNull();
        state.failedRead = "";
        await act(async () => {
          await Promise.all([
            result.current.header.refetch(),
            result.current.page.refetch(),
          ]);
        });
      }
      expect(result.current.header.service?.undeployedChanges).toBe(true);
      expect(result.current.page.service?.undeployedChanges).toBe(true);
    },
  );

  it("rejects a failed write without refreshing or changing the saved/live flag", async () => {
    const { result, state } = harness(true);
    await waitFor(() =>
      expect(result.current.header.service?.undeployedChanges).toBe(true),
    );
    await waitFor(() => expect(result.current.files.names).toHaveLength(1));
    state.requests = [];
    state.failedWrite = true;

    await act(async () => {
      await expect(
        result.current.editor.save("srv-save", patch, "save_only"),
      ).rejects.toThrow("PatchServiceEnvironment unavailable");
    });
    expect(state.writes).toBe(0);
    expect(state.requests).toEqual(["PatchServiceEnvironment"]);
    expect(result.current.header.service?.undeployedChanges).toBe(true);
    expect(result.current.page.service?.undeployedChanges).toBe(true);
  });
});
