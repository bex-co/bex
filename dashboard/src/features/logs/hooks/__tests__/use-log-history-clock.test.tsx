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
import { useLiveRange } from "@/features/metrics/hooks/use-live-range";
import type { RangeSelection } from "@/features/metrics/lib/range";
import { setupVirtualGeometry, scrollViewport } from "@/test/virtual-geometry";
import { LogLineList } from "../../components/log-line-list";
import { EMPTY_LOG_FILTERS } from "../../types";
import { useLogHistory } from "../use-log-history";
const NOW = Date.parse("2026-10-02T12:00:00Z");
const RANGE: RangeSelection = {
  id: "1h",
  spanSeconds: 3600,
  resolutionSeconds: 30,
};
function rows(start: number, count: number, age = 600_000) {
  return Array.from({ length: count }, (_, i) => ({
    __typename: "LogEntry" as const,
    timestamp: new Date(NOW - age + (start + i) * 1000).toISOString(),
    message: `line ${start + i}`,
    type: "app",
    instance: "pod-1",
    level: null,
    method: null,
    statusCode: null,
  }));
}
function envelope(logs: ReturnType<typeof rows>, hasMore = false) {
  return {
    logs: {
      __typename: "LogList",
      logs,
      hasMore,
      nextStartTime: new Date(NOW - 3600_000).toISOString(),
      nextEndTime: logs[0]?.timestamp ?? "",
    },
  };
}
function transport() {
  const requests: {
    variables: Record<string, unknown>;
    reply: (data: ReturnType<typeof envelope>) => void;
    fail: () => void;
    deny: (code: string) => void;
  }[] = [];
  const client = new ApolloClient({
    cache: new InMemoryCache(),
    link: new ApolloLink(
      (operation) =>
        new Observable<ApolloLink.Result>((observer) => {
          requests.push({
            variables: operation.variables,
            fail: () => observer.error(new Error("temporary network outage")),
            deny: (code) => {
              observer.next({
                errors: [{ message: "access denied", extensions: { code } }],
              });
              observer.complete();
            },
            reply: (data) => {
              observer.next({ data });
              observer.complete();
            },
          });
        }),
    ),
  });
  return { client, requests };
}
function Reader({
  resource = "srv-one",
  text = "",
  range = RANGE,
}: {
  resource?: string;
  text?: string;
  range?: RangeSelection;
}) {
  const window = useLiveRange(range);
  const history = useLogHistory(
    resource,
    { ...EMPTY_LOG_FILTERS, text },
    window,
    JSON.stringify(range),
  );
  return (
    <>
      <output data-testid="count">{history.lines.length}</output>
      <output data-testid="messages">
        {history.lines.map((line) => line.message).join(",")}
      </output>
      <output data-testid="error">{history.error?.message}</output>
      <output data-testid="more">{String(history.hasMore)}</output>
      <button onClick={history.loadOlder}>Older</button>
      {history.loading ? (
        <div data-testid="loading" />
      ) : (
        <LogLineList lines={history.lines} />
      )}
    </>
  );
}
async function flush() {
  await act(async () => {
    await vi.advanceTimersByTimeAsync(1);
  });
}
async function reply(
  request: ReturnType<typeof transport>["requests"][number],
  data: ReturnType<typeof envelope>,
) {
  await act(async () => {
    request.reply(data);
    await vi.advanceTimersByTimeAsync(1);
  });
}
function count(n: number) {
  expect(screen.getByTestId("count").textContent).toBe(String(n));
}
describe("relative history with real Apollo and virtual reader", () => {
  setupVirtualGeometry();
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(NOW);
  });
  afterEach(() => {
    cleanup();
    vi.clearAllTimers();
    vi.useRealTimers();
  });
  it("retains 100+40 rows and unpinned viewport through two delayed clock heads", async () => {
    const server = transport();
    const { container } = render(
      <ApolloProvider client={server.client}>
        <Reader />
      </ApolloProvider>,
    );
    await flush();
    await reply(server.requests[0], envelope(rows(40, 100), true));
    fireEvent.click(screen.getByText("Older"));
    await flush();
    await reply(server.requests[1], envelope(rows(0, 41)));
    count(140);
    const viewport = container.querySelector<HTMLElement>(
      "[data-log-viewport]",
    )!;
    act(() =>
      scrollViewport(viewport, {
        scrollTop: 900,
        scrollHeight: 140 * 24 + 24,
        clientHeight: 520,
      }),
    );
    const position = viewport.scrollTop;
    expect(position).toBe(900);
    const anchor = container.querySelector("[data-index]")?.textContent;
    expect(anchor).toBeTruthy();
    for (let tick = 0; tick < 2; tick++) {
      await act(async () => {
        await vi.advanceTimersByTimeAsync(30_000);
      });
      count(140);
      expect(screen.queryByTestId("loading")).not.toBeInTheDocument();
      expect(container.querySelector("[data-log-viewport]")).toBe(viewport);
      expect(viewport.scrollTop).toBe(position);
      await reply(server.requests.at(-1)!, envelope(rows(40, 100), true));
      count(140);
      expect(screen.getByTestId("more")).toHaveTextContent("false");
      expect(viewport.scrollTop).toBe(position);
      expect(container.querySelector("[data-index]")?.textContent).toBe(anchor);
    }
    expect(server.requests).toHaveLength(4);
    server.client.stop();
  });
  it("accepts older completion across a tick and excludes rows expired by its moving lower bound", async () => {
    const server = transport();
    render(
      <ApolloProvider client={server.client}>
        <Reader />
      </ApolloProvider>,
    );
    await flush();
    await reply(server.requests[0], envelope(rows(40, 100), true));
    fireEvent.click(screen.getByText("Older"));
    await flush();
    const older = server.requests[1];
    await act(async () => {
      await vi.advanceTimersByTimeAsync(30_000);
    });
    await reply(older, envelope([...rows(0, 40), ...rows(0, 1, 3590_000)]));
    count(140);
    expect(screen.getByTestId("more")).toHaveTextContent("false");
    await reply(server.requests[2], envelope(rows(40, 100), true));
    count(140);
    expect(screen.getByTestId("more")).toHaveTextContent("false");
    server.client.stop();
  });
  it.each(["network", "FORBIDDEN", "NOT_FOUND"])(
    "handles background %s without false empty success or stale access",
    async (failure) => {
      const server = transport();
      const { container } = render(
        <ApolloProvider client={server.client}>
          <Reader />
        </ApolloProvider>,
      );
      await flush();
      await reply(server.requests[0], envelope(rows(40, 100), true));
      const viewport = container.querySelector("[data-log-viewport]");
      await act(async () => {
        await vi.advanceTimersByTimeAsync(30_000);
      });
      await act(async () => {
        const request = server.requests.at(-1)!;
        if (failure === "network") request.fail();
        else request.deny(failure);
        await vi.advanceTimersByTimeAsync(1);
      });
      expect(screen.getByTestId("error").textContent).not.toBe("");
      count(failure === "network" ? 100 : 0);
      if (failure === "network")
        expect(container.querySelector("[data-log-viewport]")).toBe(viewport);
      else expect(screen.getByTestId("more")).toHaveTextContent("false");
      server.client.stop();
    },
  );

  it.each(["resource", "filter", "preset", "custom"])(
    "resets explicit %s and isolates late older responses",
    async (change) => {
      const server = transport();
      const view = (props: Parameters<typeof Reader>[0] = {}) => (
        <ApolloProvider client={server.client}>
          <Reader {...props} />
        </ApolloProvider>
      );
      const { rerender } = render(view());
      await flush();
      await reply(server.requests[0], envelope(rows(40, 100), true));
      fireEvent.click(screen.getByText("Older"));
      await flush();
      const older = server.requests[1];
      const props =
        change === "resource"
          ? { resource: "srv-two" }
          : change === "filter"
            ? { text: "new" }
            : change === "preset"
              ? {
                  range: {
                    id: "30m" as const,
                    spanSeconds: 1800,
                    resolutionSeconds: 15,
                  },
                }
              : {
                  range: {
                    id: "custom" as const,
                    startTime: new Date(NOW - 1800_000).toISOString(),
                    endTime: new Date(NOW).toISOString(),
                    resolutionSeconds: 15,
                  },
                };
      rerender(view(props));
      count(0);
      await flush();
      await reply(older, envelope(rows(0, 40)));
      count(0);
      await reply(server.requests.at(-1)!, envelope(rows(200, 1)));
      count(1);
      expect(screen.getByTestId("messages")).toHaveTextContent(/^line 200$/);
      server.client.stop();
    },
  );
});
