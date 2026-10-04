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
import { useLogLabelDiscovery } from "../use-log-label-values";

// w4/178: discovery refreshes while the page stays open, keeps its last
// answer through a transient failure, and skips work while the tab is hidden.

type Reply = { values?: string[]; fail?: boolean };

function transport(replies: Reply[]) {
  let calls = 0;
  const client = new ApolloClient({
    cache: new InMemoryCache(),
    link: new ApolloLink(
      () =>
        new Observable<ApolloLink.Result>((observer) => {
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
  return { client, calls: () => calls };
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
