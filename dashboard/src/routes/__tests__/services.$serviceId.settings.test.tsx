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

// The danger-zone card (w5/m14) is a client of useDeleteService (Apollo) —
// mock it so this page test stays about section presence, not the delete wire.
vi.mock("@/features/services/hooks/use-delete-service", () => ({
  useDeleteService: () => ({
    remove: vi.fn(async () => true),
    deleting: false,
  }),
}));

// Scaling row (w5/m16) calls scaleService; mock so section-presence assertions
// don't hit Apollo.
vi.mock("@/features/services/hooks/use-scale-service", () => ({
  useScaleService: () => ({
    scaleService: vi.fn(async () => true),
    busy: false,
  }),
}));

// Health Check Path row (w5/m21) calls setHealthCheckPath via Apollo; mock it
// so section-presence assertions don't need an Apollo client.
vi.mock("@/features/services/hooks/use-health-check-path", () => ({
  useHealthCheckPath: () => ({
    setHealthCheckPath: vi.fn(async () => true),
    busy: false,
  }),
}));

vi.mock("@/features/services/hooks/use-display-name", () => ({
  useDisplayName: () => ({
    setDisplayName: vi.fn(async () => true),
    busy: false,
  }),
}));

// DeployHookSection (w2/m33) owns an Apollo query/mutation pair. Keep this page
// test on section composition while the component/hook suites cover the real
// reveal, copy, rotation, and failure behavior.
vi.mock("@/features/services/hooks/use-deploy-hook", () => ({
  useDeployHook: () => ({
    url: "https://api.bex.co/v1/deploy-hooks/dhk-test",
    loading: false,
    error: undefined,
    regenerate: vi.fn(async () => true),
    regenerating: false,
  }),
}));

// CronDeploySection (w5/m18) calls useCronJob which hits Apollo; mock it.
vi.mock("@/features/services/hooks/use-cron-job", () => ({
  useCronJob: () => ({ updateCronJob: vi.fn(async () => true), busy: false }),
}));

// BuildDeploySection (w5/m13, and w5/010 for git-sourced cron) is a client of
// useRootDir/useAutoDeploy/useGitConnection — mock them so this page test stays
// about section presence.
vi.mock("@/features/services/hooks/use-root-dir", () => ({
  useRootDir: () => ({ setRootDir: vi.fn(async () => true), busy: false }),
}));
vi.mock("@/features/services/hooks/use-pre-deploy-command", () => ({
  usePreDeployCommand: () => ({
    setPreDeployCommand: vi.fn(async () => true),
    busy: false,
  }),
}));
vi.mock("@/features/services/hooks/use-auto-deploy", () => ({
  useAutoDeploy: () => ({
    setAutoDeploy: vi.fn(async () => true),
    busy: false,
  }),
}));
vi.mock("@/features/git/hooks/use-git-connection", () => ({
  useGitConnection: () => ({ connection: undefined }),
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
    healthCheckPath: null,
    ...overrides,
  };
}

// The route's component reads serviceId via Route.useParams(), so it needs a
// real router context — rebuild it as a minimal tree rooted at the settings
// path, mirroring services.$serviceId.test.tsx.
function renderSettings(serviceId = "app") {
  const rootRoute = createRootRoute();
  const settingsRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/services/$serviceId/settings",
    component: ServiceSettingsPage,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([settingsRoute]),
    history: createMemoryHistory({
      initialEntries: [`/services/${serviceId}/settings`],
    }),
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
  it("shows the mutable Service Name while making the immutable id explicit", async () => {
    serverState.service = svc({
      id: "stable-service-id",
      name: "Customer API",
      displayName: "Customer API",
    });
    renderSettings("stable-service-id");

    expect(await screen.findByText("Service Name")).toBeInTheDocument();
    expect(screen.getByText("Customer API")).toBeInTheDocument();
    expect(
      screen.getByText(
        "The service ID remains stable-service-id; URLs and infrastructure do not change.",
      ),
    ).toBeInTheDocument();
  });

  it("shows Custom Domains + Idle timeout + instance count stepper, no Deploy section, for a web service", async () => {
    serverState.service = svc({ type: "web_service" });
    renderSettings();

    expect(await screen.findByText("Custom Domains")).toBeInTheDocument();
    expect(screen.getByText("Idle timeout")).toBeInTheDocument();
    expect(screen.getByText("Instance count")).toBeInTheDocument();
    expect(screen.getByText("Deploy Hook")).toBeInTheDocument();
    expect(screen.queryByText("Deploy")).not.toBeInTheDocument();
  });

  it("shows instance count stepper for a background_worker", async () => {
    serverState.service = svc({ type: "background_worker" });
    renderSettings();

    expect(await screen.findByText("Instance count")).toBeInTheDocument();
  });

  it("shows a Deploy section (schedule + command), hides Custom Domains, Idle timeout, and instance count for a cron job", async () => {
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
    expect(screen.queryByText("Instance count")).not.toBeInTheDocument();
  });

  it("shows Build & Deploy (Auto-Deploy toggle) alongside the Deploy section for a git-sourced cron job", async () => {
    serverState.service = svc({
      type: "cron_job",
      url: null,
      schedule: "*/15 * * * *",
      command: "npm run send-nightly-report",
      repo: "https://github.com/acme/reports",
      branch: "main",
    });
    renderSettings();

    // Both the cron Deploy section (schedule/command) and Build & Deploy render.
    expect(await screen.findByText("Deploy")).toBeInTheDocument();
    expect(screen.getByText("Build & Deploy")).toBeInTheDocument();
    expect(screen.getByText("Auto-Deploy")).toBeInTheDocument();
    expect(
      screen.getByText("https://github.com/acme/reports"),
    ).toBeInTheDocument();
  });

  it("hides Build & Deploy for an image-backed cron job (nothing to build)", async () => {
    serverState.service = svc({
      type: "cron_job",
      url: null,
      schedule: "*/15 * * * *",
      command: "npm run send-nightly-report",
      repo: null,
    });
    renderSettings();

    expect(await screen.findByText("Deploy")).toBeInTheDocument();
    expect(screen.queryByText("Build & Deploy")).not.toBeInTheDocument();
    expect(screen.queryByText("Auto-Deploy")).not.toBeInTheDocument();
  });
});
