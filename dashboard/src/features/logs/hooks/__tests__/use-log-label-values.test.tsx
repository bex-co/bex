import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, render, screen } from "@testing-library/react";
import {
  ApolloClient,
  ApolloLink,
  InMemoryCache,
  Observable,
} from "@apollo/client";
import { ApolloProvider } from "@apollo/client/react";
import { RESOURCE_POLL_INTERVAL_MS } from "@/common/lib/polling";
import {
  discoveryStartTime,
  useLogLabelDiscovery,
  useProjectLogLabelDiscovery,
} from "../use-log-label-values";

// w4/178: discovery refreshes while the page stays open, keeps its last
// answer through a transient failure, and skips work while the tab is hidden.

type Reply = { values?: string[]; fail?: boolean };

function transport(replies: Reply[]) {
  let calls = 0;
  const variables: Record<string, unknown>[] = [];
  const client = new ApolloClient({
    cache: new InMemoryCache(),
    link: new ApolloLink(
      (operation) =>
        new Observable<ApolloLink.Result>((observer) => {
          variables.push(operation.variables);
          const reply = replies[Math.min(calls, replies.length - 1)];
          calls++;
          if (reply.fail) {
            observer.error(new Error("log store unavailable"));
            return;
          }
          observer.next({ data: { logLabelValues: reply.values ?? [] } });
          observer.complete();
        }),
    ),
  });
  return { client, calls: () => calls, variables };
}

function Probe() {
  const { values, resolved } = useLogLabelDiscovery("srv-1", "method");
  return (
    <p data-testid="probe">
      {resolved ? "resolved" : "unresolved"}:{values.join(",")}
    </p>
  );
}

function mount(client: ApolloClient) {
  render(
    <ApolloProvider client={client}>
      <Probe />
    </ApolloProvider>,
  );
}

async function tick(ms: number) {
  await act(async () => {
    await vi.advanceTimersByTimeAsync(ms);
  });
}

const probe = () => screen.getByTestId("probe").textContent;

beforeEach(() => {
  vi.useFakeTimers();
});
afterEach(() => {
  cleanup();
  vi.useRealTimers();
  Object.defineProperty(document, "hidden", {
    configurable: true,
    value: false,
  });
});

describe("useLogLabelDiscovery freshness", () => {
  it("offers a method first seen after mount on the next poll", async () => {
    const { client } = transport([
      { values: [] },
      { values: ["GET"] },
      { values: ["GET", "OPTIONS"] },
    ]);
    mount(client);
    await tick(0);
    expect(probe()).toBe("resolved:");
    await tick(RESOURCE_POLL_INTERVAL_MS);
    expect(probe()).toBe("resolved:GET");
    await tick(RESOURCE_POLL_INTERVAL_MS);
    expect(probe()).toBe("resolved:GET,OPTIONS");
  });

  it("keeps the last answer through a transient poll failure", async () => {
    const { client } = transport([
      { values: ["GET", "TRACE"] },
      { fail: true },
      { values: ["GET", "TRACE", "HEAD"] },
    ]);
    mount(client);
    await tick(0);
    expect(probe()).toBe("resolved:GET,TRACE");
    await tick(RESOURCE_POLL_INTERVAL_MS);
    expect(probe()).toBe("resolved:GET,TRACE");
    await tick(RESOURCE_POLL_INTERVAL_MS);
    expect(probe()).toBe("resolved:GET,TRACE,HEAD");
  });

  it("stays unresolved when discovery has never answered", async () => {
    const { client } = transport([{ fail: true }]);
    mount(client);
    await tick(0);
    expect(probe()).toBe("unresolved:");
  });

  it("skips polls while the tab is hidden and resumes when visible", async () => {
    const { client, calls } = transport([
      { values: ["GET"] },
      { values: ["GET", "POST"] },
    ]);
    mount(client);
    await tick(0);
    expect(calls()).toBe(1);
    Object.defineProperty(document, "hidden", {
      configurable: true,
      value: true,
    });
    await tick(RESOURCE_POLL_INTERVAL_MS * 3);
    expect(calls()).toBe(1);
    expect(probe()).toBe("resolved:GET");
    Object.defineProperty(document, "hidden", {
      configurable: true,
      value: false,
    });
    await tick(RESOURCE_POLL_INTERVAL_MS);
    expect(calls()).toBe(2);
    expect(probe()).toBe("resolved:GET,POST");
  });
});

// w5/197: a logLabelValues read without a startTime covers only the last hour,
// as Render's does, so discovery names its own window: the shortest plan's
// seven days, rounded up to the hour so polls within an hour share one query.
describe("useLogLabelDiscovery window", () => {
  it("rounds the seven-day start up to the hour", () => {
    expect(discoveryStartTime(Date.parse("2026-11-19T12:34:56Z"))).toBe(
      "2026-11-12T13:00:00.000Z",
    );
    expect(discoveryStartTime(Date.parse("2026-11-19T12:00:00Z"))).toBe(
      "2026-11-12T12:00:00.000Z",
    );
  });

  it("asks for seven days and keeps the start across polls within the hour", async () => {
    vi.setSystemTime(new Date("2026-11-19T12:34:56Z"));
    const { client, variables } = transport([{ values: ["GET"] }]);
    mount(client);
    await tick(0);
    await tick(RESOURCE_POLL_INTERVAL_MS);
    expect(variables.length).toBeGreaterThanOrEqual(2);
    for (const v of variables) {
      expect(v).toEqual({
        resource: "srv-1",
        label: "method",
        startTime: "2026-11-12T13:00:00.000Z",
      });
    }
  });
});

// w3/079: the Edit view's Instance picker reads the project's values once,
// over the same seven days, for the view's log type.
describe("useProjectLogLabelDiscovery", () => {
  function ProjectProbe() {
    const { values, status } = useProjectLogLabelDiscovery(
      "prj-1",
      "instance",
      "request",
    );
    return (
      <p data-testid="probe">
        {status}:{values.join(",")}
      </p>
    );
  }

  function mountProject(client: ApolloClient) {
    render(
      <ApolloProvider client={client}>
        <ProjectProbe />
      </ApolloProvider>,
    );
  }

  it("reads the project's values for the view's log type, once", async () => {
    vi.setSystemTime(new Date("2026-11-19T12:34:56Z"));
    const { client, calls, variables } = transport([
      { values: ["srv-1-a", "dpg-2-b"] },
    ]);
    mountProject(client);
    expect(probe()).toBe("loading:");
    await tick(0);
    expect(probe()).toBe("ready:srv-1-a,dpg-2-b");
    expect(variables).toEqual([
      {
        project: "prj-1",
        label: "instance",
        type: "request",
        startTime: "2026-11-12T13:00:00.000Z",
      },
    ]);
    await tick(RESOURCE_POLL_INTERVAL_MS * 3);
    expect(calls()).toBe(1);
  });

  it("reports a read that failed", async () => {
    const { client } = transport([{ fail: true }]);
    mountProject(client);
    await tick(0);
    expect(probe()).toBe("failed:");
  });
});
