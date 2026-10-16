import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  act,
  cleanup,
  fireEvent,
  render,
  screen,
} from "@testing-library/react";
import {
  ApolloClient,
  ApolloLink,
  InMemoryCache,
  Observable,
} from "@apollo/client";
import { ApolloProvider } from "@apollo/client/react";
import type { ReactNode } from "react";
import { parseCustomRange, RANGE_PRESETS } from "@/features/metrics/lib/range";
import { setupVirtualGeometry } from "@/test/virtual-geometry";
import { DatastoreLogViewer } from "../datastore-log-viewer";

// Instance discovery is its own query; keep the transport to the Logs reader.
vi.mock("../../hooks/use-log-label-values", () => ({
  useLogLabelValues: () => ["dpg-x-1"],
}));

const T0 = Date.parse("2026-10-16T02:41:34Z");
const HOUR = RANGE_PRESETS.find((preset) => preset.id === "1h")!;

function transport() {
  const requests: {
    variables: Record<string, unknown>;
    reply: (messages: [number, string][]) => void;
  }[] = [];
  const client = new ApolloClient({
    cache: new InMemoryCache(),
    link: new ApolloLink(
      (operation) =>
        new Observable<ApolloLink.Result>((observer) => {
          requests.push({
            variables: operation.variables,
            reply: (messages) => {
              observer.next({
                data: {
                  logs: {
                    __typename: "LogList",
                    hasMore: false,
                    nextStartTime: "",
                    nextEndTime: "",
                    logs: messages.map(([at, message]) => ({
                      __typename: "LogEntry",
                      timestamp: new Date(at).toISOString(),
                      message,
                      type: "postgres",
                      instance: "dpg-x-1",
                      level: "error",
                      method: null,
                      statusCode: null,
                    })),
                  },
                },
              });
              observer.complete();
            },
          });
        }),
    ),
  });
  return { client, requests };
}

async function settle(ms = 1) {
  await act(async () => {
    await vi.advanceTimersByTimeAsync(ms);
  });
}

function endOf(request: { variables: Record<string, unknown> }) {
  return Date.parse(String(request.variables.endTime));
}

// w4/319: a datastore Logs page opened at T0 and searched at T0+15s for an
// error stored at T0+10s used to send the search with the page's T0 cutoff and
// report "No matching logs". The first request for a newly committed search,
// instance or resource now ends at that commit; an unchanged reader and an
// explicit Custom range keep their windows.
describe("DatastoreLogViewer window per committed reader (w4/319)", () => {
  setupVirtualGeometry();
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(T0);
  });
  afterEach(() => {
    cleanup();
    vi.clearAllTimers();
    vi.useRealTimers();
  });

  function mount(
    server: ReturnType<typeof transport>,
    node: ReactNode,
  ): ReturnType<typeof render> {
    return render(
      <ApolloProvider client={server.client}>{node}</ApolloProvider>,
    );
  }

  it("ends a new search's first request at its commit and shows the stored line", async () => {
    const server = transport();
    mount(
      server,
      <DatastoreLogViewer kind="databases" resource="dpg-x" range={HOUR} />,
    );
    await settle();
    expect(endOf(server.requests[0])).toBe(T0);
    server.requests[0].reply([]);
    await settle();

    vi.setSystemTime(T0 + 15_000);
    fireEvent.change(screen.getByRole("textbox"), {
      target: { value: "qa_a662_error_delta" },
    });
    await settle(300); // the search debounce
    const search = server.requests.at(-1)!;
    expect(search.variables.text).toBe("qa_a662_error_delta");
    expect(endOf(search)).toBeGreaterThanOrEqual(T0 + 15_000);
    expect(endOf(search) - Date.parse(String(search.variables.startTime))).toBe(
      3600_000,
    );
    await act(async () => {
      search.reply([[T0 + 10_000, 'column "qa_a662_error_delta" does not exist']]);
      await vi.advanceTimersByTimeAsync(1);
    });
    expect(
      screen.getByText(/qa_a662_error_delta" does not exist/),
    ).toBeInTheDocument();

    // Clearing the search is a new reader too.
    vi.setSystemTime(T0 + 60_000);
    fireEvent.change(screen.getByRole("textbox"), { target: { value: "" } });
    await settle(300);
    expect(endOf(server.requests.at(-1)!)).toBeGreaterThanOrEqual(T0 + 60_000);
  });

  it("resamples on a new instance or resource, and not for an unchanged reader", async () => {
    const server = transport();
    const view = (resource: string, instance: string) => (
      <ApolloProvider client={server.client}>
        <DatastoreLogViewer
          kind="databases"
          resource={resource}
          range={HOUR}
          filters={{ text: "", instance }}
          onFiltersChange={() => undefined}
        />
      </ApolloProvider>
    );
    const { rerender } = render(view("dpg-x", ""));
    await settle();
    server.requests[0].reply([]);
    await settle();

    vi.setSystemTime(T0 + 20_000);
    rerender(view("dpg-x", ""));
    await settle();
    expect(server.requests).toHaveLength(1); // same reader: no new window

    rerender(view("dpg-x", "dpg-x-1"));
    await settle();
    expect(server.requests.at(-1)!.variables.instance).toEqual(["dpg-x-1"]);
    expect(endOf(server.requests.at(-1)!)).toBeGreaterThanOrEqual(T0 + 20_000);
    server.requests.at(-1)!.reply([]);
    await settle();

    vi.setSystemTime(T0 + 40_000);
    rerender(view("dpg-y", "dpg-x-1"));
    await settle();
    expect(server.requests.at(-1)!.variables.resource).toBe("dpg-y");
    expect(endOf(server.requests.at(-1)!)).toBeGreaterThanOrEqual(T0 + 40_000);
  });

  it("keeps an explicit Custom range's bounds when the search changes", async () => {
    const custom = parseCustomRange(
      "2026-10-16T02:00:00Z",
      "2026-10-16T02:40:00Z",
    );
    if (!custom) throw new Error("custom range rejected");
    const server = transport();
    mount(
      server,
      <DatastoreLogViewer kind="keyvalue" resource="red-x" range={custom} />,
    );
    await settle();
    server.requests[0].reply([]);
    await settle();
    vi.setSystemTime(T0 + 30_000);
    fireEvent.change(screen.getByRole("textbox"), {
      target: { value: "epsilon" },
    });
    await settle(300);
    const search = server.requests.at(-1)!;
    expect(search.variables.text).toBe("epsilon");
    expect(search.variables.startTime).toBe(custom.startTime);
    expect(search.variables.endTime).toBe(custom.endTime);
  });
});
