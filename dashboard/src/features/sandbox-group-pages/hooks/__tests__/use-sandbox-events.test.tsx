import { describe, expect, it } from "vitest";
import type { ReactNode } from "react";
import { renderHook, waitFor } from "@testing-library/react";
import { ApolloClient, InMemoryCache } from "@apollo/client";
import { ApolloProvider } from "@apollo/client/react";
import { MockLink } from "@apollo/client/testing";
import { SandboxEventsDocument } from "@/graphql/definitions";
import { useSandboxEvents } from "../use-group-pages";

const SANDBOX = "sbx-0000000000000000000a";
const OWNER = "tea-a";
const VARIABLES = { sandboxId: SANDBOX, ownerId: OWNER, limit: 50 };

function lifecycle(id: string, type: string, timestamp: string) {
  return {
    __typename: "SandboxLifecycleEvent",
    id,
    timestamp,
    outcome: "SUCCESS",
    label: type,
    lifecycleType: type,
    trigger: "DASHBOARD",
    userEmail: null,
  };
}

const CREATED = lifecycle("evt-1", "CREATED", "2026-11-26T10:00:00Z");
const TERMINATED = lifecycle("evt-2", "TERMINATED", "2026-11-26T10:05:00Z");

function answer(events: unknown[]) {
  return {
    request: { query: SandboxEventsDocument, variables: VARIABLES },
    result: { data: { sandboxEvents: { hasMore: false, events } } },
  };
}

describe("useSandboxEvents (w1/m658 t009)", () => {
  // On production the Terminated event stayed missing until a reload: the
  // newest page polls only while the sandbox runs, so the poll stopped with
  // the status change that the event records.
  it("reads the newest page again when the sandbox's status changes", async () => {
    const client = new ApolloClient({
      cache: new InMemoryCache(),
      link: new MockLink([answer([CREATED]), answer([TERMINATED, CREATED])]),
    });
    const wrapper = ({ children }: { children: ReactNode }) => (
      <ApolloProvider client={client}>{children}</ApolloProvider>
    );
    const { result, rerender } = renderHook(
      ({ status }: { status: string }) =>
        useSandboxEvents(SANDBOX, OWNER, status),
      { wrapper, initialProps: { status: "running" } },
    );
    await waitFor(() =>
      expect(result.current.events.map((e) => e.id)).toEqual(["evt-1"]),
    );

    rerender({ status: "terminated" });
    await waitFor(() =>
      expect(result.current.events.map((e) => e.id)).toEqual([
        "evt-2",
        "evt-1",
      ]),
    );
  });
});
