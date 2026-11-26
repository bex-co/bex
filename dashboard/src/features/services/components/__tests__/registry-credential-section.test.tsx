import { beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
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
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";

beforeAll(() => {
  if (!Element.prototype.hasPointerCapture) {
    Element.prototype.hasPointerCapture = () => false;
  }
  if (!Element.prototype.releasePointerCapture) {
    Element.prototype.releasePointerCapture = () => {};
  }
  if (!Element.prototype.scrollIntoView) {
    Element.prototype.scrollIntoView = () => {};
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

const refetch = vi.fn();
vi.mock(
  "@/features/registry-credentials/hooks/use-registry-credentials",
  () => ({
    useRegistryCredentials: () => ({
      ...credentialsState,
      refetch,
    }),
  }),
);

// w4/m670: a create lands in the list, as the refetch would bring it.
const createCredential = vi.fn(async (input: { name?: string }) => {
  credentialsState.credentials = [
    ...credentialsState.credentials,
    { ...PRIVATE_GHCR, id: "rgc-added", name: input.name ?? "" },
  ];
  return "rgc-added";
});
vi.mock(
  "@/features/registry-credentials/hooks/use-create-registry-credential",
  () => ({
    useCreateRegistryCredential: () => ({
      create: createCredential,
      busy: false,
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
  registry: "GITHUB",
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
    path: "/workspace/settings",
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
  createCredential.mockClear();
  refetch.mockClear();
});

/** Add credential → fill Render's dialog for GHCR → Add Credential. */
async function addFromDropdown(
  user: ReturnType<typeof userEvent.setup>,
  combobox: HTMLElement,
) {
  await user.click(combobox);
  const options = screen.getAllByRole("option");
  expect(options[0]).toHaveTextContent("Add credential");
  await user.click(options[0]);
  const dialog = await screen.findByRole("dialog");
  await user.type(within(dialog).getByLabelText("Name"), "Added GHCR");
  await user.click(within(dialog).getByRole("combobox", { name: "Registry" }));
  await user.click(await screen.findByRole("option", { name: "GitHub" }));
  await user.type(within(dialog).getByLabelText("Username"), "robot");
  await user.type(
    within(dialog).getByLabelText("Personal Access Token"),
    "ghp_token",
  );
  await user.click(
    within(dialog).getByRole("button", { name: "Add Credential" }),
  );
}

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
      await screen.findByRole("combobox", { name: "Registry Credential" }),
    );
    await user.click(
      screen.getByRole("option", { name: "No credential — public image" }),
    );

    expect(onChange).toHaveBeenCalledWith("");
  });

  it("adds a credential in place and selects it, refreshing the list", async () => {
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
    await addFromDropdown(
      user,
      await screen.findByRole("combobox", { name: "Registry Credential" }),
    );

    expect(createCredential).toHaveBeenCalledWith(
      expect.objectContaining({ registry: "GITHUB", name: "Added GHCR" }),
    );
    // Choosing Add credential never handed the sentinel over; the new id did.
    expect(onChange.mock.calls).toEqual([["rgc-added"]]);
    expect(refetch).toHaveBeenCalled();
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("cancelling the dialog keeps the selection", async () => {
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
    const combobox = await screen.findByRole("combobox", {
      name: "Registry Credential",
    });
    await user.click(combobox);
    await user.click(screen.getByRole("option", { name: "Add credential" }));
    const dialog = await screen.findByRole("dialog");
    // The closing list leaves focus in the dialog, not on its own trigger.
    await waitFor(() =>
      expect(within(dialog).getByLabelText("Name")).toHaveFocus(),
    );
    await user.click(within(dialog).getByRole("button", { name: "Cancel" }));

    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(onChange).not.toHaveBeenCalled();
    expect(combobox).toHaveTextContent("Private GHCR (ghcr.io)");
  });

  // bex-api lets only `can_manage` members create a credential.
  it("offers no Add credential to a member who may not create one", async () => {
    vi.mocked(useCapabilities).mockReturnValue({
      ...vi.mocked(useCapabilities)(),
      canManage: false,
    });
    const user = userEvent.setup();
    renderInRouter(
      <RegistryCredentialSelect
        id="credential"
        value=""
        onValueChange={vi.fn()}
        description="Private image credential"
      />,
    );
    await user.click(
      await screen.findByRole("combobox", { name: "Registry Credential" }),
    );
    expect(
      screen.getAllByRole("option").map((option) => option.textContent),
    ).toEqual([
      "No credential — public image",
      "Private GHCR (ghcr.io)",
      "Backup GHCR (ghcr.io)",
    ]);
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
    ).toHaveAttribute("href", "/workspace/settings#registry-credentials");
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
      await screen.findByRole("combobox", { name: "Registry Credential" }),
    );
    await user.click(screen.getByRole("option", { name: /Backup GHCR/ }));
    await user.click(screen.getByRole("button", { name: "Save credential" }));

    expect(setRegistryCredential).toHaveBeenCalledWith("srv-web", "rgc-backup");
    expect(onChanged).toHaveBeenCalledOnce();
  });

  // w4/m670: Settings → Image adds one in place too; Save binds it.
  it("adds a credential from Settings and saves it as the binding", async () => {
    const user = userEvent.setup();
    renderInRouter(
      <RegistryCredentialSection
        serviceId="srv-web"
        registryCredentialId="rgc-private"
      />,
    );
    await addFromDropdown(
      user,
      await screen.findByRole("combobox", { name: "Registry Credential" }),
    );
    expect(
      screen.getByRole("combobox", { name: "Registry Credential" }),
    ).toHaveTextContent("Added GHCR (ghcr.io)");
    await user.click(screen.getByRole("button", { name: "Save credential" }));

    expect(setRegistryCredential).toHaveBeenCalledWith("srv-web", "rgc-added");
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
      await screen.findByRole("combobox", { name: "Registry Credential" }),
    );
    await user.click(
      screen.getByRole("option", { name: "No credential — public image" }),
    );
    await user.click(screen.getByRole("button", { name: "Save credential" }));

    expect(setRegistryCredential).toHaveBeenCalledWith("srv-web", "");
  });
});
