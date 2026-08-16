import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MaintenanceModeSection } from "@/features/services/components/maintenance-mode-section";

// Stub the mutation hook so tests don't need an Apollo context.
const mockSetMaintenanceMode = vi.fn().mockResolvedValue(true);
vi.mock("@/features/services/hooks/use-maintenance-mode", () => ({
  useMaintenanceMode: () => ({
    setMaintenanceMode: mockSetMaintenanceMode,
    busy: false,
  }),
}));

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
});
