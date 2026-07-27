import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { PlatformSubdomainRow } from "@/features/services/components/platform-subdomain-section";

// Stub the mutation hook so tests don't need an Apollo context.
const mockSetSubdomainPolicy = vi.fn().mockResolvedValue(true);
vi.mock("@/features/services/hooks/use-subdomain-policy", () => ({
  useSubdomainPolicy: () => ({
    setSubdomainPolicy: mockSetSubdomainPolicy,
    busy: false,
  }),
}));

describe("PlatformSubdomainRow", () => {
  it("shows the service URL as a link when policy is enabled", () => {
    render(
      <PlatformSubdomainRow
        serviceId="my-svc"
        url="https://web.onbex.co"
        renderSubdomainPolicy="enabled"
      />,
    );

    const link = screen.getByRole("link", { name: /web\.onbex\.co/ });
    expect(link).toHaveAttribute("href", "https://web.onbex.co");
    expect(link).toHaveAttribute("target", "_blank");
    expect(screen.getByText("Enabled")).toBeInTheDocument();
  });

  it("shows a pending note when enabled but service has no URL yet", () => {
    render(
      <PlatformSubdomainRow
        serviceId="my-svc"
        url={null}
        renderSubdomainPolicy="enabled"
      />,
    );

    expect(screen.queryByRole("link")).not.toBeInTheDocument();
    expect(
      screen.getByText(/platform URL is assigned once the service is running/),
    ).toBeInTheDocument();
  });

  it("shows disabled note when policy is disabled", () => {
    render(
      <PlatformSubdomainRow
        serviceId="my-svc"
        url="https://web.onbex.co"
        renderSubdomainPolicy="disabled"
      />,
    );

    expect(screen.queryByRole("link")).not.toBeInTheDocument();
    expect(
      screen.getByText(/only reachable via custom domains/),
    ).toBeInTheDocument();
    expect(screen.getByText("Disabled")).toBeInTheDocument();
  });

  it("defaults to enabled when policy is null", () => {
    render(
      <PlatformSubdomainRow
        serviceId="my-svc"
        url="https://web.onbex.co"
        renderSubdomainPolicy={null}
      />,
    );

    const link = screen.getByRole("link", { name: /web\.onbex\.co/ });
    expect(link).toBeInTheDocument();
  });

  it("calls setSubdomainPolicy when the switch is toggled", async () => {
    const user = userEvent.setup();
    render(
      <PlatformSubdomainRow
        serviceId="my-svc"
        url="https://web.onbex.co"
        renderSubdomainPolicy="enabled"
      />,
    );

    const toggle = screen.getByRole("switch");
    await user.click(toggle);
    expect(mockSetSubdomainPolicy).toHaveBeenCalledWith("my-svc", "disabled");
  });
});
