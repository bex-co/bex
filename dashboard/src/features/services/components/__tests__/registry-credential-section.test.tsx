import { beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from "@tanstack/react-router";
import { RegistryCredentialSection } from "@/features/services/components/registry-credential-section";
import { RegistryCredentialSelect } from "@/features/services/components/registry-credential-select";
import type { RegistryCredentialView } from "@/features/registry-credentials/types";

beforeAll(() => {
  if (!Element.prototype.hasPointerCapture) {
    Element.prototype.hasPointerCapture = () => false;
  }
  if (!Element.prototype.releasePointerCapture) {
    Element.prototype.releasePointerCapture = () => {};
  }
});

const credentialsState: {
  credentials: RegistryCredentialView[];
  loading: boolean;
  error: Error | undefined;
} = {
  credentials: [],
  loading: false,
  error: undefined,
};

vi.mock(
  "@/features/registry-credentials/hooks/use-registry-credentials",
  () => ({
    useRegistryCredentials: () => ({
      ...credentialsState,
      refetch: vi.fn(),
    }),
  }),
);

const setRegistryCredential = vi.fn(async () => true);
vi.mock("@/features/services/hooks/use-registry-credential", () => ({
  useRegistryCredential: () => ({ setRegistryCredential, busy: false }),
}));

const PRIVATE_GHCR: RegistryCredentialView = {
  id: "rgc-private",
  name: "Private GHCR",
  host: "ghcr.io",
  username: "robot",
  expiresAt: null,
  status: "active",
  createdAt: null,
};
const BACKUP_GHCR: RegistryCredentialView = {
  ...PRIVATE_GHCR,
  id: "rgc-backup",
  name: "Backup GHCR",
};

function renderInRouter(component: React.ReactNode) {
  const rootRoute = createRootRoute();
  const indexRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/",
    component: () => component,
  });
  const settingsRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/settings",
    component: () => null,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([indexRoute, settingsRoute]),
    history: createMemoryHistory({ initialEntries: ["/"] }),
    context: { client: {} as never, session: null },
  });
  return render(<RouterProvider router={router} />);
}

beforeEach(() => {
  credentialsState.credentials = [PRIVATE_GHCR, BACKUP_GHCR];
  credentialsState.loading = false;
  credentialsState.error = undefined;
  setRegistryCredential.mockReset();
  setRegistryCredential.mockResolvedValue(true);
});

describe("RegistryCredentialSelect", () => {
  it("maps the None option to an explicit empty string", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    renderInRouter(
      <RegistryCredentialSelect
        id="credential"
        value="rgc-private"
        onValueChange={onChange}
        description="Private image credential"
      />,
    );

    await user.click(
      await screen.findByRole("combobox", { name: "Registry credential" }),
    );
    await user.click(
      screen.getByRole("option", { name: "No credential — public image" }),
    );

    expect(onChange).toHaveBeenCalledWith("");
  });

  it("links an empty credential list to the settings panel", async () => {
    credentialsState.credentials = [];
    renderInRouter(
      <RegistryCredentialSelect
        id="credential"
        value=""
        onValueChange={vi.fn()}
        description="Private image credential"
      />,
    );

    expect(
      await screen.findByText("No stored registry credentials are available."),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Manage registry credentials" }),
    ).toHaveAttribute("href", "/settings#registry-credentials");
  });
});

describe("RegistryCredentialSection", () => {
  it("changes the binding and refreshes after success", async () => {
    const user = userEvent.setup();
    const onChanged = vi.fn();
    renderInRouter(
      <RegistryCredentialSection
        serviceId="srv-web"
        registryCredentialId="rgc-private"
        onChanged={onChanged}
      />,
    );

    await user.click(
      await screen.findByRole("combobox", { name: "Registry credential" }),
    );
    await user.click(screen.getByRole("option", { name: /Backup GHCR/ }));
    await user.click(screen.getByRole("button", { name: "Save credential" }));

    expect(setRegistryCredential).toHaveBeenCalledWith("srv-web", "rgc-backup");
    expect(onChanged).toHaveBeenCalledOnce();
  });

  it("clears the current binding with the backend's explicit empty value", async () => {
    const user = userEvent.setup();
    renderInRouter(
      <RegistryCredentialSection
        serviceId="srv-web"
        registryCredentialId="rgc-private"
      />,
    );

    await user.click(
      await screen.findByRole("combobox", { name: "Registry credential" }),
    );
    await user.click(
      screen.getByRole("option", { name: "No credential — public image" }),
    );
    await user.click(screen.getByRole("button", { name: "Save credential" }));

    expect(setRegistryCredential).toHaveBeenCalledWith("srv-web", "");
  });
});
