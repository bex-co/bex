import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ApolloClient, ApolloLink, InMemoryCache } from "@apollo/client";
import { ApolloProvider } from "@apollo/client/react";
import {
  Outlet,
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from "@tanstack/react-router";
import { Observable } from "rxjs";
import { toast } from "sonner";
import { apolloCacheConfig } from "@/common/apollo/cache";
import { ServerDocument } from "@/graphql/definitions";
import { DeleteServiceCard } from "@/features/services/components/delete-service-card";
import { ServiceDetailLayout } from "@/features/services/components/service-detail-layout";
import { useServer } from "@/features/services/hooks/use-server";
import { blankSelection } from "@/test/mocks/blank-selection";

/**
 * w4/340: deleting a service from its own Settings evicts the Service, and the
 * still-mounted detail layout refetches `server(id)` and reads not-found while
 * the navigation home is still loading. Its dead-id redirect then toasted
 * "That resource doesn't exist or was deleted." beside "Deleted <name>". Real
 * Apollo cache, real layout, hook and card; only the network is faked, and the
 * home route's loader is held so the layout stays mounted through the
 * not-found read, as it does in production while the next route loads.
 */

vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}));
// The topbar, header and lifecycle read sibling Apollo documents; stub them at
// the hook boundary as the dead-id test does.
vi.mock("@/features/services/hooks/use-cached-service-type", () => ({
  useCachedServiceType: () => null,
}));
vi.mock("@/features/services/hooks/use-services", () => ({
  useServices: () => ({
    services: [],
    loading: false,
    error: undefined,
    refetch: vi.fn(async () => []),
  }),
}));
vi.mock("@/features/projects/hooks/use-projects", () => ({
  useProjects: () => ({
    projects: [],
    loading: false,
    error: undefined,
    refetch: vi.fn(async () => undefined),
  }),
}));
vi.mock("@/features/environments/hooks/use-environments", () => ({
  useEnvironments: () => ({
    environments: [],
    loading: false,
    error: undefined,
    refetch: vi.fn(async () => undefined),
  }),
}));
vi.mock("@/features/deploys/hooks/use-latest-deploy", () => ({
  useLatestDeploy: () => ({ deploy: null, loading: false, error: undefined }),
}));
vi.mock("@/features/services/hooks/use-service-lifecycle", () => ({
  useServiceLifecycle: () => ({ pending: null }),
}));
vi.mock("@/features/services/components/service-detail-header", () => ({
  ServiceDetailHeader: () => <p>service header</p>,
  ServiceDetailHeaderSkeleton: () => <p>service header skeleton</p>,
}));

const PHRASE = "sudo delete web service web";
const NOT_FOUND = "That resource doesn't exist or was deleted.";

let gone: boolean;
let refuseDelete: boolean;

const SERVER_SHAPE = blankSelection(ServerDocument, "server");

function service(id: string) {
  return {
    ...SERVER_SHAPE,
    __typename: "Service",
    id,
    name: "web",
    type: "web_service",
    phase: "Running",
  };
}

const link = new ApolloLink(
  (op) =>
    new Observable<ApolloLink.Result>((sub) => {
      const send = (result: ApolloLink.Result) => {
        sub.next(result);
        sub.complete();
      };
      const id = (op.variables as { id: string }).id;
      if (op.operationName === "Server") {
        // bex-api answers a deleted id with a null row plus a NOT_FOUND error.
        if (gone)
          send({
            data: { server: null },
            errors: [
              { message: "not found", extensions: { code: "NOT_FOUND" } },
            ],
          });
        else send({ data: { server: service(id) } });
        return;
      }
      if (op.operationName === "DeleteService") {
        if (refuseDelete) {
          send({
            data: null,
            errors: [
              { message: "internal error", extensions: { code: "INTERNAL" } },
            ],
          });
          return;
        }
        gone = true;
        send({ data: { deleteService: true } });
        return;
      }
      sub.error(new Error(`unexpected operation ${op.operationName}`));
    }),
);

function SettingsTab() {
  const { service: view } = useServer("srv-1", { poll: false });
  return view ? <DeleteServiceCard service={view} /> : null;
}

function renderDetail() {
  const client = new ApolloClient({
    link,
    cache: new InMemoryCache(apolloCacheConfig),
  });
  let releaseHome!: () => void;
  const homeGate = new Promise<void>((resolve) => {
    releaseHome = resolve;
  });
  const rootRoute = createRootRoute({ component: () => <Outlet /> });
  const home = createRoute({
    getParentRoute: () => rootRoute,
    path: "/",
    loader: () => homeGate,
    component: () => <p>home page</p>,
  });
  const detail = createRoute({
    getParentRoute: () => rootRoute,
    path: "/services/$serviceId",
    component: () => <ServiceDetailLayout serviceId="srv-1" />,
  });
  const settings = createRoute({
    getParentRoute: () => detail,
    path: "settings",
    component: SettingsTab,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([home, detail.addChildren([settings])]),
    history: createMemoryHistory({
      initialEntries: ["/services/srv-1/settings"],
    }),
    defaultPendingMs: Infinity,
  });
  render(
    <ApolloProvider client={client}>
      <RouterProvider router={router} />
    </ApolloProvider>,
  );
  return { client, router, releaseHome };
}

async function confirmDelete() {
  const user = userEvent.setup();
  await user.click(
    await screen.findByRole("button", { name: "Delete Service" }),
  );
  const dialog = await screen.findByRole("alertdialog");
  await user.type(within(dialog).getByLabelText("Sudo Command"), PHRASE);
  await user.click(
    within(dialog).getByRole("button", { name: "Delete Service" }),
  );
}

beforeEach(() => {
  gone = false;
  refuseDelete = false;
  vi.mocked(toast.success).mockReset();
  vi.mocked(toast.error).mockReset();
});

describe("service detail delete (w4/340)", () => {
  it("shows only the success toast while the eviction's not-found read lands", async () => {
    const { router, releaseHome } = renderDetail();
    await confirmDelete();

    // The eviction's refetch has read not-found with the layout still
    // mounted: only that settled branch renders this skeleton.
    await waitFor(() =>
      expect(
        document.querySelector('[data-route-skeleton="service-active-tab"]'),
      ).not.toBeNull(),
    );
    expect(toast.error).not.toHaveBeenCalled();

    await act(async () => releaseHome());
    expect(await screen.findByText("home page")).toBeInTheDocument();
    expect(router.state.location.pathname).toBe("/");
    expect(toast.success).toHaveBeenCalledTimes(1);
    expect(toast.success).toHaveBeenCalledWith("Deleted web");
    expect(toast.error).not.toHaveBeenCalled();
  });

  it("keeps the dead-id redirect for a not-found that follows a refused delete", async () => {
    refuseDelete = true;
    const { client, router, releaseHome } = renderDetail();
    await confirmDelete();
    await waitFor(() =>
      expect(toast.error).toHaveBeenCalledWith("Couldn't delete web"),
    );
    expect(router.state.location.pathname).toBe("/services/srv-1/settings");

    // Deleted elsewhere later: the next read is a real dead id again.
    gone = true;
    await act(async () => {
      await client.refetchQueries({ include: [ServerDocument] });
    });
    await waitFor(() =>
      expect(toast.error).toHaveBeenCalledWith(NOT_FOUND, {
        id: "resource-not-found",
      }),
    );
    await act(async () => releaseHome());
    expect(await screen.findByText("home page")).toBeInTheDocument();
    expect(toast.success).not.toHaveBeenCalled();
  });
});
