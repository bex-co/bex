import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  RouterProvider,
  createRouter,
  createRootRoute,
  createRoute,
  createMemoryHistory,
  type AnyRouter,
} from "@tanstack/react-router";
import { ServiceEventsPage } from "../services.$serviceId.events";
import { formatDateTime } from "@/common/lib/format";
import type { UseServerResult } from "@/features/services/hooks/use-server";

// The Events tab reads server(id) for the cron-runs section; a null service
// (the common non-cron case) keeps that section out of the way.
const serverState: UseServerResult = {
  service: null,
  loading: false,
  error: undefined,
  refetch: vi.fn(async () => []),
};
vi.mock("@/features/services/hooks/use-server", () => ({
  useServer: () => serverState,
}));

const mockUseQuery = vi.fn();
const rollbackService = vi.fn();
const cancelDeploy = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useQuery: (...args: unknown[]) => mockUseQuery(...args),
  useMutation: vi.fn(),
}));

function deployEvent(over: Record<string, unknown> = {}) {
  return {
    id: "evt-live-001",
    type: "deploy_ended",
    timestamp: "2026-07-14T14:30:00Z",
    cursor: "cursor-live-001",
    details: {
      deployId: "dep-live-001",
      deployStatus: "succeeded",
      preDeployStatus: "",
      actor: "dev@localhost",
      triggeredByUser: "dev@localhost",
      trigger: { manual: true },
    },
    ...over,
  };
}

// The route's component reads serviceId via Route.useParams(), so it needs a
// real router context — rebuild it rooted at the events path (mirrors
// services.$serviceId.settings.test.tsx's pattern). The deploys/$deployId
// destination route isn't mounted: TanStack's <Link> renders a real <a href>
// synchronously, so asserting on href proves the row points at the right
// page without the destination page needing to exist in this test; for the
// rollback case we assert the router's own post-navigate location instead
// (useNavigate is real here, not mocked).
function renderEvents(serviceId = "app") {
  const rootRoute = createRootRoute();
  const eventsRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/services/$serviceId/events",
    component: ServiceEventsPage,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([eventsRoute]),
    history: createMemoryHistory({
      initialEntries: [`/services/${serviceId}/events`],
    }),
    context: { client: {} as never, session: null },
  });
  const result = render(<RouterProvider router={router} />);
  return { router: router as AnyRouter, ...result };
}

beforeEach(async () => {
  mockUseQuery.mockReset();
  rollbackService.mockReset();
  cancelDeploy.mockReset();
  serverState.service = null;
  const { useMutation } = await import("@apollo/client/react");
  vi.mocked(useMutation).mockImplementation((doc: unknown) => {
    // Match by the mutation's own operation name (definitions[0].name.value)
    // rather than call order — robust to the two useMutation calls in
    // ServiceEventsPage being reordered.
    type OpDoc = { definitions?: Array<{ name?: { value?: string } }> };
    const opName = (doc as OpDoc)?.definitions?.[0]?.name?.value;
    if (opName === "RollbackService") {
      return [rollbackService, { loading: false }] as never;
    }
    return [cancelDeploy, { loading: false }] as never;
  });
});

describe("ServiceEventsPage — deploy rows link to the deploy page (w9/m1/t004)", () => {
  it("links each stored deploy row to its exact deploy page", async () => {
    mockUseQuery.mockReturnValue({
      data: { serviceEvents: [deployEvent()] },
      loading: false,
      refetch: vi.fn(),
    });

    renderEvents("app");

    const link = await screen.findByRole("link");
    expect(link).toHaveAttribute("href", "/services/app/deploys/dep-live-001");
  });

  it("presents deploy_started as in progress even though its status is empty", async () => {
    mockUseQuery.mockReturnValue({
      data: {
        serviceEvents: [
          deployEvent({
            type: "deploy_started",
            details: {
              deployId: "dep-live-001",
              deployStatus: "",
              preDeployStatus: "",
              trigger: { firstBuild: true },
            },
          }),
        ],
      },
      loading: false,
      refetch: vi.fn(),
    });

    renderEvents("app");

    expect(await screen.findByText("In Progress")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Cancel" })).toBeInTheDocument();
    // The row's <time> hover carries the exact stamp via the shared formatter
    // ("July 14, 2026 at …") — computed through the helper so this holds in
    // any runner timezone.
    expect(
      screen.getByTitle(formatDateTime("2026-07-14T14:30:00Z")!),
    ).toBeInTheDocument();
  });

  it("does not offer cancel on a started event whose deploy already finished", async () => {
    mockUseQuery.mockReturnValue({
      data: {
        serviceEvents: [
          deployEvent(),
          deployEvent({
            id: "evt-started-001",
            type: "deploy_started",
            details: {
              deployId: "dep-live-001",
              deployStatus: "",
              preDeployStatus: "",
              trigger: { firstBuild: true },
            },
          }),
        ],
      },
      loading: false,
      refetch: vi.fn(),
    });

    renderEvents("app");

    expect(await screen.findByText("Deploy started")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Cancel" }),
    ).not.toBeInTheDocument();
  });

  it("names non-deploy activity instead of presenting it as an unknown deploy", async () => {
    mockUseQuery.mockReturnValue({
      data: {
        serviceEvents: [
          deployEvent({
            id: "evt-scale-001",
            type: "instance_count_changed",
            details: {
              deployId: "",
              deployStatus: "",
              preDeployStatus: "",
              trigger: null,
            },
          }),
        ],
      },
      loading: false,
      refetch: vi.fn(),
    });

    renderEvents("app");

    expect(
      await screen.findByText("Instance count changed"),
    ).toBeInTheDocument();
    expect(screen.queryByText("Unknown")).not.toBeInTheDocument();
    expect(screen.queryByRole("link")).not.toBeInTheDocument();
  });

  it("navigates to the rollback deploy's own page once the mutation resolves its id", async () => {
    mockUseQuery.mockReturnValue({
      data: { serviceEvents: [deployEvent()] },
      loading: false,
      refetch: vi.fn(),
    });
    rollbackService.mockResolvedValue({
      data: { rollbackService: { id: "dep-rollback-1", status: "live" } },
    });

    const user = userEvent.setup();
    const { router } = renderEvents("app");

    await user.click(
      await screen.findByRole("button", { name: "Rollback" }),
    );
    const dialog = await screen.findByRole("alertdialog");
    await user.click(within(dialog).getByRole("button", { name: "Proceed" }));

    expect(rollbackService).toHaveBeenCalledWith({
      variables: { serviceId: "app", deployId: "dep-live-001" },
    });
    await vi.waitFor(() => {
      expect(router.state.location.pathname).toBe(
        "/services/app/deploys/dep-rollback-1",
      );
    });
  });

  it("does not navigate when the rollback mutation rejects", async () => {
    mockUseQuery.mockReturnValue({
      data: { serviceEvents: [deployEvent()] },
      loading: false,
      refetch: vi.fn(),
    });
    rollbackService.mockRejectedValue(new Error("conflict"));

    const user = userEvent.setup();
    const { router } = renderEvents("app");

    await user.click(
      await screen.findByRole("button", { name: "Rollback" }),
    );
    const dialog = await screen.findByRole("alertdialog");
    await user.click(within(dialog).getByRole("button", { name: "Proceed" }));

    expect(rollbackService).toHaveBeenCalled();
    expect(router.state.location.pathname).toBe("/services/app/events");
  });
});
