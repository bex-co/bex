import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
  saveAllowList: vi.fn(async () => true),
}));

vi.mock("@/features/services/hooks/use-service-networking", () => ({
  useServiceNetworking: () => ({
    saveAllowList: mocks.saveAllowList,
    saving: false,
  }),
}));

import { ServiceNetworkingPanel } from "@/features/services/components/service-networking-panel";

beforeEach(() => mocks.saveAllowList.mockClear());

describe("ServiceNetworkingPanel", () => {
  it("renders and resubmits descriptions without dropping them", async () => {
    const user = userEvent.setup();
    render(
      <ServiceNetworkingPanel
        serviceId="srv-web"
        currentAllowList={[
          { cidrBlock: "203.0.113.0/24", description: "office" },
        ]}
      />,
    );

    expect(screen.getByDisplayValue("office")).toBeInTheDocument();
    await user.type(
      screen.getByPlaceholderText("203.0.113.0/24"),
      "10.0.0.0/8",
    );
    const descriptionInput = screen
      .getAllByPlaceholderText("Description (optional)")
      .at(-1);
    expect(descriptionInput).toBeDefined();
    await user.type(descriptionInput!, "vpn");
    await user.click(screen.getByRole("button", { name: "Add" }));
    await user.click(screen.getByRole("button", { name: "Save" }));

    expect(mocks.saveAllowList).toHaveBeenCalledWith("srv-web", [
      { cidrBlock: "203.0.113.0/24", description: "office" },
      { cidrBlock: "10.0.0.0/8", description: "vpn" },
    ]);
  });

  it("edits descriptions and reorders entries before saving", async () => {
    const user = userEvent.setup();
    render(
      <ServiceNetworkingPanel
        serviceId="srv-web"
        currentAllowList={[
          { cidrBlock: "203.0.113.0/24", description: "office" },
          { cidrBlock: "10.0.0.0/8", description: "vpn" },
        ]}
      />,
    );

    const firstDescription = screen.getByRole("textbox", {
      name: "Description (optional) 1",
    });
    await user.clear(firstDescription);
    await user.type(firstDescription, "headquarters");
    await user.click(
      screen.getByRole("button", { name: "Move 10.0.0.0/8 up" }),
    );
    await user.click(screen.getByRole("button", { name: "Save" }));

    expect(mocks.saveAllowList).toHaveBeenCalledWith("srv-web", [
      { cidrBlock: "10.0.0.0/8", description: "vpn" },
      { cidrBlock: "203.0.113.0/24", description: "headquarters" },
    ]);
  });

  it("keeps an invalid CIDR visible and does not submit it", async () => {
    const user = userEvent.setup();
    render(
      <ServiceNetworkingPanel serviceId="srv-web" currentAllowList={[]} />,
    );
    const input = screen.getByPlaceholderText("203.0.113.0/24");
    await user.type(input, "not-a-cidr");
    await user.click(screen.getByRole("button", { name: "Add" }));

    expect(input).toHaveValue("not-a-cidr");
    expect(screen.getByRole("alert")).toHaveTextContent(
      "Enter a valid IPv4 or IPv6 CIDR block.",
    );
    expect(screen.getByRole("button", { name: "Save" })).toBeDisabled();
    expect(mocks.saveAllowList).not.toHaveBeenCalled();
  });

  it("clears the complete allowlist", async () => {
    const user = userEvent.setup();
    render(
      <ServiceNetworkingPanel
        serviceId="srv-web"
        currentAllowList={[{ cidrBlock: "203.0.113.0/24", description: null }]}
      />,
    );
    await user.click(
      screen.getByRole("button", { name: "Remove 203.0.113.0/24" }),
    );
    await user.click(screen.getByRole("button", { name: "Save" }));
    expect(mocks.saveAllowList).toHaveBeenCalledWith("srv-web", []);
  });
});
