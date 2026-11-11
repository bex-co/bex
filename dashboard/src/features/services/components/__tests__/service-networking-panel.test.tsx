import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
  saveAllowList: vi.fn(async () => true),
  plan: undefined as string | undefined,
}));

vi.mock("@/features/workspaces/context", () => ({
  useWorkspace: () => ({
    currentWorkspace: mocks.plan ? { plan: mocks.plan } : null,
  }),
}));

vi.mock("@/features/services/hooks/use-service-networking", () => ({
  useServiceNetworking: () => ({
    saveAllowList: mocks.saveAllowList,
    saving: false,
  }),
}));

import { ServiceNetworkingPanel } from "@/features/services/components/service-networking-panel";

const allowAll = [
  { cidrBlock: "0.0.0.0/0", description: "" },
  { cidrBlock: "::/0", description: "" },
];
const blocked =
  "No rules: all inbound traffic to this service is blocked. Add 0.0.0.0/0 to allow all source IPs.";

beforeEach(() => {
  mocks.saveAllowList.mockClear();
  mocks.plan = undefined;
});

describe("ServiceNetworkingPanel", () => {
  // w8/m137: inbound IP rules need the Scale plan; a legacy rule stays visible
  // and can only be replaced by the allow-all rule. An empty list would block
  // the service (w8/m157), which the plan refuses.
  it.each(["hobby", "pro"])("is open-only on the %s plan", async (plan) => {
    mocks.plan = plan;
    const user = userEvent.setup();
    render(
      <ServiceNetworkingPanel
        serviceId="srv-web"
        currentAllowList={[
          { cidrBlock: "203.0.113.0/24", description: "office" },
        ]}
      />,
    );
    expect(screen.getByText(/require the Scale plan/)).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Add allowlist entry" }),
    ).toBeNull();
    await user.click(screen.getByRole("button", { name: "Allow all traffic" }));
    expect(mocks.saveAllowList).toHaveBeenCalledWith("srv-web", allowAll);
  });

  it("shows an unrestricted service as open below the Scale plan, with nothing to press", () => {
    mocks.plan = "hobby";
    render(
      <ServiceNetworkingPanel
        serviceId="srv-web"
        currentAllowList={allowAll}
      />,
    );
    expect(screen.getByText("Open to all source IPs")).toBeVisible();
    expect(
      screen.queryByRole("button", { name: "Allow all traffic" }),
    ).toBeNull();
  });

  it("lets a blocked service be opened below the Scale plan", async () => {
    mocks.plan = "hobby";
    const user = userEvent.setup();
    render(
      <ServiceNetworkingPanel serviceId="srv-web" currentAllowList={[]} />,
    );
    expect(screen.getByText(blocked)).toBeVisible();
    await user.click(screen.getByRole("button", { name: "Allow all traffic" }));
    expect(mocks.saveAllowList).toHaveBeenCalledWith("srv-web", allowAll);
  });

  // w8/m157: as on Render, a service nobody restricted has the allow-all rule
  // and a service with no rule refuses every public request.
  it("lists the allow-all rule of an unrestricted service", () => {
    mocks.plan = "scale";
    render(
      <ServiceNetworkingPanel
        serviceId="srv-web"
        currentAllowList={allowAll}
      />,
    );
    expect(screen.getByDisplayValue("0.0.0.0/0")).toBeInTheDocument();
    expect(screen.getByDisplayValue("::/0")).toBeInTheDocument();
    expect(screen.queryByText(blocked)).toBeNull();
  });

  it("claims nothing before the list has loaded", () => {
    mocks.plan = "scale";
    render(
      <ServiceNetworkingPanel
        serviceId="srv-web"
        currentAllowList={undefined}
      />,
    );
    expect(screen.queryByText(blocked)).toBeNull();
    expect(screen.queryByRole("button", { name: "Save allowlist" })).toBeNull();
  });

  // A lone 0.0.0.0/0 saved before w8/m157 still refuses IPv6 sources, so it
  // is a rule to replace, not the open state.
  it("offers to replace a lone 0.0.0.0/0 below the Scale plan", async () => {
    mocks.plan = "hobby";
    const user = userEvent.setup();
    render(
      <ServiceNetworkingPanel
        serviceId="srv-web"
        currentAllowList={[{ cidrBlock: "0.0.0.0/0", description: "" }]}
      />,
    );
    expect(screen.queryByText("Open to all source IPs")).toBeNull();
    await user.click(screen.getByRole("button", { name: "Allow all traffic" }));
    expect(mocks.saveAllowList).toHaveBeenCalledWith("srv-web", allowAll);
  });

  it("says a service with no rule is blocked", () => {
    mocks.plan = "scale";
    render(
      <ServiceNetworkingPanel serviceId="srv-web" currentAllowList={[]} />,
    );
    expect(screen.getByText(blocked)).toBeVisible();
    expect(screen.queryByText("Open to all source IPs")).toBeNull();
  });

  it("stays editable on the Scale plan", () => {
    mocks.plan = "scale";
    render(
      <ServiceNetworkingPanel serviceId="srv-web" currentAllowList={[]} />,
    );
    expect(
      screen.getByRole("button", { name: "Add allowlist entry" }),
    ).toBeInTheDocument();
  });

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
    await user.click(
      screen.getByRole("button", { name: "Add allowlist entry" }),
    );
    await user.click(screen.getByRole("button", { name: "Save allowlist" }));

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
      name: "Description for rule 1",
    });
    await user.clear(firstDescription);
    await user.type(firstDescription, "headquarters");
    await user.click(
      screen.getByRole("button", { name: "Move 10.0.0.0/8 up" }),
    );
    await user.click(screen.getByRole("button", { name: "Save allowlist" }));

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
    await user.click(
      screen.getByRole("button", { name: "Add allowlist entry" }),
    );

    expect(input).toHaveValue("not-a-cidr");
    expect(screen.getByRole("alert")).toHaveTextContent(
      "Enter a valid IPv4 or IPv6 CIDR block.",
    );
    expect(
      screen.getByRole("button", { name: "Save allowlist" }),
    ).toBeDisabled();
    expect(mocks.saveAllowList).not.toHaveBeenCalled();
  });

  it("warns that removing the last rule blocks the service, before it is saved", async () => {
    const user = userEvent.setup();
    render(
      <ServiceNetworkingPanel
        serviceId="srv-web"
        currentAllowList={[{ cidrBlock: "203.0.113.0/24", description: null }]}
      />,
    );
    expect(screen.queryByText(blocked)).toBeNull();
    await user.click(
      screen.getByRole("button", { name: "Remove 203.0.113.0/24" }),
    );
    expect(screen.getByText(blocked)).toBeVisible();
    expect(mocks.saveAllowList).not.toHaveBeenCalled();
    await user.click(screen.getByRole("button", { name: "Save allowlist" }));
    expect(mocks.saveAllowList).toHaveBeenCalledWith("srv-web", []);
  });

  it("names the Cloudflare-proxied domains the allowlist cannot filter (w1/m171)", () => {
    render(
      <ServiceNetworkingPanel
        serviceId="srv-web"
        currentAllowList={[{ cidrBlock: "203.0.113.0/24", description: "" }]}
        proxiedDomains={["shop.example.com"]}
      />,
    );

    const notice = screen.getByTestId("ip-allow-list-proxied-notice");
    expect(notice).toHaveTextContent(
      "The allowlist can't see clients on 1 custom domain",
    );
    expect(notice).toHaveTextContent("shop.example.com");
    expect(notice).toHaveTextContent("DNS only (grey cloud)");
    // A warning, not a refusal: the editor stays and can still save.
    expect(
      screen.getByRole("button", { name: "Save allowlist" }),
    ).toBeInTheDocument();
  });

  it("renders no Cloudflare notice without proxied domains", () => {
    render(
      <ServiceNetworkingPanel
        serviceId="srv-web"
        currentAllowList={[{ cidrBlock: "203.0.113.0/24", description: "" }]}
        proxiedDomains={null}
      />,
    );
    expect(screen.queryByTestId("ip-allow-list-proxied-notice")).toBeNull();
  });
});
