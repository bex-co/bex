import { describe, it, expect, vi, beforeAll, beforeEach } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  RouterProvider,
  createRouter,
  createRootRoute,
  createRoute,
  createMemoryHistory,
} from "@tanstack/react-router";
import { NewKeyValuePage } from "../keyvalue.new";
import type { KeyValueInstanceTypeView } from "@/features/keyvalue/types";

beforeAll(() => {
  if (!Element.prototype.hasPointerCapture) {
    Element.prototype.hasPointerCapture = () => false;
  }
  if (!Element.prototype.releasePointerCapture) {
    Element.prototype.releasePointerCapture = () => {};
  }
});

const instanceTypesState: {
  instanceTypes: KeyValueInstanceTypeView[];
  loading: boolean;
  error: Error | undefined;
} = { instanceTypes: [], loading: false, error: undefined };

vi.mock("@/features/keyvalue/hooks/use-key-value-instance-types", () => ({
  useKeyValueInstanceTypes: () => instanceTypesState,
}));

const create = vi.fn();
vi.mock("@/features/keyvalue/hooks/use-create-key-value", () => ({
  useCreateKeyValue: () => ({ create, busy: false }),
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

const FREE: KeyValueInstanceTypeView = {
  id: "free",
  name: "Free",
  cpu: "100m",
  memory: "128Mi",
  storageGB: 1,
};
const STARTER: KeyValueInstanceTypeView = {
  id: "starter",
  name: "Starter",
  cpu: "100m",
  memory: "256Mi",
  storageGB: 1,
};

function renderPage() {
  const rootRoute = createRootRoute();
  const newRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/keyvalue/new",
    component: NewKeyValuePage,
  });
  const detailRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/keyvalue/$keyValueId",
    component: () => <p>Key Value destination</p>,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([newRoute, detailRoute]),
    history: createMemoryHistory({ initialEntries: ["/keyvalue/new"] }),
    context: { client: {} as never, session: null },
  });
  return { ...render(<RouterProvider router={router} />), router };
}

beforeEach(() => {
  instanceTypesState.instanceTypes = [FREE, STARTER];
  create.mockReset();
  create.mockResolvedValue("red-returned-id");
});

describe("NewKeyValuePage", () => {
  it("validates the name and keeps submit disabled until it's valid", async () => {
    const user = userEvent.setup();
    renderPage();

    const submit = await screen.findByRole("button", {
      name: "Create Key Value Instance",
    });
    expect(submit).toBeDisabled(); // empty name

    // Invalid: can't start with a hyphen — the inline error shows, submit stays off.
    await user.type(screen.getByLabelText("Name"), "-bad");
    expect(
      screen.getByText(/can't start or end with a hyphen/i),
    ).toBeInTheDocument();
    expect(submit).toBeDisabled();

    // Valid name enables submit.
    await user.clear(screen.getByLabelText("Name"));
    await user.type(screen.getByLabelText("Name"), "cache-1");
    expect(submit).toBeEnabled();
  });

  it("renders a plan-tier card per catalog instance type, defaulting to the first", async () => {
    renderPage();
    const radiogroup = await screen.findByRole("radiogroup");
    expect(screen.getByText("Free")).toBeInTheDocument();
    expect(screen.getByText("Starter")).toBeInTheDocument();
    expect(
      within(radiogroup).getByRole("radio", { name: /Free/ }),
    ).toHaveAttribute("aria-checked", "true");
  });

  it('submits with the selected plan and omits an unset version (default -> "")', async () => {
    const user = userEvent.setup();
    renderPage();

    await user.type(await screen.findByLabelText("Name"), "cache-1");
    await user.click(screen.getByRole("radio", { name: /Starter/ }));
    await user.click(
      screen.getByRole("button", { name: "Create Key Value Instance" }),
    );

    expect(create).toHaveBeenCalledWith({
      name: "cache-1",
      plan: "starter",
      version: "",
      public: false,
      maxmemoryPolicy: "allkeys-lru",
      persistenceMode: "journal-snapshot",
      environmentId: undefined,
    });
  });

  it("submits the selected environment so the backend auto-joins its project", async () => {
    const user = userEvent.setup();
    renderPage();

    await user.type(await screen.findByLabelText("Name"), "cache-1");
    await user.click(screen.getByRole("combobox", { name: "Project" }));
    await user.click(await screen.findByRole("option", { name: "Commerce" }));
    await user.click(screen.getByRole("combobox", { name: "Environment" }));
    await user.click(await screen.findByRole("option", { name: "Production" }));
    await user.click(
      screen.getByRole("button", { name: "Create Key Value Instance" }),
    );

    expect(create).toHaveBeenCalledWith(
      expect.objectContaining({ environmentId: "env-1" }),
    );
  });

  it("does not force persistence off on the Free plan — it defaults durable and stays editable (w6/m127)", async () => {
    const user = userEvent.setup();
    renderPage();

    await user.type(await screen.findByLabelText("Name"), "cache-free");
    // Free is the default (first) plan, so no plan click needed.

    // The persistence control is NOT locked on Free (bex's free tier has a 1 GB
    // disk — the old "no persistent disk" premise was false), and the false hint
    // is gone.
    expect(
      screen.getByRole("combobox", { name: "Persistence Mode" }),
    ).not.toBeDisabled();
    expect(
      screen.queryByText(/Free plan has no persistent disk/i),
    ).not.toBeInTheDocument();

    await user.click(
      screen.getByRole("button", { name: "Create Key Value Instance" }),
    );

    // A free store now created through the form matches the API create path,
    // which yields journal_snapshot — not the silently non-durable `off`.
    expect(create).toHaveBeenCalledWith(
      expect.objectContaining({
        plan: "free",
        persistenceMode: "journal-snapshot",
      }),
    );
  });

  it("lands on the immutable id returned by a successful create", async () => {
    const user = userEvent.setup();
    const { router } = renderPage();

    await user.type(await screen.findByLabelText("Name"), "friendly-cache");
    await user.click(
      screen.getByRole("button", { name: "Create Key Value Instance" }),
    );

    await waitFor(() =>
      expect(router.state.location.pathname).toBe("/keyvalue/red-returned-id"),
    );
  });

  it("does not navigate when create fails", async () => {
    create.mockResolvedValueOnce(null);
    const user = userEvent.setup();
    const { router } = renderPage();

    await user.type(await screen.findByLabelText("Name"), "friendly-cache");
    await user.click(
      screen.getByRole("button", { name: "Create Key Value Instance" }),
    );

    await waitFor(() => expect(create).toHaveBeenCalled());
    expect(router.state.location.pathname).toBe("/keyvalue/new");
    expect(screen.getByLabelText("Name")).toHaveValue("friendly-cache");
  });
});
