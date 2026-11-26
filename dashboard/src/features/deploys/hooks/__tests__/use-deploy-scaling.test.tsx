import { describe, expect, it } from "vitest";
import { renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { ApolloClient, ApolloLink, InMemoryCache } from "@apollo/client";
import { ApolloProvider } from "@apollo/client/react";
import { Observable } from "rxjs";
import { apolloCacheConfig } from "@/common/apollo/cache";
import { useDeployScaling } from "@/features/deploys/hooks/use-deploy-scaling";
import type { DeployView } from "@/features/deploys/hooks/use-deploy";
import type { ServiceView } from "@/features/services/types";

// w1/m661: the deploy header's Scaling fact, read through a real Apollo
// client from a fake server that answers autoscalingConfig and
// serviceInstances the way bex-api does, and records what was asked.

interface Instance {
  id: string;
  deployId: string | null;
  status: string;
}

interface ServerState {
  autoscaling: { enabled: boolean; minInstances: number };
  instances: Instance[];
  failInstances?: boolean;
}

function scalingServer(state: ServerState) {
  const asked: string[] = [];
  const link = new ApolloLink(
    (op) =>
      new Observable((sub) => {
        asked.push(op.operationName ?? "");
        if (op.operationName === "AutoscalingConfig") {
          sub.next({
            data: {
              autoscalingConfig: {
                __typename: "Autoscaling",
                maxInstances: 10,
                targetCPUPercent: 70,
                targetMemoryPercent: null,
                metricsUnavailable: false,
                ...state.autoscaling,
              },
            },
          });
        } else if (op.operationName === "DeployInstances") {
          if (state.failInstances) {
            sub.error(new Error("instances unavailable"));
            return;
          }
          sub.next({
            data: {
              serviceInstances: state.instances.map((i) => ({
                __typename: "ServiceInstance",
                ...i,
              })),
            },
          });
        } else {
          sub.error(new Error(`unexpected ${op.operationName}`));
          return;
        }
        sub.complete();
      }),
  );
  const client = new ApolloClient({
    link,
    cache: new InMemoryCache(apolloCacheConfig),
  });
  const wrapper = ({ children }: { children: ReactNode }) => (
    <ApolloProvider client={client}>{children}</ApolloProvider>
  );
  const count = (name: string) => asked.filter((n) => n === name).length;
  return { asked, count, wrapper };
}

const service = (over: Partial<ServiceView> = {}) =>
  ({ id: "srv-1", type: "web_service", replicas: 2, ...over }) as ServiceView;

const deploy = (status: string, id = "dep-new") =>
  ({ id, status }) as DeployView;

const noAutoscaling = { enabled: false, minInstances: 1 };

const rollingTwo: Instance[] = [
  { id: "srv-1-a", deployId: "dep-new", status: "running" },
  { id: "srv-1-b", deployId: "dep-new", status: "pending" },
  { id: "srv-1-c", deployId: "dep-old", status: "running" },
];

describe("useDeployScaling", () => {
  it("counts a 2-instance service's in-progress deploy, holding the place while it loads", async () => {
    const server = scalingServer({
      autoscaling: noAutoscaling,
      instances: rollingTwo,
    });
    const { result } = renderHook(
      () => useDeployScaling("srv-1", service(), deploy("update_in_progress")),
      { wrapper: server.wrapper },
    );
    expect(result.current).toBe("loading");
    await waitFor(() =>
      expect(result.current).toEqual({ deployed: 1, total: 2 }),
    );
  });

  it("reads no instances for a single-instance service without autoscaling", async () => {
    const server = scalingServer({
      autoscaling: noAutoscaling,
      instances: rollingTwo,
    });
    const { result } = renderHook(
      () => useDeployScaling("srv-1", service({ replicas: 1 }), deploy("live")),
      { wrapper: server.wrapper },
    );
    await waitFor(() => expect(server.count("AutoscalingConfig")).toBe(1));
    expect(result.current).toBeNull();
    expect(server.count("DeployInstances")).toBe(0);
  });

  it("counts an autoscaled single-instance service against its minimum", async () => {
    const server = scalingServer({
      autoscaling: { enabled: true, minInstances: 3 },
      instances: rollingTwo,
    });
    const { result } = renderHook(
      () => useDeployScaling("srv-1", service({ replicas: 1 }), deploy("live")),
      { wrapper: server.wrapper },
    );
    await waitFor(() =>
      expect(result.current).toEqual({ deployed: 1, total: 3 }),
    );
  });

  it.each(["deactivated", "update_failed", "canceled"])(
    "reads nothing for a %s deploy opened afresh",
    async (status) => {
      const server = scalingServer({
        autoscaling: noAutoscaling,
        instances: rollingTwo,
      });
      const { result } = renderHook(
        () => useDeployScaling("srv-1", service(), deploy(status)),
        { wrapper: server.wrapper },
      );
      expect(result.current).toBeNull();
      await new Promise((r) => setTimeout(r, 20));
      expect(server.asked).toEqual([]);
    },
  );

  it.each(["static_site", "cron_job"])(
    "reads nothing for a %s, which has no instances",
    async (type) => {
      const server = scalingServer({
        autoscaling: noAutoscaling,
        instances: rollingTwo,
      });
      const { result } = renderHook(
        () => useDeployScaling("srv-1", service({ type }), deploy("live")),
        { wrapper: server.wrapper },
      );
      expect(result.current).toBeNull();
      await new Promise((r) => setTimeout(r, 20));
      expect(server.asked).toEqual([]);
    },
  );

  it("keeps the count of a deploy it watched in progress once it fails", async () => {
    const server = scalingServer({
      autoscaling: noAutoscaling,
      instances: rollingTwo,
    });
    const { result, rerender } = renderHook(
      ({ status }) => useDeployScaling("srv-1", service(), deploy(status)),
      {
        wrapper: server.wrapper,
        initialProps: { status: "update_in_progress" },
      },
    );
    await waitFor(() =>
      expect(result.current).toEqual({ deployed: 1, total: 2 }),
    );
    rerender({ status: "update_failed" });
    expect(result.current).toEqual({ deployed: 1, total: 2 });
  });

  it("reads the count again the moment the rollout goes live", async () => {
    const state: ServerState = {
      autoscaling: noAutoscaling,
      instances: rollingTwo,
    };
    const server = scalingServer(state);
    const { result, rerender } = renderHook(
      ({ status }) => useDeployScaling("srv-1", service(), deploy(status)),
      {
        wrapper: server.wrapper,
        initialProps: { status: "update_in_progress" },
      },
    );
    await waitFor(() =>
      expect(result.current).toEqual({ deployed: 1, total: 2 }),
    );
    state.instances = [
      { id: "srv-1-a", deployId: "dep-new", status: "running" },
      { id: "srv-1-b", deployId: "dep-new", status: "running" },
    ];
    rerender({ status: "live" });
    await waitFor(() =>
      expect(result.current).toEqual({ deployed: 2, total: 2 }),
    );
  });

  it("shows nothing when the instances cannot be read", async () => {
    const server = scalingServer({
      autoscaling: noAutoscaling,
      instances: [],
      failInstances: true,
    });
    const { result } = renderHook(
      () => useDeployScaling("srv-1", service(), deploy("live")),
      { wrapper: server.wrapper },
    );
    await waitFor(() => expect(server.count("DeployInstances")).toBe(1));
    await waitFor(() => expect(result.current).toBeNull());
  });
});
