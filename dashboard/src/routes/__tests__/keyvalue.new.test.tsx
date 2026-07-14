import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, within } from "@testing-library/react";
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
    path: "/",
    component: NewKeyValuePage,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([newRoute]),
    history: createMemoryHistory({ initialEntries: ["/"] }),
    context: { client: {} as never, session: null },
  });
  return render(<RouterProvider router={router} />);
}

beforeEach(() => {
  instanceTypesState.instanceTypes = [FREE, STARTER];
  create.mockReset();
  create.mockResolvedValue("cache-1");
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
    });
  });

  it("forces persistence to off on the Free plan (Render parity, w5/011)", async () => {
    const user = userEvent.setup();
    renderPage();

    await user.type(await screen.findByLabelText("Name"), "cache-free");
    // Free is the default (first) plan, so no plan click needed.
    await user.click(
      screen.getByRole("button", { name: "Create Key Value Instance" }),
    );

    expect(create).toHaveBeenCalledWith(
      expect.objectContaining({ plan: "free", persistenceMode: "off" }),
    );
  });
});
