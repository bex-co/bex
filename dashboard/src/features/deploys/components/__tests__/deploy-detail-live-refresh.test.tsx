import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ApolloClient, ApolloLink, InMemoryCache } from "@apollo/client";
import { ApolloProvider } from "@apollo/client/react";
import { Observable } from "rxjs";
import { apolloCacheConfig } from "@/common/apollo/cache";
import { formatDateTime } from "@/common/lib/format";
import { DeployDetailPage } from "../deploy-detail-page";

// w4/262: a held Live detail must notice a newer deploy replacing it. Real
// page, useDeploy, header and timeline over a real Apollo client; actions are
// stubbed to expose the status that gates Rollback.

vi.mock("../deploy-log-panel", () => ({ DeployLogPanel: () => null }));
vi.mock("../deploy-actions", () => ({
  DeployActions: ({ status }: { status: string }) => (
    <div data-testid="deploy-actions" data-status={status} />
  ),
}));

const SERVICE = "srv-db623dkkcuvc73d6ncu0";
const DEPLOY = "dep-db623dkkcuvc73d6ncug";
const LIVE_AT = "2026-10-11T23:56:48.850929Z";
const REPLACED_AT = "2026-10-12T00:02:06.569454Z";

interface Server {
  status: string;
  updatedAt: string;
  fail: boolean;
  deployReads: number;
}

function serverLink(server: Server) {
  return new ApolloLink(
    (op) =>
      new Observable((sub) => {
        if (op.operationName === "Deploy") {
          server.deployReads++;
          if (server.fail) {
            sub.error(new Error("503 Service Unavailable"));
            return;
          }
          sub.next({
            data: {
              deploy: {
                __typename: "Deploy",
                id: DEPLOY,
                status: server.status,
                trigger: "api",
                image: "",
                rollbackOf: "",
                commitId: "d59ed1b",
                commitMessage: "",
                commitCreatedAt: null,
                createdAt: "2026-10-11T23:55:00Z",
                updatedAt: server.updatedAt,
                startedAt: "2026-10-11T23:55:01Z",
                finishedAt: LIVE_AT,
                preDeployStatus: "",
                failureReason: "",
                cancelReason: "",
                stallReason: "",
              },
            },
          });
        } else if (op.operationName === "DeployTimelineEvents") {
          sub.next({ data: { serviceEvents: [] } });
        } else {
          sub.error(new Error(`unexpected ${op.operationName}`));
          return;
        }
        sub.complete();
      }),
  );
}

function renderPage(server: Server) {
  const client = new ApolloClient({
    link: serverLink(server),
    cache: new InMemoryCache(apolloCacheConfig),
  });
  return render(
    <ApolloProvider client={client}>
      <DeployDetailPage serviceId={SERVICE} deployId={DEPLOY} />
    </ApolloProvider>,
  );
}

const liveServer = (): Server => ({
  status: "live",
  updatedAt: LIVE_AT,
  fail: false,
  deployReads: 0,
});

const advance = (ms: number) =>
  act(async () => {
    await vi.advanceTimersByTimeAsync(ms);
  });

function expectShown(status: "Live" | "Deactivated") {
  expect(screen.getByTestId("deploy-actions")).toHaveAttribute(
    "data-status",
    status.toLowerCase(),
  );
  // The header badge and the timeline's terminal step.
  expect(screen.getAllByText(status)).toHaveLength(2);
  // The replacement instant: the header's Updated and the Deactivated step.
  const replaced = screen.queryAllByText(formatDateTime(REPLACED_AT)!);
  expect(replaced).toHaveLength(status === "Live" ? 0 : 2);
}

let hidden = false;

beforeEach(() => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  hidden = false;
  vi.spyOn(document, "hidden", "get").mockImplementation(() => hidden);
});

afterEach(() => {
  vi.restoreAllMocks();
  vi.useRealTimers();
});

describe("DeployDetailPage — a Live deploy replaced by a newer one (w4/262)", () => {
  it("turns Deactivated, with its later time and timeline step, within one baseline refresh", async () => {
    const server = liveServer();
    renderPage(server);
    await screen.findAllByText(formatDateTime(LIVE_AT)!);
    expectShown("Live");

    server.status = "deactivated";
    server.updatedAt = REPLACED_AT;
    await advance(29_000);
    expectShown("Live");

    await advance(1_500);
    expectShown("Deactivated");
  });

  it("keeps the last good detail with an inline retry when a refresh fails", async () => {
    const server = liveServer();
    renderPage(server);
    await screen.findAllByText(formatDateTime(LIVE_AT)!);

    server.fail = true;
    await advance(30_500);
    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent("Couldn't refresh this deploy.");
    expectShown("Live");

    server.fail = false;
    server.status = "deactivated";
    server.updatedAt = REPLACED_AT;
    await userEvent
      .setup({ advanceTimers: vi.advanceTimersByTime })
      .click(within(alert).getByRole("button", { name: "Try again" }));
    await advance(0);
    expect(screen.queryByRole("alert")).toBeNull();
    expectShown("Deactivated");
  });

  it("does not refresh while the tab is hidden", async () => {
    const server = liveServer();
    renderPage(server);
    await screen.findAllByText(formatDateTime(LIVE_AT)!);
    const reads = server.deployReads;

    hidden = true;
    await advance(65_000);
    expect(server.deployReads).toBe(reads);

    hidden = false;
    await advance(30_500);
    expect(server.deployReads).toBeGreaterThan(reads);
  });

  it("stops reading a settled Deactivated record", async () => {
    const server = liveServer();
    server.status = "deactivated";
    server.updatedAt = REPLACED_AT;
    renderPage(server);
    await screen.findAllByText(formatDateTime(REPLACED_AT)!);
    const reads = server.deployReads;

    await advance(120_000);
    expect(server.deployReads).toBe(reads);
  });
});
