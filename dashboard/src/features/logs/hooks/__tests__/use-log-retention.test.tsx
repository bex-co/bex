import { afterEach, describe, expect, it } from "vitest";
import { act, cleanup, render, screen, waitFor } from "@testing-library/react";
import {
  ApolloClient,
  ApolloLink,
  InMemoryCache,
  Observable,
} from "@apollo/client";
import { ApolloProvider } from "@apollo/client/react";
import { useLogRetentionBound } from "../use-log-retention";

// w9/m389: the log viewers bound their range picker by the window bex-api's
// logRetention reports for the resource, with a hint naming the plan's window
// and the next plan's — no plan table in the client. The answer holds still,
// so it is read once and again when the tab regains focus.

type Retention = {
  plan: string;
  days: number;
  visibleFrom: string | null;
  upgradePlan: string | null;
  upgradeDays: number | null;
} | null;

function client(retention: Retention, seen: Record<string, unknown>[] = []) {
  return new ApolloClient({
    cache: new InMemoryCache(),
    link: new ApolloLink(
      (operation) =>
        new Observable<ApolloLink.Result>((observer) => {
          seen.push(operation.variables);
          observer.next({
            data: {
              logRetention: retention && {
                __typename: "LogRetention",
                ...retention,
              },
            },
          });
          observer.complete();
        }),
    ),
  });
}

function Probe({ resource }: { resource: string }) {
  const bound = useLogRetentionBound(resource);
  return (
    <p data-testid="probe">
      {bound
        ? `${bound.maxSpanSeconds}|${bound.notBeforeMs ?? "-"}|${bound.hint}`
        : "unbounded"}
    </p>
  );
}

function mount(c: ApolloClient, resource = "srv-1") {
  render(
    <ApolloProvider client={c}>
      <Probe resource={resource} />
    </ApolloProvider>,
  );
}

const probe = () => screen.getByTestId("probe").textContent;

afterEach(cleanup);

const HOBBY: Retention = {
  plan: "hobby",
  days: 7,
  visibleFrom: null,
  upgradePlan: "pro",
  upgradeDays: 14,
};

describe("useLogRetentionBound", () => {
  it("bounds a Hobby workspace's logs to 7 days and points at Pro's 14", async () => {
    const seen: Record<string, unknown>[] = [];
    mount(client(HOBBY, seen), "dpg-x");
    await waitFor(() => expect(probe()).not.toBe("unbounded"));
    expect(probe()).toBe(
      `${7 * 24 * 60 * 60}|-|Logs are kept for 7 days on the Hobby plan. Upgrade to Pro to keep 14 days.`,
    );
    expect(seen[0]).toEqual({ resource: "dpg-x" });
  });

  it("keeps an upgrade's floor as the bound's notBeforeMs", async () => {
    const visibleFrom = "2026-11-05T12:00:00Z";
    mount(
      client({
        plan: "pro",
        days: 14,
        visibleFrom,
        upgradePlan: "scale",
        upgradeDays: 30,
      }),
    );
    await waitFor(() => expect(probe()).not.toBe("unbounded"));
    expect(probe()).toMatch(
      new RegExp(`^${14 * 24 * 60 * 60}\\|${Date.parse(visibleFrom)}\\|`),
    );
  });

  it("names no upgrade on the longest window", async () => {
    mount(
      client({
        plan: "scale",
        days: 30,
        visibleFrom: null,
        upgradePlan: null,
        upgradeDays: null,
      }),
    );
    await waitFor(() => expect(probe()).not.toBe("unbounded"));
    expect(probe()).toMatch(/\|Logs are kept for 30 days on the Scale plan\.$/);
  });

  it("reads again when the tab regains focus", async () => {
    const seen: Record<string, unknown>[] = [];
    mount(client(HOBBY, seen));
    await waitFor(() => expect(seen).toHaveLength(1));
    await act(async () => {
      window.dispatchEvent(new Event("focus"));
    });
    await waitFor(() => expect(seen).toHaveLength(2));
  });

  it("stays unbounded where no plan applies", async () => {
    const seen: Record<string, unknown>[] = [];
    mount(client(null, seen));
    await waitFor(() => expect(seen).toHaveLength(1));
    expect(probe()).toBe("unbounded");
  });
});
