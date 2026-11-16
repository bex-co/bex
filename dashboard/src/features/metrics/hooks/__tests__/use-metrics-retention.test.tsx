import { afterEach, describe, expect, it } from "vitest";
import { act, cleanup, render, screen, waitFor } from "@testing-library/react";
import {
  ApolloClient,
  ApolloLink,
  InMemoryCache,
  Observable,
} from "@apollo/client";
import { ApolloProvider } from "@apollo/client/react";
import { useMetricsRetentionBound } from "../use-metrics-retention";

// w7/m427: the Metrics page bounds its range picker by the window bex-api's
// metricsRetention reports for the service, with a hint naming the plan's
// window and every plan that keeps more — no plan table in the client. The
// answer holds still, so it is read once and again when the tab regains focus.

type Retention = {
  plan: string;
  days: number;
  upgrades: { plan: string; days: number }[];
} | null;

function client(
  retention: Retention | "error",
  seen: Record<string, unknown>[] = [],
) {
  return new ApolloClient({
    cache: new InMemoryCache(),
    link: new ApolloLink(
      (operation) =>
        new Observable<ApolloLink.Result>((observer) => {
          seen.push(operation.variables);
          if (retention === "error") {
            observer.next({ data: null, errors: [{ message: "plan read" }] });
          } else {
            observer.next({
              data: {
                metricsRetention: retention && {
                  __typename: "MetricsRetention",
                  ...retention,
                  upgrades: retention.upgrades.map((upgrade) => ({
                    __typename: "MetricsRetentionUpgrade",
                    ...upgrade,
                  })),
                },
              },
            });
          }
          observer.complete();
        }),
    ),
  });
}

function Probe({ resource }: { resource: string }) {
  const bound = useMetricsRetentionBound(resource);
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

const PRO = { plan: "pro", days: 14 };
const SCALE = { plan: "scale", days: 30 };
const HOBBY: Retention = { plan: "hobby", days: 7, upgrades: [PRO, SCALE] };
const DAY = 24 * 60 * 60;

describe("useMetricsRetentionBound", () => {
  it("bounds a Hobby workspace's metrics to 7 days and names Pro's 14 and Scale's 30", async () => {
    const seen: Record<string, unknown>[] = [];
    mount(client(HOBBY, seen), "srv-x");
    await waitFor(() => expect(probe()).not.toBe("unbounded"));
    // No kept floor: the current plan's window alone applies to metrics.
    expect(probe()).toBe(
      `${7 * DAY}|-|Metrics are kept for 7 days on the Hobby plan. The Pro plan keeps 14 days. The Scale plan keeps 30 days.`,
    );
    expect(seen[0]).toEqual({ resource: "srv-x" });
  });

  it("names only Scale on Pro, and no plan on the longest window", async () => {
    mount(client({ ...PRO, upgrades: [SCALE] }));
    await waitFor(() => expect(probe()).not.toBe("unbounded"));
    expect(probe()).toBe(
      `${14 * DAY}|-|Metrics are kept for 14 days on the Pro plan. The Scale plan keeps 30 days.`,
    );
    cleanup();

    mount(client({ ...SCALE, upgrades: [] }));
    await waitFor(() => expect(probe()).not.toBe("unbounded"));
    expect(probe()).toBe(
      `${30 * DAY}|-|Metrics are kept for 30 days on the Scale plan.`,
    );
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

  it("stays unbounded where no plan applies, and when the read fails", async () => {
    const seen: Record<string, unknown>[] = [];
    mount(client(null, seen));
    await waitFor(() => expect(seen).toHaveLength(1));
    expect(probe()).toBe("unbounded");
    cleanup();

    const failed: Record<string, unknown>[] = [];
    mount(client("error", failed));
    await waitFor(() => expect(failed).toHaveLength(1));
    expect(probe()).toBe("unbounded");
  });
});
