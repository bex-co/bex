import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import {
  RouterProvider,
  createRouter,
  createRootRoute,
  createRoute,
  createMemoryHistory,
} from "@tanstack/react-router";
import { ServiceSettingsPage } from "../services.$serviceId.settings";
import type { ServiceView } from "@/features/services/types";
import type { UseServerResult } from "@/features/services/hooks/use-server";

// The Settings page is a client of useServer plus each section's own data
// hooks (CustomDomainsSection/IdleTimeoutRow/InstanceTypeRow); mock all of them
// so the test can drive section presence/absence by service type alone,
// mirroring the pattern in services.$serviceId.test.tsx.
const serverState: UseServerResult = {
  service: null,
  loading: false,
  error: undefined,
  refetch: vi.fn(async () => []),
};
vi.mock("@/features/services/hooks/use-server", () => ({
  useServer: () => serverState,
}));

vi.mock("@/features/services/hooks/use-custom-domains", () => ({
  useCustomDomains: () => ({
    domains: [],
    loading: false,
    error: undefined,
    refetch: vi.fn(async () => []),
  }),
  useCustomDomainMutations: () => ({
    addDomain: vi.fn(),
    deleteDomain: vi.fn(),
    verifyDomain: vi.fn(),
    busy: false,
  }),
}));

vi.mock("@/features/services/hooks/use-idle-timeout", () => ({
  useIdleTimeout: () => ({ setIdleTimeout: vi.fn(), busy: false }),
}));

vi.mock("@/features/services/hooks/use-instance-types", () => ({
  useInstanceTypes: () => ({
    instanceTypes: [],
    loading: false,
    error: undefined,
    byID: () => undefined,
  }),
}));

function svc(overrides: Partial<ServiceView> = {}): ServiceView {
  return {
    id: "app",
    name: "app",
    type: "web_service",
    suspended: false,
    phase: "Running",
    url: "https://app.onbex.co",
    createdAt: "2026-01-01T00:00:00Z",
    replicas: 1,
    revision: "r1",
    plan: null,
    idleTTLSeconds: 0,
    schedule: null,
    command: null,
    runs: [],
    ...overrides,
  };
}

// The route's component reads serviceId via Route.useParams(), so it needs a
// real router context — rebuild it as a minimal tree rooted at the settings
// path, mirroring services.$serviceId.test.tsx.
function renderSettings() {
  const rootRoute = createRootRoute();
  const settingsRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/services/$serviceId/settings",
    component: ServiceSettingsPage,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([settingsRoute]),
    history: createMemoryHistory({ initialEntries: ["/services/app/settings"] }),
    context: { client: {} as never, session: null },
  });
  return render(<RouterProvider router={router} />);
}

beforeEach(() => {
  serverState.service = null;
  serverState.loading = false;
  serverState.error = undefined;
});

describe("ServiceSettingsPage", () => {
  it("shows Custom Domains + Idle timeout, no Deploy section, for a web service", async () => {
    serverState.service = svc({ type: "web_service" });
    renderSettings();

    expect(await screen.findByText("Custom Domains")).toBeInTheDocument();
    expect(screen.getByText("Idle timeout")).toBeInTheDocument();
    expect(screen.queryByText("Deploy")).not.toBeInTheDocument();
  });

  it("shows a Deploy section (schedule + command), no Custom Domains or Idle timeout, for a cron job", async () => {
    serverState.service = svc({
      type: "cron_job",
      url: null,
      schedule: "*/15 * * * *",
      command: "npm run send-nightly-report",
    });
    renderSettings();

    expect(await screen.findByText("Deploy")).toBeInTheDocument();
    expect(screen.getByText("*/15 * * * *")).toBeInTheDocument();
    expect(screen.getByText("npm run send-nightly-report")).toBeInTheDocument();
    expect(screen.queryByText("Custom Domains")).not.toBeInTheDocument();
    expect(screen.queryByText("Idle timeout")).not.toBeInTheDocument();
  });
});
