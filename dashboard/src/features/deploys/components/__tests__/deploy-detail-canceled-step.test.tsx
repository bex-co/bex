import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { ApolloClient, ApolloLink, InMemoryCache } from "@apollo/client";
import { ApolloProvider } from "@apollo/client/react";
import { Observable } from "rxjs";
import { apolloCacheConfig } from "@/common/apollo/cache";
import { setupVirtualGeometry } from "@/test/virtual-geometry";
import { DeployDetailPage } from "../deploy-detail-page";
import type { LogRange } from "../../lib/log-range";

// w4/m198: a canceled deploy's pre-deploy step kept running and printed its
// final line after the cancellation. The real page, useDeploy, step-end
// resolver, log panel and three typed log legs run over a real Apollo client;
// only the server is fake — and it filters logs by the requested window exactly
// as bex-api does, so the default window's end bound is what's under test.

vi.mock("../deploy-header", () => ({
  DeployHeader: ({ deploy }: { deploy: { finishedAt: string } | null }) => (
    <div data-testid="deploy-header" data-finished={deploy?.finishedAt ?? ""} />
  ),
}));
vi.mock("../deploy-actions", () => ({ DeployActions: () => null }));
vi.mock("../deploy-timeline", () => ({ DeployTimeline: () => null }));

setupVirtualGeometry();

const SERVICE = "srv-db5glu8k3t2s73e5vb2g";
const POD = `${SERVICE}-nvej7lbkin5snjptmhao`;

interface ServerDeploy {
  id: string;
  status: string;
  preDeployStatus: string;
  createdAt: string;
  updatedAt: string;
  finishedAt: string;
}
interface Line {
  timestamp: string;
  message: string;
  type: "predeploy" | "app";
}
interface Server {
  deploy: ServerDeploy;
  facts: { id: string; deployId: string; at: string }[];
  lines: Line[];
  logRequests: Record<string, unknown>[];
}

function serverLink(server: Server) {
  const within = (at: string, vars: Record<string, unknown>) => {
    const t = Date.parse(at);
    const start = vars.startTime
      ? Date.parse(String(vars.startTime))
      : -Infinity;
    const end = vars.endTime ? Date.parse(String(vars.endTime)) : Infinity;
    return t >= start && t <= end;
  };
  return new ApolloLink(
    (op) =>
      new Observable((sub) => {
        const vars = op.variables as Record<string, unknown>;
        let data: Record<string, unknown>;
        if (op.operationName === "Deploy") {
          data = {
            deploy: {
              __typename: "Deploy",
              trigger: "api",
              image: "docker.io/library/busybox:1.36",
              rollbackOf: "",
              commitId: "",
              commitMessage: "",
              commitCreatedAt: null,
              startedAt: null,
              failureReason: "",
              cancelReason: "",
              stallReason: "",
              ...server.deploy,
            },
          };
        } else if (op.operationName === "DeployPreDeployEnd") {
          data = {
            serviceEvents: server.facts
              .filter((f) => within(f.at, vars))
              .map((f) => ({
                __typename: "ServiceEvent",
                id: f.id,
                timestamp: f.at,
                cursor: f.id,
                details: {
                  __typename: "ServiceEventDetails",
                  deployId: f.deployId,
                },
              })),
          };
        } else if (op.operationName === "Logs") {
          server.logRequests.push(vars);
          // A pre-deploy line is also durable build output (w4/180).
          const types =
            vars.type === "build" ? ["predeploy"] : [String(vars.type)];
          data = {
            logs: {
              __typename: "LogList",
              hasMore: false,
              nextStartTime: null,
              nextEndTime: null,
              logs: server.lines
                .filter(
                  (l) => types.includes(l.type) && within(l.timestamp, vars),
                )
                .map((l) => ({
                  __typename: "LogEntry",
                  timestamp: l.timestamp,
                  message: l.message,
                  type: vars.type,
                  instance: POD,
                  level: null,
                  method: null,
                  statusCode: null,
                })),
            },
          };
        } else {
          sub.error(new Error(`unexpected ${op.operationName}`));
          return;
        }
        sub.next({ data });
        sub.complete();
      }),
  );
}

function renderPage(server: Server, range?: LogRange) {
  const client = new ApolloClient({
    link: serverLink(server),
    cache: new InMemoryCache(apolloCacheConfig),
  });
  const wrap = (node: ReactNode) => (
    <ApolloProvider client={client}>{node}</ApolloProvider>
  );
  const page = (r?: LogRange) =>
    wrap(
      <DeployDetailPage
        serviceId={SERVICE}
        deployId={server.deploy.id}
        range={r}
        onRangeChange={() => {}}
      />,
    );
  const view = render(page(range));
  return { ...view, rerenderRange: (r?: LogRange) => view.rerender(page(r)) };
}

// finding.md's successful-step replay (dep-db5gnf0k3t2s73e5vb5g).
const successServer = (): Server => ({
  deploy: {
    id: "dep-db5gnf0k3t2s73e5vb5g",
    status: "canceled",
    preDeployStatus: "succeeded",
    createdAt: "2026-10-11T04:08:28.610523Z",
    updatedAt: "2026-10-11T04:09:58.252713Z",
    finishedAt: "2026-10-11T04:09:01.5668Z",
  },
  facts: [
    {
      id: "evt-sagp2f8t5g64q0ugvs7j",
      deployId: "dep-db5gnf0k3t2s73e5vb5g",
      at: "2026-10-11T04:09:17Z",
    },
  ],
  lines: [
    {
      timestamp: "2026-10-11T04:08:29.772991661Z",
      message: "qa-a147-cancel-start",
      type: "predeploy",
    },
    {
      timestamp: "2026-10-11T04:09:14.773261156Z",
      message: "qa-a147-cancel-complete",
      type: "predeploy",
    },
  ],
  logRequests: [],
});

// finding.md's error-step replay (dep-db5gq372659s73bbul30), plus a later
// deploy's step end and output that must not widen this viewer.
const failureServer = (): Server => ({
  deploy: {
    id: "dep-db5gq372659s73bbul30",
    status: "canceled",
    preDeployStatus: "failed",
    createdAt: "2026-10-11T04:14:04.170318Z",
    updatedAt: "2026-10-11T04:14:58.242833Z",
    finishedAt: "2026-10-11T04:14:23.441207Z",
  },
  facts: [
    {
      id: "evt-later",
      deployId: "dep-later",
      at: "2026-10-11T04:14:55Z",
    },
    {
      id: "evt-su4fhv4sghiftsoq5dtr",
      deployId: "dep-db5gq372659s73bbul30",
      at: "2026-10-11T04:14:42Z",
    },
  ],
  lines: [
    {
      timestamp: "2026-10-11T04:14:05.414283981Z",
      message: "qa-a147-cancel-failure-start",
      type: "predeploy",
    },
    {
      timestamp: "2026-10-11T04:14:40.414394093Z",
      message: "qa-a147-cancel-failure-end",
      type: "predeploy",
    },
    {
      timestamp: "2026-10-11T04:14:50.1Z",
      message: "qa-later-deploy-output",
      type: "predeploy",
    },
  ],
  logRequests: [],
});

async function search(text: string) {
  const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
  const input = screen.getByPlaceholderText(/search/i);
  await user.clear(input);
  await user.type(input, text);
}

const count = (text: string) => screen.queryAllByText(text).length;

beforeEach(() => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
});

afterEach(() => {
  vi.useRealTimers();
});

describe("DeployDetailPage — canceled deploy's continued pre-deploy step", () => {
  it("shows the step's post-cancel completion once in the default window", async () => {
    const server = successServer();
    renderPage(server);

    await screen.findByText("qa-a147-cancel-complete");
    await search("qa-a147-cancel-complete");
    await waitFor(() => expect(count("qa-a147-cancel-start")).toBe(0));
    expect(count("qa-a147-cancel-complete")).toBe(1);

    // The deploy still reads as canceled at the cancellation instant.
    expect(screen.getByTestId("deploy-header")).toHaveAttribute(
      "data-finished",
      "2026-10-11T04:09:01.5668Z",
    );
    // No leg ever read the known-truncated cancellation bound.
    const ends = new Set(server.logRequests.map((v) => v.endTime));
    expect(ends).toEqual(new Set(["2026-10-11T04:09:18.000Z"]));
    expect(
      server.logRequests.every((v) => v.startTime === server.deploy.createdAt),
    ).toBe(true);
  });

  it("keeps the failed step's final error marker across a range switch and back", async () => {
    vi.setSystemTime(new Date("2026-10-11T04:20:00Z"));
    const server = failureServer();
    const { rerenderRange } = renderPage(server);

    await screen.findByText("qa-a147-cancel-failure-end");
    // Another deploy's later step end and output stay out of this window.
    expect(count("qa-later-deploy-output")).toBe(0);
    await search("qa-a147-cancel-failure-end");
    await waitFor(() => expect(count("qa-a147-cancel-failure-start")).toBe(0));
    expect(count("qa-a147-cancel-failure-end")).toBe(1);

    rerenderRange("15m");
    await waitFor(() =>
      expect(server.logRequests.some((v) => v.endTime == null)).toBe(true),
    );
    await waitFor(() => expect(count("qa-a147-cancel-failure-end")).toBe(1));

    rerenderRange(undefined);
    await waitFor(() => expect(count("qa-a147-cancel-failure-end")).toBe(1));
    expect(server.logRequests.at(-1)?.endTime).toBe("2026-10-11T04:14:43.000Z");
  });

  it("widens the window when the step's verdict settles after the page loaded", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    // Viewed while the step still runs after the cancel.
    vi.setSystemTime(new Date("2026-10-11T04:14:30Z"));
    const server = failureServer();
    server.deploy = {
      ...server.deploy,
      preDeployStatus: "canceled",
      updatedAt: server.deploy.finishedAt,
    };
    const facts = server.facts;
    server.facts = [];
    renderPage(server);

    await screen.findByText("qa-a147-cancel-failure-start");
    expect(count("qa-a147-cancel-failure-end")).toBe(0);

    // The step finishes and its verdict + fact settle server-side.
    server.deploy = {
      ...server.deploy,
      preDeployStatus: "failed",
      updatedAt: "2026-10-11T04:14:58.242833Z",
    };
    server.facts = facts;
    await act(async () => {
      vi.setSystemTime(new Date("2026-10-11T04:15:00Z"));
      await vi.advanceTimersByTimeAsync(3500);
    });
    await screen.findByText("qa-a147-cancel-failure-end");
    expect(count("qa-a147-cancel-failure-end")).toBe(1);
  });

  it("keeps the cancellation bound with an explicit notice when no step end was recorded", async () => {
    const server = successServer();
    server.facts = [];
    renderPage(server);

    await screen.findByText("qa-a147-cancel-start");
    expect(
      screen.getByText(/step's completion time wasn't recorded/),
    ).toBeInTheDocument();
    expect(count("qa-a147-cancel-complete")).toBe(0);
    expect(server.logRequests.at(-1)?.endTime).toBe(server.deploy.finishedAt);
  });

  it("keeps the normal createdAt..finishedAt window for a live deploy", async () => {
    const server = successServer();
    server.deploy = { ...server.deploy, status: "live" };
    renderPage(server);

    await screen.findByText("qa-a147-cancel-start");
    expect(new Set(server.logRequests.map((v) => v.endTime))).toEqual(
      new Set([server.deploy.finishedAt]),
    );
  });
});
