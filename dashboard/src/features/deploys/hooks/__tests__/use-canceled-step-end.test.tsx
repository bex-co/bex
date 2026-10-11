import { describe, expect, it } from "vitest";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { ApolloClient, ApolloLink, InMemoryCache } from "@apollo/client";
import { ApolloProvider } from "@apollo/client/react";
import { Observable } from "rxjs";
import { apolloCacheConfig } from "@/common/apollo/cache";
import {
  stepLogEnd,
  useCanceledStepEnd,
} from "@/features/deploys/hooks/use-canceled-step-end";
import type { DeployView } from "@/features/deploys/hooks/use-deploy";

// w4/m198: a canceled deploy's continued pre-deploy step ends after the
// cancellation, and only its own pre_deploy_ended fact says when. These drive
// the resolver through a real Apollo client against a fake serviceEvents
// server that applies the API's inclusive window, newest-first order and
// keyset cursor — so paging and deploy matching are exercised, not mocked.

interface Fact {
  id: string;
  deployId: string;
  at: string;
}

function eventsServer(facts: Fact[], opts: { fail?: () => boolean } = {}) {
  const requests: Record<string, unknown>[] = [];
  const link = new ApolloLink(
    (op) =>
      new Observable((sub) => {
        const vars = op.variables as Record<string, string | number>;
        requests.push(vars);
        if (opts.fail?.()) {
          sub.error(new Error("events unavailable"));
          return;
        }
        const start = Date.parse(String(vars.startTime));
        const end = Date.parse(String(vars.endTime));
        const ordered = facts
          .filter((f) => {
            const at = Date.parse(f.at);
            return at >= start && at <= end;
          })
          .sort((a, b) => Date.parse(b.at) - Date.parse(a.at));
        const from = vars.cursor
          ? ordered.findIndex((f) => f.id === vars.cursor) + 1
          : 0;
        const page = ordered.slice(from, from + Number(vars.limit));
        sub.next({
          data: {
            serviceEvents: page.map((f) => ({
              __typename: "ServiceEvent",
              id: f.id,
              timestamp: f.at,
              cursor: f.id,
              details: {
                __typename: "ServiceEventDetails",
                deployId: f.deployId,
              },
            })),
          },
        });
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
  return { requests, wrapper };
}

// The observed successful-step replay (finding.md, dep-db5gnf0k3t2s73e5vb5g).
const canceled = (over: Partial<DeployView> = {}): DeployView => ({
  id: "dep-a",
  status: "canceled",
  trigger: "api",
  image: "",
  rollbackOf: "",
  commitId: "",
  commitMessage: "",
  commitCreatedAt: null,
  createdAt: "2026-10-11T04:08:28.610523Z",
  updatedAt: "2026-10-11T04:09:58.252713Z",
  startedAt: "2026-10-11T04:08:41.494919Z",
  finishedAt: "2026-10-11T04:09:01.5668Z",
  preDeployStatus: "succeeded",
  failureReason: "",
  cancelReason: "",
  stallReason: "",
  ...over,
});

const OWN: Fact = {
  id: "evt-own",
  deployId: "dep-a",
  at: "2026-10-11T04:09:17Z",
};

describe("stepLogEnd", () => {
  it("covers the whole second the step's end fact names", () => {
    // Completion printed at 04:09:14.77s and 04:09:17.9s both stay inside.
    expect(
      stepLogEnd("2026-10-11T04:09:01.5668Z", "2026-10-11T04:09:17Z"),
    ).toBe("2026-10-11T04:09:18.000Z");
  });

  it("never ends before the cancellation itself", () => {
    expect(
      stepLogEnd("2026-10-11T04:09:01.5668Z", "2026-10-11T04:08:50Z"),
    ).toBe("2026-10-11T04:09:01.5668Z");
  });
});

describe("useCanceledStepEnd", () => {
  it("checks, then ends the window at this deploy's own step-end fact", async () => {
    const { requests, wrapper } = eventsServer([OWN]);
    const { result } = renderHook(
      () => useCanceledStepEnd("srv-1", canceled()),
      { wrapper },
    );
    expect(result.current).toEqual({ status: "checking" });
    await waitFor(() =>
      expect(result.current).toEqual({
        status: "found",
        end: "2026-10-11T04:09:18.000Z",
      }),
    );
    // The search interval reaches past the cancellation, to the settle time.
    expect(requests).toHaveLength(1);
    expect(requests[0].startTime).toBe("2026-10-11T04:08:28.610523Z");
    expect(Date.parse(String(requests[0].endTime))).toBeGreaterThan(
      Date.parse("2026-10-11T04:09:58Z"),
    );
  });

  it("pages past a full first page of other deploys' later step ends", async () => {
    const others: Fact[] = Array.from({ length: 100 }, (_, i) => ({
      id: `evt-other-${i}`,
      deployId: `dep-other-${i}`,
      at: new Date(Date.parse("2026-10-11T04:09:20Z") + i * 100).toISOString(),
    }));
    const { requests, wrapper } = eventsServer([...others, OWN]);
    const { result } = renderHook(
      () => useCanceledStepEnd("srv-1", canceled()),
      { wrapper },
    );
    await waitFor(() =>
      expect(result.current).toEqual({
        status: "found",
        end: "2026-10-11T04:09:18.000Z",
      }),
    );
    expect(requests).toHaveLength(2);
    // One frozen interval across pages; only the cursor advances.
    expect(requests[1].startTime).toBe(requests[0].startTime);
    expect(requests[1].endTime).toBe(requests[0].endTime);
    expect(requests[1].cursor).toBe("evt-other-0");
  });

  it("reports missing rather than borrowing another deploy's step end", async () => {
    const { wrapper } = eventsServer([
      { id: "evt-b", deployId: "dep-b", at: "2026-10-11T04:09:40Z" },
    ]);
    const { result } = renderHook(
      () => useCanceledStepEnd("srv-1", canceled()),
      { wrapper },
    );
    await waitFor(() => expect(result.current).toEqual({ status: "missing" }));
  });

  it("surfaces a failed read with a retry that re-reads the fact", async () => {
    let fail = true;
    const { requests, wrapper } = eventsServer([OWN], { fail: () => fail });
    const { result } = renderHook(
      () => useCanceledStepEnd("srv-1", canceled()),
      { wrapper },
    );
    await waitFor(() => expect(result.current.status).toBe("error"));
    fail = false;
    act(() => {
      if (result.current.status === "error") result.current.retry();
    });
    await waitFor(() => expect(result.current.status).toBe("found"));
    expect(requests).toHaveLength(2);
  });

  it.each([
    ["a live deploy", { status: "live" }],
    ["a deploy with no pre-deploy step", { preDeployStatus: "" }],
    ["a step that was actually stopped", { preDeployStatus: "canceled" }],
    [
      "a failed step that closed the deploy",
      { status: "pre_deploy_failed", preDeployStatus: "failed" },
    ],
  ])("keeps the normal window for %s", async (_name, over) => {
    const { requests, wrapper } = eventsServer([OWN]);
    const { result } = renderHook(
      () => useCanceledStepEnd("srv-1", canceled(over)),
      { wrapper },
    );
    expect(result.current).toEqual({ status: "none" });
    await new Promise((r) => setTimeout(r, 20));
    expect(requests).toHaveLength(0);
  });

  it("resolves afresh for another deploy id", async () => {
    const { wrapper } = eventsServer([
      OWN,
      { id: "evt-b", deployId: "dep-b", at: "2026-10-11T04:09:30Z" },
    ]);
    const { result, rerender } = renderHook(
      ({ id }) => useCanceledStepEnd("srv-1", canceled({ id })),
      { wrapper, initialProps: { id: "dep-a" } },
    );
    await waitFor(() => expect(result.current.status).toBe("found"));
    rerender({ id: "dep-b" });
    expect(result.current).toEqual({ status: "checking" });
    await waitFor(() =>
      expect(result.current).toEqual({
        status: "found",
        end: "2026-10-11T04:09:31.000Z",
      }),
    );
  });
});
