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

// The suspend/resume card calls useServiceLifecycle (Apollo mutations) —
// mock it so section-presence assertions don't hit Apollo.
vi.mock("@/features/services/hooks/use-service-lifecycle", () => ({
  useServiceLifecycle: () => ({
    pending: null,
    run: vi.fn(async () => ({ status: "success" as const })),
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

// Platform Subdomain section (w7/m31) calls setSubdomainPolicy via Apollo;
// mock it so section-presence assertions don't need an Apollo client.
vi.mock("@/features/services/hooks/use-subdomain-policy", () => ({
  useSubdomainPolicy: () => ({
    setSubdomainPolicy: vi.fn(async () => true),
    busy: false,
  }),
}));

// Maintenance Mode section (w1/m37) calls setMaintenanceMode via Apollo; mock
// it so section-presence assertions don't need an Apollo client.
vi.mock("@/features/services/hooks/use-maintenance-mode", () => ({
  useMaintenanceMode: () => ({
    setMaintenanceMode: vi.fn(async () => true),
    busy: false,
  }),
}));

// Notifications row (w4/m21) calls setNotifyOnFail via Apollo; mock it so
// section-presence assertions don't need an Apollo client.
vi.mock("@/features/services/hooks/use-service-notifications", () => ({
  useServiceNotifications: () => ({
    setNotificationsToSend: vi.fn(async () => true),
    busy: false,
  }),
}));

vi.mock("@/features/services/hooks/use-max-shutdown-delay", () => ({
  useMaxShutdownDelay: () => ({
    setMaxShutdownDelay: vi.fn(async () => true),
    busy: false,
  }),
}));

vi.mock("@/features/services/hooks/use-display-name", () => ({
  useDisplayName: () => ({
    setDisplayName: vi.fn(async () => true),
    busy: false,
  }),
}));

vi.mock("@/features/services/hooks/use-registry-credential", () => ({
  useRegistryCredential: () => ({
    setRegistryCredential: vi.fn(async () => true),
    busy: false,
  }),
}));

vi.mock(
  "@/features/registry-credentials/hooks/use-registry-credentials",
  () => ({
    useRegistryCredentials: () => ({
      credentials: [
        {
          id: "rgc-private",
          name: "Private GHCR",
          host: "ghcr.io",
          username: "robot",
          expiresAt: null,
          status: "active",
          createdAt: null,
        },
      ],
      loading: false,
      error: undefined,
      refetch: vi.fn(),
    }),
  }),
);

vi.mock("@/features/services/hooks/use-subdomain-policy", () => ({
  useSubdomainPolicy: () => ({
    setSubdomainPolicy: vi.fn(async () => true),
    busy: false,
  }),
}));

vi.mock("@/features/services/hooks/use-service-networking", () => ({
  useServiceNetworking: () => ({
    saveAllowList: vi.fn(async () => true),
    saving: false,
  }),
}));

vi.mock("@/features/services/hooks/use-maintenance-mode", () => ({
  useMaintenanceMode: () => ({
    setMaintenanceMode: vi.fn(async () => true),
    busy: false,
  }),
}));

vi.mock("@/features/services/hooks/use-static-site", () => ({
  useStaticSiteMutations: () => ({
    setRoutes: vi.fn(async () => true),
    setHeaders: vi.fn(async () => true),
    setPublishPath: vi.fn(async () => true),
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
vi.mock("@/features/services/hooks/use-branch", () => ({
  useBranch: () => ({ setBranch: vi.fn(async () => true), busy: false }),
}));
vi.mock("@/features/services/hooks/use-build-command", () => ({
  useBuildCommand: () => ({
    setBuildCommand: vi.fn(async () => true),
    busy: false,
  }),
}));
vi.mock("@/features/services/hooks/use-start-command", () => ({
  useStartCommand: () => ({
    setStartCommand: vi.fn(async () => true),
    busy: false,
  }),
}));
vi.mock("@/features/services/hooks/use-dockerfile-path", () => ({
  useDockerfilePath: () => ({
    setDockerfilePath: vi.fn(async () => true),
    busy: false,
  }),
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
vi.mock("@/features/services/hooks/use-build-filter", () => ({
  useBuildFilter: () => ({
    setBuildFilter: vi.fn(async () => true),
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
    maintenanceMode: { enabled: false, uri: "" },
    schedule: null,
    command: null,
    runs: [],
    healthCheckPath: null,
    maxShutdownDelaySeconds: 30,
    registryCredentialId: null,
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
    expect(screen.getByRole("textbox", { name: "Service Name" })).toHaveValue(
      "Customer API",
    );
    expect(
      screen.getByText(
        "The service ID remains stable-service-id; URLs and infrastructure do not change.",
      ),
    ).toBeInTheDocument();
  });

  it("shows Custom Domains + Idle timeout, no instance stepper (moved to Scaling, w7/m43), no Deploy section, for a web service", async () => {
    serverState.service = svc({ type: "web_service" });
    renderSettings();

    expect(await screen.findByText("Custom Domains")).toBeInTheDocument();
    expect(screen.getByText("Idle timeout")).toBeInTheDocument();
    // Manual instance count lives on the Scaling tab beside autoscaling
    // (w7/m43 — Render's placement; supersedes the w5/m16 Settings stepper).
    expect(screen.queryByText("Instance count")).not.toBeInTheDocument();
    expect(screen.getByText("Max shutdown delay")).toBeInTheDocument();
    expect(screen.getByText("Deploy Hook")).toBeInTheDocument();
    expect(screen.getByText("Maintenance Mode")).toBeInTheDocument();
    expect(screen.queryByText("Deploy")).not.toBeInTheDocument();
  });

  // Settings IA alignment with Render's section layout (w5/m52).
  it("titles the first card 'General' with a read-only Region row when set", async () => {
    serverState.service = svc({ region: "fsn1" });
    renderSettings();

    expect(await screen.findByText("General")).toBeInTheDocument();
    const region = screen.getByRole("textbox", { name: "Region" });
    expect(region).toHaveValue("fsn1");
    expect(region).toBeDisabled();
  });

  it("hides the Region row when no region is configured", async () => {
    serverState.service = svc({ region: null });
    renderSettings();

    await screen.findByText("General");
    expect(
      screen.queryByRole("textbox", { name: "Region" }),
    ).not.toBeInTheDocument();
  });

  it("splits Build and Deploy into separate cards for a repo-backed service", async () => {
    serverState.service = svc({
      repo: "https://github.com/acme/app",
      runtime: "node",
      builder: "native",
    });
    renderSettings();

    expect(await screen.findByText("Build")).toBeInTheDocument();
    expect(screen.getByText("Deploy")).toBeInTheDocument();
    // Deploy Hook is embedded in the Deploy card (not a standalone card).
    expect(screen.getByText("Deploy Hook")).toBeInTheDocument();
  });

  it("folds the Platform Subdomain toggle into the Custom Domains card", async () => {
    serverState.service = svc({ type: "web_service" });
    renderSettings();

    expect(await screen.findByText("Custom Domains")).toBeInTheDocument();
    expect(screen.getByText("Platform Subdomain")).toBeInTheDocument();
  });

  it("shows Idle timeout + Max shutdown delay for a background_worker, without the moved instance stepper", async () => {
    serverState.service = svc({ type: "background_worker" });
    renderSettings();

    expect(await screen.findByText("Idle timeout")).toBeInTheDocument();
    expect(screen.queryByText("Instance count")).not.toBeInTheDocument();
    expect(screen.getByText("Max shutdown delay")).toBeInTheDocument();
    expect(screen.queryByText("Maintenance Mode")).not.toBeInTheDocument();
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
    expect(screen.getByRole("textbox", { name: "Schedule" })).toHaveValue(
      "*/15 * * * *",
    );
    expect(screen.getByRole("textbox", { name: "Command" })).toHaveValue(
      "npm run send-nightly-report",
    );
    expect(screen.queryByText("Custom Domains")).not.toBeInTheDocument();
    expect(screen.queryByText("Idle timeout")).not.toBeInTheDocument();
    expect(screen.queryByText("Instance count")).not.toBeInTheDocument();
    expect(screen.queryByText("Max shutdown delay")).not.toBeInTheDocument();
  });

  it("shows Build & Deploy (Auto-Deploy toggle) alongside the Deploy section for a git-sourced cron job", async () => {
    serverState.service = svc({
      type: "cron_job",
      url: null,
      schedule: "*/15 * * * *",
      command: "npm run send-nightly-report",
      repo: "https://github.com/acme/reports",
      branch: "main",
      runtime: "docker",
      builder: "dockerfile",
      startCommand: "bin/cron",
      dockerfilePath: "docker/Dockerfile.cron",
    });
    renderSettings();

    // Both the cron Deploy section (schedule/command) and Build & Deploy render.
    expect(await screen.findByText("Deploy")).toBeInTheDocument();
    expect(screen.getByText("Build")).toBeInTheDocument();
    expect(screen.getByText("Auto-Deploy")).toBeInTheDocument();
    expect(
      screen.getByText("https://github.com/acme/reports"),
    ).toBeInTheDocument();
    expect(screen.queryByText("Docker Command")).not.toBeInTheDocument();
    expect(screen.queryByText("Dockerfile Path")).not.toBeInTheDocument();
  });

  it("hides command and Dockerfile controls for a git-sourced static site", async () => {
    serverState.service = svc({
      type: "static_site",
      repo: "https://github.com/acme/docs",
      branch: "main",
      runtime: "docker",
      builder: "dockerfile",
      startCommand: "bin/server",
      dockerfilePath: "docker/Dockerfile.static",
      publishPath: "dist",
      routes: [],
      headers: [],
    });
    renderSettings();

    expect(await screen.findByText("Build")).toBeInTheDocument();
    expect(screen.queryByText("Docker Command")).not.toBeInTheDocument();
    expect(screen.queryByText("Dockerfile Path")).not.toBeInTheDocument();
  });

  it("hides the Instance Type row for a static site (no sized pod, w5/m48)", async () => {
    serverState.service = svc({
      type: "static_site",
      repo: "https://github.com/acme/docs",
      branch: "main",
      publishPath: "dist",
      routes: [],
      headers: [],
    });
    renderSettings();

    expect(await screen.findByText("Static Site")).toBeInTheDocument();
    expect(screen.queryByText("Instance Type")).not.toBeInTheDocument();
  });

  it("keeps the Instance Type row for a web service (guard against over-gating)", async () => {
    serverState.service = svc();
    renderSettings();

    expect(await screen.findByText("Instance Type")).toBeInTheDocument();
  });

  it("links the static edge-rule editors' dedicated pages from Settings (w5/m48)", async () => {
    serverState.service = svc({
      type: "static_site",
      repo: "https://github.com/acme/docs",
      branch: "main",
      publishPath: "dist",
      routes: [],
      headers: [],
    });
    renderSettings();

    expect(
      await screen.findByRole("link", { name: "Redirects/Rewrites" }),
    ).toHaveAttribute("href", "/services/app/redirects");
    expect(screen.getByRole("link", { name: "Headers" })).toHaveAttribute(
      "href",
      "/services/app/headers",
    );
    // The editors themselves moved off Settings.
    expect(screen.queryByText("Add rule")).not.toBeInTheDocument();
    expect(screen.queryByText("Add header")).not.toBeInTheDocument();
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
    expect(screen.queryByText("Build")).not.toBeInTheDocument();
    expect(screen.queryByText("Auto-Deploy")).not.toBeInTheDocument();
  });

  it("offers registry credential attach for an image-backed service", async () => {
    serverState.service = svc({
      repo: null,
      registryCredentialId: "rgc-private",
    });
    renderSettings();

    expect(
      await screen.findByText("Image registry credential"),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("combobox", { name: "Registry credential" }),
    ).toBeInTheDocument();
  });

  it("offers registry credential attach for a repository Docker build", async () => {
    serverState.service = svc({
      repo: "https://github.com/acme/private-base",
      runtime: "docker",
      builder: "dockerfile",
    });
    renderSettings();

    expect(
      await screen.findByText("Image registry credential"),
    ).toBeInTheDocument();
  });

  it("hides registry credential attach for a native repository build", async () => {
    serverState.service = svc({
      repo: "https://github.com/acme/native",
      runtime: "node",
      builder: "native",
    });
    renderSettings();

    await screen.findByText("Build");
    expect(
      screen.queryByText("Image registry credential"),
    ).not.toBeInTheDocument();
  });

  it("shows the Build Command row for a native-runtime repo service (w5/m51)", async () => {
    serverState.service = svc({
      repo: "https://github.com/acme/native",
      runtime: "node",
      builder: "native",
      buildCommand: "yarn build",
    });
    renderSettings();

    await screen.findByText("Build");
    expect(screen.getByRole("textbox", { name: "Build Command" })).toHaveValue(
      "yarn build",
    );
  });

  it("hides Build Command for a Docker-runtime repo service, showing Dockerfile Path instead (w5/m51)", async () => {
    serverState.service = svc({
      repo: "https://github.com/acme/dockerized",
      runtime: "docker",
      builder: "dockerfile",
      dockerfilePath: "Dockerfile",
    });
    renderSettings();

    await screen.findByText("Build");
    expect(
      screen.queryByRole("textbox", { name: "Build Command" }),
    ).not.toBeInTheDocument();
    expect(screen.getByText("Dockerfile Path")).toBeInTheDocument();
  });
});
