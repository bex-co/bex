import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from "@tanstack/react-router";
import type {
  BuildSourceBuildView,
  BuildSourceView,
} from "@/features/build-sources/types";
import { TriggerBuildDialog } from "../trigger-build-dialog";

const triggerMock = vi.hoisted(() => vi.fn());

vi.mock("@/features/build-sources/hooks/use-trigger-build", () => ({
  useTriggerBuild: () => ({
    trigger: triggerMock,
    busy: false,
  }),
}));

const GIT_SOURCE: BuildSourceView = {
  id: "bds-git-1",
  name: "shared-api",
  ownerId: "tea-1",
  projectId: null,
  kind: "git",
  git: {
    repoUrl: "https://github.com/acme/shared-api",
    branch: "main",
    runtime: "node",
    region: "oregon",
    buildCommand: "npm run build",
    rootDir: null,
    dockerfilePath: null,
    dockerContext: null,
    registryCredentialId: null,
    buildFilter: null,
  },
  image: null,
  serviceLinks: [
    { id: "srv-1", name: "api-web" },
    { id: "srv-2", name: "jobs-worker" },
  ],
  createdAt: "2026-11-23T00:00:00Z",
  updatedAt: "2026-11-24T00:00:00Z",
  cursor: "bds-git-1",
};

const TRIGGERED_BUILD: BuildSourceBuildView = {
  id: "bld-999",
  buildSourceId: "bds-git-1",
  status: "created",
  succeeded: false,
  commitId: null,
  commitUrl: null,
  runtime: "node",
  buildStartedAt: "2026-11-24T02:00:00Z",
  buildFinishedAt: null,
  imageRef: null,
  imageSha: null,
  registryCredentialId: null,
  deploys: [{ serviceId: "srv-1", deployId: "dep-101" }],
  skippedServices: [{ serviceId: "srv-2", reason: "suspended" }],
  createdAt: "2026-11-24T02:00:00Z",
  updatedAt: "2026-11-24T02:00:00Z",
  cursor: "dep-999",
};

function renderDialog(source: BuildSourceView = GIT_SOURCE) {
  const rootRoute = createRootRoute();
  const routes = [
    createRoute({
      getParentRoute: () => rootRoute,
      path: "/",
      component: () => (
        <TriggerBuildDialog
          buildSource={source}
          open
          onOpenChange={vi.fn()}
        />
      ),
    }),
    createRoute({
      getParentRoute: () => rootRoute,
      path: "/services/$serviceId/deploys/$deployId",
      component: () => null,
    }),
    createRoute({
      getParentRoute: () => rootRoute,
      path: "/build-sources/$buildSourceId/builds/$buildId",
      component: () => null,
    }),
  ];
  const router = createRouter({
    routeTree: rootRoute.addChildren(routes),
    history: createMemoryHistory({ initialEntries: ["/"] }),
    context: { client: {} as never, session: null },
  });
  return render(<RouterProvider router={router} />);
}

beforeEach(() => {
  vi.clearAllMocks();
});

describe("TriggerBuildDialog (w2/m614/t002)", () => {
  it("submits autodeploy_only without serviceIds and surfaces deploys + skippedServices", async () => {
    triggerMock.mockResolvedValue(TRIGGERED_BUILD);
    const user = userEvent.setup();
    renderDialog();

    const submit = await screen.findByRole("button", { name: "Trigger Build" });
    await user.click(submit);

    expect(triggerMock).toHaveBeenCalledWith({
      buildSourceId: "bds-git-1",
      clearCache: "do_not_clear",
      deploy: { serviceScope: "autodeploy_only" },
    });

    expect(await screen.findByText("Build started")).toBeInTheDocument();
    expect(screen.getByText(/bld-999/)).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /dep-101/ })).toHaveAttribute(
      "href",
      "/services/srv-1/deploys/dep-101",
    );
    expect(screen.getByTestId("trigger-result-skipped")).toHaveTextContent(
      "jobs-worker",
    );
    expect(screen.getByTestId("trigger-result-skipped")).toHaveTextContent(
      "Suspended",
    );
    expect(
      screen.getByRole("link", { name: "View build logs" }),
    ).toHaveAttribute("href", "/build-sources/bds-git-1/builds/bld-999");
  });

  it("submits none scope without serviceIds", async () => {
    triggerMock.mockResolvedValue({
      ...TRIGGERED_BUILD,
      deploys: [],
      skippedServices: [],
    });
    const user = userEvent.setup();
    renderDialog();

    await user.click(
      await screen.findByLabelText(/Build only \(don't deploy\)/i),
    );
    await user.click(screen.getByRole("button", { name: "Trigger Build" }));

    expect(triggerMock).toHaveBeenCalledWith({
      buildSourceId: "bds-git-1",
      clearCache: "do_not_clear",
      deploy: { serviceScope: "none" },
    });
    expect(
      await screen.findByText("No service deploys were queued for this build."),
    ).toBeInTheDocument();
  });

  it("requires >=1 linked service for custom scope and sends serviceIds + clearCache", async () => {
    triggerMock.mockResolvedValue(TRIGGERED_BUILD);
    const user = userEvent.setup();
    renderDialog();

    await user.click(await screen.findByLabelText(/Selected services/i));
    const submit = screen.getByRole("button", { name: "Trigger Build" });
    expect(submit).toBeDisabled();
    expect(
      screen.getByText("Select at least one service to deploy."),
    ).toBeInTheDocument();

    await user.click(screen.getByLabelText("api-web"));
    await user.click(screen.getByLabelText(/Clear build cache/i));
    expect(submit).not.toBeDisabled();

    await user.click(submit);

    expect(triggerMock).toHaveBeenCalledWith({
      buildSourceId: "bds-git-1",
      clearCache: "clear",
      deploy: { serviceScope: "custom", serviceIds: ["srv-1"] },
    });
  });
});
