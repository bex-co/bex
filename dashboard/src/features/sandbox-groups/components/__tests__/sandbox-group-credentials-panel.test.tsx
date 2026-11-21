import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { mockCapabilities } from "@/test/mocks/capabilities";

const capabilitiesState = vi.hoisted(() => ({
  value: {} as ReturnType<typeof mockCapabilities>,
}));
vi.mock("@/features/capabilities/hooks/use-capabilities", () => ({
  useCapabilities: () => capabilitiesState.value,
}));

const credentialsQuery = vi.hoisted(() => ({
  credentials: [] as unknown[],
  loading: false,
  error: undefined as Error | undefined,
  unavailable: false,
  refetch: vi.fn(),
}));
const versionsQuery = vi.hoisted(() => ({
  versions: [] as unknown[],
  loading: false,
  error: undefined as Error | undefined,
  refetch: vi.fn(),
}));
const mutationSpies = vi.hoisted(() => ({
  createCredential: vi.fn(),
  updateCredential: vi.fn(),
  deleteCredential: vi.fn(),
  restoreCredential: vi.fn(),
  busyId: null as string | null,
  submitting: false,
}));

vi.mock("@/common/components/dashboard-layout", () => ({
  DashboardLayout: ({ children }: { children: unknown }) => children,
}));

vi.mock(
  "@/features/sandbox-groups/hooks/use-sandbox-group-credentials",
  () => ({
    useSandboxGroupCredentials: () => credentialsQuery,
    useSandboxGroupCredentialVersions: () => versionsQuery,
    useSandboxGroupCredentialMutations: () => mutationSpies,
  }),
);

import { SandboxGroupCredentialsPanel } from "@/features/sandbox-groups/components/sandbox-group-credentials-panel";
import {
  SandboxGroupCredentialsPanelSkeleton,
  SandboxGroupDetailRouteSkeleton,
  SandboxGroupsListRouteSkeleton,
} from "@/features/sandbox-groups/components/sandbox-group-skeletons";
import { ROUTE_SKELETON_MANIFEST } from "@/common/lib/route-skeleton-manifest";

const SAMPLE_CRED = {
  id: "openai-key",
  domain: "api.openai.com",
  enabled: true,
  methods: ["POST"],
  paths: ["/v1/**"],
  injection: {
    type: "header",
    name: "Authorization",
    template: "Bearer ${SECRET}",
  },
  version: {
    id: 2,
    updatedAt: "2026-11-20T12:00:00Z",
    updatedBy: "usr-admin",
    fingerprint: "abcdef0123456789".repeat(4),
    restoredFromVersionId: null,
  },
};

const SAMPLE_VERSIONS = [
  {
    id: 2,
    updatedAt: "2026-11-20T12:00:00Z",
    updatedBy: "usr-admin",
    fingerprint: "abcdef0123456789".repeat(4),
    restoredFromVersionId: null,
  },
  {
    id: 1,
    updatedAt: "2026-11-19T12:00:00Z",
    updatedBy: "usr-admin",
    fingerprint: "1122334455667788".repeat(4),
    restoredFromVersionId: null,
  },
];

beforeEach(() => {
  capabilitiesState.value = mockCapabilities({
    role: "ADMIN",
    canManage: true,
    canCreate: true,
  });
  credentialsQuery.credentials = [SAMPLE_CRED];
  credentialsQuery.loading = false;
  credentialsQuery.error = undefined;
  credentialsQuery.unavailable = false;
  credentialsQuery.refetch = vi.fn();

  versionsQuery.versions = SAMPLE_VERSIONS;
  versionsQuery.loading = false;
  versionsQuery.error = undefined;
  versionsQuery.refetch = vi.fn();

  mutationSpies.createCredential = vi.fn().mockResolvedValue({
    ok: true,
    data: SAMPLE_CRED,
    errorMessage: null,
  });
  mutationSpies.updateCredential = vi.fn().mockResolvedValue({
    ok: true,
    data: SAMPLE_CRED,
    errorMessage: null,
  });
  mutationSpies.deleteCredential = vi.fn().mockResolvedValue(true);
  mutationSpies.restoreCredential = vi.fn().mockResolvedValue(SAMPLE_CRED);
  mutationSpies.busyId = null;
  mutationSpies.submitting = false;
});

describe("SandboxGroupCredentialsPanel", () => {
  it("gives Admins create, edit (with domain & write-only secret), restore, and sudo-gated delete", async () => {
    const user = userEvent.setup();
    render(
      <SandboxGroupCredentialsPanel sandboxGroupId="sbg-1" ownerId="tea-1" />,
    );

    // Admin sees Add Credential button.
    const addBtn = screen.getByRole("button", { name: /add credential/i });
    await user.click(addBtn);

    const dialog = screen.getByRole("dialog");
    const secretInput = within(dialog).getByLabelText(
      /secret value/i,
    ) as HTMLInputElement;
    expect(secretInput.type).toBe("password");
    expect(secretInput.autocomplete).toBe("new-password");

    await user.type(within(dialog).getByLabelText(/credential id/i), "stripe-key");
    await user.type(
      within(dialog).getByLabelText(/destination domain/i),
      "api.stripe.com",
    );
    const rawSecret = "sk-live-super-secret-value-never-shown";
    await user.type(secretInput, rawSecret);
    await user.click(
      within(dialog).getByRole("button", { name: /create credential/i }),
    );

    expect(mutationSpies.createCredential).toHaveBeenCalledWith(
      expect.objectContaining({
        id: "stripe-key",
        domain: "api.stripe.com",
        value: rawSecret,
      }),
    );
    // Secret value is never displayed anywhere in the DOM after submit.
    expect(document.body.textContent).not.toContain(rawSecret);

    // Admin can open Versions and restore v1 via sudo confirmation phrase.
    const versionsButtons = screen.getAllByRole("button", {
      name: /view versions for openai-key/i,
    });
    await user.click(versionsButtons[0]);

    const versionsDialog = screen.getByRole("dialog");
    expect(within(versionsDialog).getByText("v2")).toBeInTheDocument();
    expect(within(versionsDialog).getByText("Current")).toBeInTheDocument();
    expect(within(versionsDialog).getByText("v1")).toBeInTheDocument();

    await user.click(
      within(versionsDialog).getByRole("button", { name: /restore/i }),
    );
    const confirmDialog = screen.getByRole("alertdialog");
    await user.type(
      within(confirmDialog).getByRole("textbox"),
      "sudo restore credential openai-key",
    );
    await user.click(
      within(confirmDialog).getByRole("button", { name: /restore/i }),
    );
    expect(mutationSpies.restoreCredential).toHaveBeenCalledWith("openai-key", 1);

    // Close versions dialog, then test sudo-gated delete.
    await user.keyboard("{Escape}");
    const deleteButtons = screen.getAllByRole("button", {
      name: /delete credential openai-key/i,
    });
    await user.click(deleteButtons[0]);
    const deleteAlert = screen.getByRole("alertdialog");
    await user.type(
      within(deleteAlert).getByRole("textbox"),
      "sudo delete credential openai-key",
    );
    await user.click(
      within(deleteAlert).getByRole("button", { name: /delete/i }),
    );
    expect(mutationSpies.deleteCredential).toHaveBeenCalledWith("openai-key");
  });

  it("restricts Developers to routing/injection/enabled edits and hides Admin-only controls", async () => {
    capabilitiesState.value = mockCapabilities({
      role: "DEVELOPER",
      canManage: false,
      canCreate: true,
    });
    const user = userEvent.setup();
    render(
      <SandboxGroupCredentialsPanel sandboxGroupId="sbg-1" ownerId="tea-1" />,
    );

    // No Add Credential or Delete buttons for Developers.
    expect(
      screen.queryByRole("button", { name: /add credential/i }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /delete credential/i }),
    ).not.toBeInTheDocument();

    // Developer can toggle enabled inline.
    const switches = screen.getAllByRole("switch", {
      name: /toggle enabled state for openai-key/i,
    });
    await user.click(switches[0]);
    expect(mutationSpies.updateCredential).toHaveBeenCalledWith(
      "openai-key",
      { enabled: false },
      { silentToast: true },
    );

    // Developer opening Edit sees disabled domain, no secret input, and submits ONLY routing/injection/enabled.
    const editButtons = screen.getAllByRole("button", {
      name: /edit credential openai-key/i,
    });
    await user.click(editButtons[0]);
    const dialog = screen.getByRole("dialog");
    expect(within(dialog).getByLabelText(/destination domain/i)).toBeDisabled();
    expect(within(dialog).queryByLabelText(/secret value/i)).not.toBeInTheDocument();

    await user.click(within(dialog).getByRole("button", { name: "GET" }));
    await user.click(
      within(dialog).getByRole("button", { name: /save changes/i }),
    );

    const [, payload] = mutationSpies.updateCredential.mock.calls.at(-1) ?? [];
    expect(payload).not.toHaveProperty("domain");
    expect(payload).not.toHaveProperty("value");
    expect(payload).toMatchObject({
      enabled: true,
      methods: ["POST", "GET"],
    });

    // Developer opening Versions does NOT see a Restore button.
    const versionsButtons = screen.getAllByRole("button", {
      name: /view versions for openai-key/i,
    });
    await user.click(versionsButtons[0]);
    expect(
      within(screen.getByRole("dialog")).queryByRole("button", {
        name: /restore/i,
      }),
    ).not.toBeInTheDocument();
  });

  it("renders read-only metadata for Viewers with no mutations", async () => {
    capabilitiesState.value = mockCapabilities({
      role: "VIEWER",
      canManage: false,
      canCreate: false,
    });
    const user = userEvent.setup();
    render(
      <SandboxGroupCredentialsPanel sandboxGroupId="sbg-1" ownerId="tea-1" />,
    );

    expect(
      screen.queryByRole("button", { name: /add credential/i }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /edit credential/i }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /delete credential/i }),
    ).not.toBeInTheDocument();
    expect(screen.queryByRole("switch")).not.toBeInTheDocument();
    expect(screen.getAllByText("Enabled").length).toBeGreaterThan(0);

    // Viewer can still inspect version history (read-only).
    const versionsButtons = screen.getAllByRole("button", {
      name: /view versions for openai-key/i,
    });
    await user.click(versionsButtons[0]);
    const versionsDialog = screen.getByRole("dialog");
    expect(within(versionsDialog).getByText("v2")).toBeInTheDocument();
    expect(
      within(versionsDialog).queryByRole("button", { name: /restore/i }),
    ).not.toBeInTheDocument();
  });

  it("rejects forbidden headers and invalid templates inline before mutating", async () => {
    const user = userEvent.setup();
    render(
      <SandboxGroupCredentialsPanel sandboxGroupId="sbg-1" ownerId="tea-1" />,
    );

    await user.click(screen.getByRole("button", { name: /add credential/i }));
    const dialog = screen.getByRole("dialog");

    await user.type(within(dialog).getByLabelText(/credential id/i), "bad-hdr");
    await user.type(
      within(dialog).getByLabelText(/destination domain/i),
      "api.example.com",
    );
    await user.type(
      within(dialog).getByLabelText(/secret value/i),
      "secret-val",
    );
    const nameInput = within(dialog).getByLabelText(/target name/i);
    await user.clear(nameInput);
    await user.type(nameInput, "Host");

    await user.click(
      within(dialog).getByRole("button", { name: /create credential/i }),
    );
    expect(mutationSpies.createCredential).not.toHaveBeenCalled();
    expect(within(dialog).getByRole("alert")).toHaveTextContent(
      /routing and connection headers/i,
    );
  });

  it("matches pending skeleton structure to ready layout for desktop and mobile widths", () => {
    const { container: pendingContainer } = render(
      <SandboxGroupCredentialsPanelSkeleton rows={2} />,
    );
    const { container: readyContainer } = render(
      <SandboxGroupCredentialsPanel sandboxGroupId="sbg-1" ownerId="tea-1" />,
    );

    // Both render a desktop table (hidden md:block) with 6 columns and a mobile
    // stacked card list (md:hidden) inside the Card.
    const pendingHeaders = pendingContainer.querySelectorAll("thead th");
    const readyHeaders = readyContainer.querySelectorAll("thead th");
    expect(pendingHeaders.length).toBe(6);
    expect(readyHeaders.length).toBe(6);

    const pendingMobile = pendingContainer.querySelector(".md\\:hidden");
    const readyMobile = readyContainer.querySelector(".md\\:hidden");
    expect(pendingMobile).not.toBeNull();
    expect(readyMobile).not.toBeNull();

    // Pending skeleton exposes no interactive buttons or switches.
    expect(pendingContainer.querySelectorAll("button").length).toBe(0);
    expect(pendingContainer.querySelectorAll("input").length).toBe(0);
  });

  it("route skeletons declare every region in ROUTE_SKELETON_MANIFEST", () => {
    for (const [path, El] of [
      ["/sandbox-groups", SandboxGroupsListRouteSkeleton],
      ["/sandbox-groups/$sandboxGroupId", SandboxGroupDetailRouteSkeleton],
    ] as const) {
      const entry = ROUTE_SKELETON_MANIFEST[path];
      expect(entry.kind).toBe("render");
      if (entry.kind !== "render") continue;
      const { container, unmount } = render(<El />);
      const root = container.querySelector("[data-route-skeleton]");
      expect(root).toHaveAttribute("data-route-skeleton", entry.shape);
      expect(root).toHaveAttribute("aria-hidden", "true");
      const regions = [
        ...container.querySelectorAll("[data-skeleton-region]"),
      ].map((n) => n.getAttribute("data-skeleton-region"));
      expect(regions).toEqual(entry.regions);
      unmount();
    }
  });
});
