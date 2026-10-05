import {
  afterEach,
  beforeEach,
  describe,
  expect,
  it,
  onTestFinished,
  vi,
} from "vitest";
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
import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  RouterProvider,
} from "@tanstack/react-router";
import { ServiceEventsPage } from "../services.$serviceId.events";
import { ServiceBaseProvider } from "@/features/services/lib/service-base";
import { apolloDefaultOptions } from "@/common/apollo/default-options";
import type { ServiceEventsQueryVariables } from "@/graphql/definitions";

vi.mock("@/features/services/hooks/use-server", () => ({
  useServer: () => ({ service: null, loading: false }),
}));
vi.mock("@/common/hooks/use-workspace-subject-label", () => ({
  useWorkspaceSubjectLabel: (subject: string) => subject,
}));
vi.mock("@/features/capabilities/hooks/use-resource-actions", async () => {
  const { mockAllowedResourceActions } =
    await import("@/test/mocks/resource-actions");
  return mockAllowedResourceActions("app");
});
vi.mock("@/features/workspaces/context/hooks", async () => {
  const { mockWorkspaceContext } = await import("@/test/mocks/workspace");
  return mockWorkspaceContext();
});

const mountedAt = Date.parse("2026-10-02T12:00:00Z");
const maxWindowMs = 720 * 60 * 60 * 1000;

function event(id: string, type: string, offset: number, status = "") {
  return {
    __typename: "ServiceEvent",
    id,
    type,
    timestamp: new Date(mountedAt + offset).toISOString(),
    cursor: id,
    details: {
      __typename: "ServiceEventDetails",
      deployId: type.startsWith("deploy_") ? "dep-live" : "",
      deployStatus: status,
      preDeployStatus: "",
      fullDeployStatus: "",
      failureReason: "",
      cancelReason: "",
      stallReason: "",
      status: "",
      actor: "",
      triggeredByUser: "",
      image: "",
      commitId: "",
      commitMessage: "",
      startedAt: "",
      finishedAt: "",
      reasonCode: "",
      exitCode: null as number | null,
      instanceId: "",
      fromCount: null,
      toCount: null,
      branchFrom: "",
      branchTo: "",
      commitUrl: "",
      projectFrom: null,
      projectTo: null,
      environmentFrom: null,
      environmentTo: null,
      trigger: null,
    },
  };
}

function mountLiveEvents(base: "services" | "static" = "services") {
  const available = [event("evt-start", "deploy_started", -1000)];
  const requests: ServiceEventsQueryVariables[] = [];
  let fail = false;
  const client = new ApolloClient({
    cache: new InMemoryCache(),
    defaultOptions: apolloDefaultOptions,
    link: new ApolloLink(
      (operation) =>
        new Observable((observer) => {
          if (operation.operationName !== "ServiceEvents") {
            observer.next({ data: { deployActions: [] } });
            observer.complete();
            return;
          }
          const variables = operation.variables as ServiceEventsQueryVariables;
          requests.push({ ...variables });
          if (fail) {
            observer.error(new Error("events unavailable"));
            return;
          }
          const start = Date.parse(variables.startTime);
          const end = Date.parse(variables.endTime);
          // The fake server enforces the real API cap and historical semantics.
          if (end - start > maxWindowMs) {
            observer.error(new Error("event range exceeds 720 hours"));
            return;
          }
          observer.next({
            data: {
              serviceEvents: available
                .filter(
                  (row) =>
                    Date.parse(row.timestamp) >= start &&
                    Date.parse(row.timestamp) <= end,
                )
                .sort((a, b) => b.timestamp.localeCompare(a.timestamp)),
            },
          });
          observer.complete();
        }),
    ),
  });
  onTestFinished(() => client.stop());
  const root = createRootRoute();
  const route = createRoute({
    getParentRoute: () => root,
    path: `/${base}/$serviceId/events`,
    component: () => (
      <ServiceBaseProvider value={base === "static" ? "/static" : "/services"}>
        <ServiceEventsPage serviceId="app" />
      </ServiceBaseProvider>
    ),
  });
  const router = createRouter({
    routeTree: root.addChildren([route]),
    history: createMemoryHistory({ initialEntries: [`/${base}/app/events`] }),
    context: { client, session: null },
  });
  const view = render(
    <ApolloProvider client={client}>
      <RouterProvider router={router} />
    </ApolloProvider>,
  );
  return {
    available,
    requests,
    setFailure: (value: boolean) => {
      fail = value;
    },
    ...view,
  };
}

async function tick(ms: number) {
  await act(async () => {
    await vi.advanceTimersByTimeAsync(ms);
  });
}

beforeEach(() => {
  vi.useFakeTimers();
  vi.setSystemTime(mountedAt);
  vi.spyOn(document, "hidden", "get").mockReturnValue(false);
});
afterEach(() => {
  cleanup();
  vi.useRealTimers();
  vi.restoreAllMocks();
});

describe("mounted Activity refresh", () => {
  it.each([
    ["services", "succeeded", "Live"],
    ["static", "succeeded", "Live"],
    ["services", "failed", "Failed"],
    ["services", "canceled", "Canceled"],
  ] as const)(
    "%s receives a later %s terminal event without reload",
    async (base, status, label) => {
      const page = mountLiveEvents(base);
      await tick(1);
      expect(screen.getByText("In Progress")).toBeInTheDocument();
      expect(
        screen.getByRole("button", { name: /^Cancel deploy / }),
      ).toBeInTheDocument();
      page.available.push(event("evt-build-start", "build_started", 2_000));
      page.available.push(event("evt-build-end", "build_ended", 7_000));
      page.available.push(event("evt-end", "deploy_ended", 10_000, status));
      await tick(30_000);
      expect(screen.getByText(label)).toBeInTheDocument();
      expect(screen.queryByText("In Progress")).not.toBeInTheDocument();
      expect(
        screen.queryByRole("button", { name: /^Cancel deploy / }),
      ).not.toBeInTheDocument();
      expect(screen.getAllByText("Deploy started")).toHaveLength(1);
      expect(screen.getAllByText("Deploy ended")).toHaveLength(1);
      expect(screen.getByText("Build started")).toBeInTheDocument();
      expect(screen.getByText("Build ended")).toBeInTheDocument();
      expect(
        page.requests.some(
          (request) => Date.parse(request.endTime) >= mountedAt + 10_000,
        ),
      ).toBe(true);
      for (const request of page.requests) {
        expect(
          Date.parse(request.endTime) - Date.parse(request.startTime),
        ).toBeLessThanOrEqual(maxWindowMs);
      }
    },
  );

  // w4/m114: a failed cron run says why it failed, with its exit status.
  it("shows why a cron run failed", async () => {
    const page = mountLiveEvents();
    await tick(1);
    const failed = event("evt-cron-end", "cron_job_run_ended", 5_000);
    failed.details.status = "failed";
    failed.details.reasonCode = "non_zero_exit";
    failed.details.exitCode = 3;
    page.available.push(failed);
    await tick(30_000);
    expect(screen.getByText("Cron run finished")).toBeInTheDocument();
    expect(
      screen.getByText("The run exited with status 3."),
    ).toBeInTheDocument();
  });

  it("keeps existing rows during a failed refresh and retries at the current time", async () => {
    const page = mountLiveEvents();
    await tick(1);
    page.setFailure(true);
    await tick(30_000);
    expect(screen.getByRole("alert")).toBeInTheDocument();
    expect(screen.getByText("Deploy started")).toBeInTheDocument();
    page.setFailure(false);
    page.available.push(event("evt-end", "deploy_ended", 40_000, "succeeded"));
    vi.setSystemTime(mountedAt + 45_000);
    fireEvent.click(screen.getByRole("button", { name: "Try again" }));
    await tick(1);
    expect(screen.getByText("Live")).toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    expect(page.requests.at(-1)?.endTime).toBe(
      new Date(mountedAt + 45_000).toISOString(),
    );
  });

  it("retains the selected filter while later configuration and lifecycle facts arrive", async () => {
    const page = mountLiveEvents();
    await tick(1);
    fireEvent.click(screen.getByRole("button", { name: /^Filter events/ }));
    fireEvent.click(screen.getByRole("checkbox", { name: "Configuration" }));
    fireEvent.keyDown(document, { key: "Escape" });
    page.available.push(event("evt-config", "idle_timeout_changed", 10_000));
    page.available.push(event("evt-sleep", "service_hibernated", 20_000));
    await tick(30_000);
    expect(screen.getByText("Service went to sleep")).toBeInTheDocument();
    expect(screen.queryByText("Idle timeout updated")).not.toBeInTheDocument();
    page.available.push(event("evt-wake", "service_woken", 40_000));
    await tick(30_000);
    expect(screen.getByText("Service woke up")).toBeInTheDocument();
    expect(screen.getAllByText("Service went to sleep")).toHaveLength(1);
    fireEvent.click(screen.getByRole("button", { name: /^Filter events/ }));
    expect(
      screen.getByRole("checkbox", { name: "Configuration" }),
    ).not.toBeChecked();
  });
});
