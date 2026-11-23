import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MaintenanceModeSection } from "@/features/services/components/maintenance-mode-section";
import { toResourceSnapshot } from "@/features/capabilities/lib/resource-actions";
import type { ResourceActionsState } from "@/features/capabilities/hooks/use-resource-actions";

// Stub the mutation hook so tests don't need an Apollo context.
const mockSetMaintenanceMode = vi.fn().mockResolvedValue(true);
vi.mock("@/features/services/hooks/use-maintenance-mode", () => ({
  useMaintenanceMode: () => ({
    setMaintenanceMode: mockSetMaintenanceMode,
    busy: false,
  }),
}));

vi.mock("@/features/workspaces/context/hooks", async () => {
  const { mockWorkspaceContext } = await import("@/test/mocks/workspace");
  return mockWorkspaceContext();
});

// The switch reads the service's toggle decision from serverActions (w4/725).
let actions: ResourceActionsState;
vi.mock("@/features/capabilities/hooks/use-resource-actions", () => ({
  useServerActions: () => actions,
}));

/** The toggle row serverActions answers `caller` about service "web". */
function toggleAnswer(caller: "allowed" | "adminOnly" | "viewer") {
  const answer = {
    allowed: { outcome: "allowed", reason: null },
    adminOnly: {
      outcome: "denied",
      reason: "protected_environment_admin_required",
    },
    viewer: { outcome: "denied", reason: "insufficient_permission" },
  }[caller];
  return {
    status: "ready" as const,
    snapshot: toResourceSnapshot("tea-test", "web", [
      { action: "toggle_maintenance_mode", ...answer, precondition: null },
    ]),
    refresh: async () => undefined,
  };
}

const ADMIN_ONLY = "This action is limited to admins.";

beforeEach(() => {
  mockSetMaintenanceMode.mockClear();
  actions = toggleAnswer("allowed");
});

describe("MaintenanceModeSection", () => {
  it("shows disabled when maintenanceMode is null (never configured)", () => {
    render(
      <MaintenanceModeSection
        serviceId="web"
        serviceName="web"
        maintenanceMode={null}
      />,
    );
    expect(screen.getByText("Disabled")).toBeInTheDocument();
    expect(screen.getByRole("switch")).not.toBeChecked();
  });

  it("shows enabled and the custom uri when configured", () => {
    render(
      <MaintenanceModeSection
        serviceId="web"
        serviceName="web"
        maintenanceMode={{ enabled: true, uri: "https://status.example.com" }}
      />,
    );
    expect(screen.getByText("Enabled")).toBeInTheDocument();
    expect(screen.getByRole("switch")).toBeChecked();
    expect(screen.getByLabelText(/Custom maintenance page URL/)).toHaveValue(
      "https://status.example.com",
    );
  });

  it("disables maintenance controls on the free plan", () => {
    render(
      <MaintenanceModeSection
        serviceId="web"
        serviceName="web"
        plan="free"
        maintenanceMode={{ enabled: false, uri: "" }}
      />,
    );
    expect(screen.getByRole("switch")).toBeDisabled();
    expect(
      screen.getByText(
        "Maintenance mode is available on paid web service plans.",
      ),
    ).toBeInTheDocument();
  });

  it("confirms before enabling, then calls setMaintenanceMode(true, uri)", async () => {
    const user = userEvent.setup();
    render(
      <MaintenanceModeSection
        serviceId="web"
        serviceName="web"
        maintenanceMode={{ enabled: false, uri: "" }}
      />,
    );

    await user.click(screen.getByRole("switch"));
    // Confirm dialog appears — the mutation must not fire yet.
    expect(mockSetMaintenanceMode).not.toHaveBeenCalled();
    expect(
      screen.getByRole("heading", { name: /Enable maintenance mode\?/ }),
    ).toBeInTheDocument();

    await user.click(
      screen.getByRole("button", { name: "Enable maintenance mode" }),
    );
    expect(mockSetMaintenanceMode).toHaveBeenCalledWith("web", true, "");
  });

  it("disabling runs immediately, no confirm dialog", async () => {
    const user = userEvent.setup();
    render(
      <MaintenanceModeSection
        serviceId="web"
        serviceName="web"
        maintenanceMode={{ enabled: true, uri: "" }}
      />,
    );

    await user.click(screen.getByRole("switch"));
    expect(mockSetMaintenanceMode).toHaveBeenCalledWith("web", false, "");
    expect(
      screen.queryByRole("heading", { name: /Enable maintenance mode\?/ }),
    ).not.toBeInTheDocument();
  });

  it("URL field is disabled until Edit, Save gates on a dirty draft, and saves with the current enabled state", async () => {
    const user = userEvent.setup();
    render(
      <MaintenanceModeSection
        serviceId="web"
        serviceName="web"
        maintenanceMode={{ enabled: true, uri: "" }}
      />,
    );

    const input = screen.getByLabelText(/Custom maintenance page URL/);
    expect(input).toBeDisabled();

    await user.click(
      screen.getByRole("button", { name: "Edit maintenance page URL" }),
    );
    expect(input).toBeEnabled();
    // Save is disabled until the draft differs from the persisted URL.
    expect(screen.getByRole("button", { name: "Save changes" })).toBeDisabled();

    await user.type(input, "https://status.example.com/m");
    const save = screen.getByRole("button", { name: "Save changes" });
    expect(save).toBeEnabled();

    await user.click(save);
    expect(mockSetMaintenanceMode).toHaveBeenCalledWith(
      "web",
      true,
      "https://status.example.com/m",
      "uri",
    );
  });

  it("does not offer a URL edit affordance on the free plan", () => {
    render(
      <MaintenanceModeSection
        serviceId="web"
        serviceName="web"
        plan="free"
        maintenanceMode={{ enabled: false, uri: "" }}
      />,
    );

    expect(screen.getByLabelText(/Custom maintenance page URL/)).toBeDisabled();
    expect(
      screen.queryByRole("button", { name: "Edit maintenance page URL" }),
    ).not.toBeInTheDocument();
  });

  // In a protected environment only an Admin toggles maintenance mode, either
  // way (w4/725). Before, a Developer's switch was enabled, the confirmation
  // opened, and the verb answered 403 after it. An Admin, and a Developer
  // outside a protected environment, read "allowed" and keep the switch.
  describe.each([
    ["on", false],
    ["off", true],
  ])("turning maintenance mode %s", (_, enabled) => {
    it("is disabled with Render's reason for a Developer in a protected environment", async () => {
      actions = toggleAnswer("adminOnly");
      const user = userEvent.setup();
      render(
        <MaintenanceModeSection
          serviceId="web"
          serviceName="web"
          plan="standard"
          maintenanceMode={{ enabled, uri: "" }}
        />,
      );

      const toggle = screen.getByRole("switch");
      expect(toggle).toBeDisabled();
      expect(screen.getByText(ADMIN_ONLY)).toBeInTheDocument();
      await user.click(toggle);
      expect(
        screen.queryByRole("heading", { name: /Enable maintenance mode\?/ }),
      ).not.toBeInTheDocument();
      expect(mockSetMaintenanceMode).not.toHaveBeenCalled();
    });

    it("stays enabled for a caller the projection allows", async () => {
      const user = userEvent.setup();
      render(
        <MaintenanceModeSection
          serviceId="web"
          serviceName="web"
          plan="standard"
          maintenanceMode={{ enabled, uri: "" }}
        />,
      );

      expect(screen.queryByText(ADMIN_ONLY)).not.toBeInTheDocument();
      await user.click(screen.getByRole("switch"));
      if (!enabled) {
        await user.click(
          screen.getByRole("button", { name: "Enable maintenance mode" }),
        );
      }
      expect(mockSetMaintenanceMode).toHaveBeenCalledWith("web", !enabled, "");
    });
  });

  it("gives a Viewer the role reason", () => {
    actions = toggleAnswer("viewer");
    render(
      <MaintenanceModeSection
        serviceId="web"
        serviceName="web"
        maintenanceMode={{ enabled: false, uri: "" }}
      />,
    );
    expect(screen.getByRole("switch")).toBeDisabled();
    expect(
      screen.getByText(
        "Your role can’t perform this action. Ask a workspace admin.",
      ),
    ).toBeInTheDocument();
  });

  it("keeps the page URL editable for a Developer in a protected environment", async () => {
    actions = toggleAnswer("adminOnly");
    const user = userEvent.setup();
    render(
      <MaintenanceModeSection
        serviceId="web"
        serviceName="web"
        maintenanceMode={{ enabled: true, uri: "" }}
      />,
    );

    await user.click(
      screen.getByRole("button", { name: "Edit maintenance page URL" }),
    );
    await user.type(
      screen.getByLabelText(/Custom maintenance page URL/),
      "https://status.example.com/m",
    );
    await user.click(screen.getByRole("button", { name: "Save changes" }));
    expect(mockSetMaintenanceMode).toHaveBeenCalledWith(
      "web",
      true,
      "https://status.example.com/m",
      "uri",
    );
  });

  it("holds the switch while access is checked, keeping its hint", () => {
    actions = { status: "checking", refresh: async () => undefined };
    render(
      <MaintenanceModeSection
        serviceId="web"
        serviceName="web"
        maintenanceMode={{ enabled: false, uri: "" }}
      />,
    );
    expect(screen.getByRole("switch")).toBeDisabled();
    expect(screen.queryByText(ADMIN_ONLY)).not.toBeInTheDocument();
    expect(
      screen.queryByText("Checking whether you can perform this action…"),
    ).not.toBeInTheDocument();
  });

  it("explains a free service's switch by its plan, not the role", () => {
    actions = toggleAnswer("adminOnly");
    render(
      <MaintenanceModeSection
        serviceId="web"
        serviceName="web"
        plan="free"
        maintenanceMode={{ enabled: false, uri: "" }}
      />,
    );
    expect(screen.getByRole("switch")).toBeDisabled();
    expect(
      screen.getByText(
        "Maintenance mode is available on paid web service plans.",
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText(ADMIN_ONLY)).not.toBeInTheDocument();
  });
});
