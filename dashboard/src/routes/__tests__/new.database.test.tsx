import { describe, it, expect, vi, beforeAll, beforeEach } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import i18n from "@/i18n/init";
import {
  RouterProvider,
  createRouter,
  createRootRoute,
  createRoute,
  createMemoryHistory,
} from "@tanstack/react-router";
import { NewDatabasePage } from "@/features/databases/pages/new-database-page";
import type { DatabaseInstanceTypeView } from "@/features/databases/types";
import type { UseCreateDatabaseResult } from "@/features/databases/hooks/use-create-database";

beforeAll(() => {
  if (!Element.prototype.hasPointerCapture) {
    Element.prototype.hasPointerCapture = () => false;
  }
  if (!Element.prototype.releasePointerCapture) {
    Element.prototype.releasePointerCapture = () => {};
  }
});

const retryCatalog = vi.fn(async () => undefined);
const instanceTypesState: {
  instanceTypes: DatabaseInstanceTypeView[];
  loading: boolean;
  error: Error | undefined;
  retry: () => Promise<void>;
  retrying: boolean;
} = {
  instanceTypes: [],
  loading: false,
  error: undefined,
  retry: retryCatalog,
  retrying: false,
};

vi.mock("@/features/databases/hooks/use-database-instance-types", () => ({
  useDatabaseInstanceTypes: () => instanceTypesState,
}));

const create = vi.fn();
const createState: Pick<
  UseCreateDatabaseResult,
  "capLimit" | "capLimitUpgrade"
> = { capLimit: null, capLimitUpgrade: true };
vi.mock("@/features/databases/hooks/use-create-database", () => ({
  useCreateDatabase: (): UseCreateDatabaseResult => ({
    create,
    busy: false,
    ...createState,
  }),
}));

vi.mock("@/features/projects/hooks/use-projects", () => ({
  useProjects: () => ({
    projects: [{ id: "prj-1", name: "Commerce" }],
  }),
}));

vi.mock("@/features/environments/hooks/use-environments", () => ({
  useEnvironments: (projectId: string | null) => ({
    environments:
      projectId === "prj-1"
        ? [{ id: "env-1", projectId: "prj-1", name: "Production" }]
        : [],
  }),
}));

const FREE: DatabaseInstanceTypeView = {
  id: "free",
  name: "Free",
  cpu: "100m",
  memory: "256Mi",
  storageGB: 1,
  maxStorageGB: 1,
  supportsDiskAutoscaling: false,
  supportsConnectionPooling: false,
  maxReadReplicas: 0,
  readReplicaMinStorageGB: 10,
  supportsHighAvailability: false,
  monthlyUsd: "0.00",
  storageMonthlyUsdPerGB: "0.00",
};
const BASIC: DatabaseInstanceTypeView = {
  id: "basic-1gb",
  name: "Basic 1GB",
  cpu: "500m",
  memory: "1Gi",
  storageGB: 5,
  maxStorageGB: 16384,
  supportsDiskAutoscaling: true,
  supportsConnectionPooling: true,
  maxReadReplicas: 5,
  readReplicaMinStorageGB: 10,
  supportsHighAvailability: false,
  monthlyUsd: "14.00",
  storageMonthlyUsdPerGB: "0.21",
};

function renderPage() {
  const rootRoute = createRootRoute();
  const newRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/new/database",
    component: NewDatabasePage,
  });
  const detailRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/databases/$databaseId",
    component: () => <p>Database destination</p>,
  });
  const overviewRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/",
    component: () => <p>Overview</p>,
  });
  const settingsRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/workspace/settings",
    component: () => <p>Workspace settings</p>,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([
      newRoute,
      detailRoute,
      overviewRoute,
      settingsRoute,
    ]),
    history: createMemoryHistory({ initialEntries: ["/new/database"] }),
    context: { client: {} as never, session: null },
  });
  return { ...render(<RouterProvider router={router} />), router };
}

const submitButton = () =>
  screen.getByRole("button", { name: "Create database" });
const pickPlan = (user: ReturnType<typeof userEvent.setup>, name: RegExp) =>
  user.click(screen.getByRole("radio", { name }));

beforeEach(() => {
  instanceTypesState.instanceTypes = [FREE, BASIC];
  instanceTypesState.loading = false;
  instanceTypesState.error = undefined;
  createState.capLimit = null;
  createState.capLimitUpgrade = true;
  create.mockReset();
  create.mockResolvedValue("dpg-returned-id");
});

describe("NewDatabasePage", () => {
  // Render's New Postgres is a full page on the shared create template, the
  // submit in the pinned action bar (live capture 2026-11-21).
  it("is a full create page titled New Postgres, with no dialog", async () => {
    const { container } = renderPage();
    expect(
      await screen.findByRole("heading", { level: 1, name: "New Postgres" }),
    ).toBeInTheDocument();
    expect(screen.queryByRole("dialog")).toBeNull();
    const footer = container.querySelector("[data-page-footer]");
    expect(footer).toHaveClass("sticky");
    expect(
      within(footer as HTMLElement).getByRole("button", {
        name: "Create database",
      }),
    ).toBeInTheDocument();
    // Database and User are optional, as on Render.
    expect(screen.getAllByText("Optional").length).toBeGreaterThanOrEqual(2);
  });

  it("validates the name and keeps submit disabled until it's valid", async () => {
    const user = userEvent.setup();
    renderPage();
    const submit = await screen.findByRole("button", {
      name: "Create database",
    });
    expect(submit).toBeDisabled(); // empty name
    expect(
      screen.getByText(i18n.t("common.createBlockedName")),
    ).toBeInTheDocument();

    // Invalid: over 100 characters — the inline error shows, submit stays off.
    await user.click(screen.getByLabelText("Name"));
    await user.paste("x".repeat(101));
    expect(screen.getByText(/up to 100 characters/i)).toBeInTheDocument();
    expect(submit).toBeDisabled();

    // A free-form label is accepted, as on Render (w5/m153), even one shaped
    // like a resource ID (w5/m118 reversed w8/049).
    for (const name of ["Shop DB_1", "dpg-abcdefghij0123456789"]) {
      await user.clear(screen.getByLabelText("Name"));
      await user.type(screen.getByLabelText("Name"), name);
      expect(submit).toBeEnabled();
    }
    expect(screen.queryByText(i18n.t("common.createBlockedName"))).toBeNull();
  });

  // w4/m170: names PostgreSQL owns block Create with their own message.
  it("blocks a reserved database user or name before submit", async () => {
    const user = userEvent.setup();
    renderPage();
    await user.type(await screen.findByLabelText("Name"), "shop-db");
    const submit = submitButton();

    await user.type(screen.getByLabelText("User"), "postgres");
    expect(
      screen.getByText(
        "“postgres” is reserved by PostgreSQL. Choose another name.",
      ),
    ).toBeInTheDocument();
    expect(submit).toBeDisabled();
    // The action bar says why, in the shared create-page words.
    expect(
      screen.getByText(i18n.t("common.createBlockedFields")),
    ).toBeInTheDocument();

    await user.clear(screen.getByLabelText("User"));
    await user.type(screen.getByLabelText("Database"), "template1");
    expect(
      screen.getByText(
        "“template1” is reserved by PostgreSQL. Choose another name.",
      ),
    ).toBeInTheDocument();
    expect(submit).toBeDisabled();

    await user.clear(screen.getByLabelText("Database"));
    await user.type(screen.getByLabelText("Database"), "orders_data");
    expect(submit).toBeEnabled();
  });

  it('submits with the default plan, omits an unset version (default -> ""), and lands on the new database', async () => {
    const user = userEvent.setup();
    const { router } = renderPage();
    await user.type(await screen.findByLabelText("Name"), "shop-db");
    await user.click(submitButton());

    // plan defaults to the catalog's first tier; version "default" maps to "";
    // an empty disk field becomes 0 (operator applies the plan floor).
    expect(create).toHaveBeenCalledWith({
      name: "shop-db",
      databaseName: "",
      databaseUser: "",
      plan: "free",
      version: "",
      diskSizeGB: 0,
      public: true,
      highAvailability: false,
      environmentId: undefined,
    });
    await waitFor(() =>
      expect(router.state.location.pathname).toBe("/databases/dpg-returned-id"),
    );
  });

  it("keeps the form in place when create fails", async () => {
    create.mockResolvedValueOnce(null);
    const user = userEvent.setup();
    const { router } = renderPage();
    await user.type(await screen.findByLabelText("Name"), "friendly-name");
    await user.click(submitButton());

    await waitFor(() => expect(create).toHaveBeenCalled());
    expect(router.state.location.pathname).toBe("/new/database");
    expect(screen.getByLabelText("Name")).toHaveValue("friendly-name");
  });

  it("returns to the overview on Cancel", async () => {
    const user = userEvent.setup();
    const { router } = renderPage();
    await user.click(await screen.findByRole("button", { name: "Cancel" }));
    await waitFor(() => expect(router.state.location.pathname).toBe("/"));
  });

  // w4/553: as on Render, a new database is reachable from any address until
  // restricted; switching Public access off creates it private.
  it("creates a public database unless Public access is switched off", async () => {
    const user = userEvent.setup();
    const { container } = renderPage();
    const toggle = await screen.findByRole("switch", { name: "Public Access" });
    expect(toggle).toBeChecked();
    expect(container).toHaveTextContent("from any IP address");
    await user.click(toggle);
    expect(container).toHaveTextContent("External connections are blocked.");
    expect(toggle).toHaveAccessibleDescription(
      /External connections are blocked/,
    );
    await user.type(screen.getByLabelText("Name"), "private-db");
    await user.click(submitButton());
    expect(create).toHaveBeenCalledWith(
      expect.objectContaining({ name: "private-db", public: false }),
    );
  });

  // w2/m171: the ≥1-CPU plans offer high availability at create, as on
  // Render; a plan below 1 CPU shows the switch off, disabled and explained,
  // and never sends it — bex-api's own rule, read from the catalog.
  it("offers high availability only on a plan that supports it", async () => {
    const ONE_CPU: DatabaseInstanceTypeView = {
      ...BASIC,
      id: "1c-2g",
      name: "1c-2g",
      cpu: "1",
      memory: "2Gi",
      storageGB: 10,
      supportsHighAvailability: true,
      monthlyUsd: "28.00",
    };
    instanceTypesState.instanceTypes = [FREE, BASIC, ONE_CPU];
    const user = userEvent.setup();
    renderPage();
    await user.type(await screen.findByLabelText("Name"), "shop-db");

    const ha = () => screen.getByRole("switch", { name: "High Availability" });
    expect(ha()).toBeDisabled();
    expect(ha()).toHaveAccessibleDescription(
      /Free does not offer high availability\. Choose a plan with at least 1 CPU\./,
    );

    await pickPlan(user, /^1c-2g/);
    expect(ha()).toBeEnabled();
    await user.click(ha());
    expect(ha()).toBeChecked();

    // Back to a plan below 1 CPU: off, disabled, not sent.
    await pickPlan(user, /Basic 1GB/);
    expect(ha()).not.toBeChecked();
    expect(ha()).toBeDisabled();

    await pickPlan(user, /^1c-2g/);
    expect(ha()).toBeChecked();
    await user.click(submitButton());
    expect(create).toHaveBeenCalledWith(
      expect.objectContaining({ plan: "1c-2g", highAvailability: true }),
    );
  });

  // The switch remembers the choice across plans, but a create on a plan
  // below 1 CPU must never carry it: bex-api would refuse the whole create.
  it("does not send HA switched on for a 1-CPU plan when the create is on a smaller one", async () => {
    instanceTypesState.instanceTypes = [
      FREE,
      BASIC,
      {
        ...BASIC,
        id: "pro-4gb",
        name: "Pro 4GB",
        cpu: "1",
        memory: "4Gi",
        storageGB: 10,
        supportsHighAvailability: true,
      },
    ];
    const user = userEvent.setup();
    renderPage();
    await user.type(await screen.findByLabelText("Name"), "shop-db");

    await pickPlan(user, /Pro 4GB/);
    await user.click(screen.getByRole("switch", { name: "High Availability" }));
    await pickPlan(user, /Basic 1GB/);
    await user.click(submitButton());
    expect(create).toHaveBeenCalledWith(
      expect.objectContaining({ plan: "basic-1gb", highAvailability: false }),
    );
  });

  it("submits a free-form name trimmed, as bex-api takes it (w5/m153)", async () => {
    const user = userEvent.setup();
    renderPage();
    await user.type(await screen.findByLabelText("Name"), " Shop DB ");
    await user.click(submitButton());

    expect(create).toHaveBeenCalledWith(
      expect.objectContaining({ name: "Shop DB" }),
    );
  });

  it("validates and submits optional physical PostgreSQL names", async () => {
    const user = userEvent.setup();
    renderPage();
    await user.type(await screen.findByLabelText("Name"), "shop-db");
    await user.type(screen.getByLabelText("Database"), "Orders-Data");
    expect(submitButton()).toBeDisabled();
    expect(screen.getByLabelText("Database")).toHaveAttribute(
      "aria-invalid",
      "true",
    );

    await user.clear(screen.getByLabelText("Database"));
    await user.type(screen.getByLabelText("Database"), "orders_data");
    await user.type(screen.getByLabelText("User"), "orders_owner");
    await user.click(submitButton());

    expect(create).toHaveBeenCalledWith(
      expect.objectContaining({
        databaseName: "orders_data",
        databaseUser: "orders_owner",
      }),
    );
  });

  // w4/376: Free's disk is fixed at 1 GB. The row states it in words rather
  // than showing an input that looks editable but ignores typing, and a
  // "between 1 and 1 GB" range message never appears.
  it("states Free's fixed disk as text and never a one-value range error", async () => {
    const user = userEvent.setup();
    renderPage();
    await user.type(await screen.findByLabelText("Name"), "shop-db");

    expect(screen.queryByRole("spinbutton")).toBeNull();
    expect(
      screen.getByText(
        "This plan includes a fixed 1 GB disk. Choose a larger plan to set storage yourself.",
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText(/between 1 and 1 GB/i)).toBeNull();
    expect(submitButton()).toBeEnabled();
    await user.click(submitButton());
    expect(create).toHaveBeenCalledWith(
      expect.objectContaining({ plan: "free", diskSizeGB: 0 }),
    );
  });

  it("uses the selected tier's disk bounds and retains the entered size across plan changes", async () => {
    instanceTypesState.instanceTypes = [FREE, { ...BASIC, maxStorageGB: 25 }];
    const user = userEvent.setup();
    renderPage();
    await user.type(await screen.findByLabelText("Name"), "shop-db");
    await pickPlan(user, /Basic 1GB/);
    const disk = () => screen.getByRole("spinbutton", { name: "Storage" });
    expect(disk()).toHaveAttribute("min", "5");
    expect(disk()).toHaveAttribute("max", "25");
    await user.type(disk(), "26");
    expect(screen.getByText(/between 5 and 25 GB/i)).toBeVisible();
    expect(submitButton()).toBeDisabled();
    await user.clear(disk());
    await user.type(disk(), "25");
    expect(submitButton()).toBeEnabled();
    // Free's disk is fixed (w4/376): it reads 1 GB and creates, while the
    // entered size is kept for when a paid plan is chosen again.
    await pickPlan(user, /^Free/);
    expect(screen.queryByRole("spinbutton")).toBeNull();
    expect(
      screen.getByText(
        "This plan includes a fixed 1 GB disk. Choose a larger plan to set storage yourself.",
      ),
    ).toBeInTheDocument();
    expect(submitButton()).toBeEnabled();
    await pickPlan(user, /Basic 1GB/);
    expect(disk()).toHaveValue(25);
    expect(create).not.toHaveBeenCalled();
  });

  it("waits for the selected tier's storage limit before allowing creation", async () => {
    instanceTypesState.instanceTypes = [{ ...BASIC, maxStorageGB: null }];
    const user = userEvent.setup();
    renderPage();
    await user.type(await screen.findByLabelText("Name"), "shop-db");
    expect(screen.getByRole("spinbutton", { name: "Storage" })).toBeDisabled();
    expect(submitButton()).toBeDisabled();
    expect(create).not.toHaveBeenCalled();
  });

  it("submits the selected environment so the backend auto-joins its project", async () => {
    const user = userEvent.setup();
    renderPage();
    await user.type(await screen.findByLabelText("Name"), "shop-db");
    await user.click(screen.getByRole("combobox", { name: "Project" }));
    await user.click(await screen.findByRole("option", { name: "Commerce" }));
    await user.click(screen.getByRole("combobox", { name: "Environment" }));
    await user.click(await screen.findByRole("option", { name: "Production" }));
    await user.click(submitButton());

    expect(create).toHaveBeenCalledWith(
      expect.objectContaining({ environmentId: "env-1" }),
    );
  });

  // w4/156: the plans state their price at the point of choice, and the action
  // bar repeats the selected plan's, as on Render.
  it("prices each plan and summarizes the selected one in the action bar", async () => {
    const user = userEvent.setup();
    const { container } = renderPage();
    expect(
      await screen.findByRole("radio", { name: /Basic 1GB.*\$14\.00\/month/ }),
    ).toBeInTheDocument();
    const footer = container.querySelector<HTMLElement>("[data-page-footer]")!;
    expect(footer).toHaveTextContent("$0.00/month");
    await pickPlan(user, /Basic 1GB/);
    expect(footer).toHaveTextContent("$14.00/month");
    expect(footer).toHaveTextContent("1 GB RAM");
  });

  it("shows the workspace cap inline with its upgrade", async () => {
    createState.capLimit = "Your workspace has reached its Postgres limit.";
    const user = userEvent.setup();
    const { router } = renderPage();
    const alert = await screen.findByText(
      "Your workspace has reached its Postgres limit.",
    );
    await user.click(
      within(alert.closest('[role="alert"]') as HTMLElement).getByRole(
        "button",
      ),
    );
    await waitFor(() =>
      expect(router.state.location.pathname).toBe("/workspace/settings"),
    );
  });
});

// w4/m218 (a785): a failed catalog left an empty plan selector with no
// explanation. It now says so and re-reads in place, keeping the typed name.
describe("NewDatabasePage with a failed plan catalog (w4/m218)", () => {
  it("explains the failure, retries in place and keeps the name", async () => {
    instanceTypesState.instanceTypes = [];
    instanceTypesState.error = new Error("catalog unavailable");
    retryCatalog.mockClear();
    const user = userEvent.setup();
    renderPage();
    await user.type(await screen.findByLabelText("Name"), "shop-db");
    const alert = screen.getByRole("alert");
    expect(alert).toHaveTextContent("Plans couldn't load");
    expect(submitButton()).toBeDisabled();
    await user.click(within(alert).getByRole("button", { name: "Try again" }));
    expect(retryCatalog).toHaveBeenCalledOnce();

    instanceTypesState.instanceTypes = [FREE, BASIC];
    instanceTypesState.error = undefined;
    await user.type(screen.getByLabelText("Name"), "2"); // any render
    expect(screen.getByLabelText("Name")).toHaveValue("shop-db2");
    expect(screen.queryByText("Plans couldn't load")).toBeNull();
    expect(submitButton()).toBeEnabled();
  });

  it("previews the collapsed list's inert rows while the catalog first loads", async () => {
    instanceTypesState.instanceTypes = [];
    instanceTypesState.loading = true;
    renderPage();
    await screen.findByLabelText("Name");
    expect(screen.queryByRole("radiogroup")).toBeNull();
    expect(screen.queryByText("Plans couldn't load")).toBeNull();
    expect(submitButton()).toBeDisabled();
  });
});
