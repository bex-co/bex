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

  // codex-security target #4: a non-http(s) URL must never become a live href.
  it("renders a non-http(s) URL as inert text, not a link", () => {
    render(
      <PlatformSubdomainRow
        serviceId="my-svc"
        url="javascript:alert(document.domain)"
        renderSubdomainPolicy="enabled"
      />,
    );

    expect(screen.queryByRole("link")).not.toBeInTheDocument();
    // The value is still shown (as escaped text), just not clickable.
    expect(
      screen.getByText("javascript:alert(document.domain)"),
    ).toBeInTheDocument();
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

  // w4/142: the server keeps the subdomain on until a custom domain is
  // verified; the switch says so before the click instead of a refusal toast
  // that also leaked the field name.
  it("explains before the click that a pending domain can't replace the subdomain yet", () => {
    render(
      <PlatformSubdomainRow
        serviceId="my-svc"
        url="https://web.onbex.co"
        renderSubdomainPolicy="enabled"
        domains={[
          { name: "qa-20260925.example.com", ownershipVerified: false },
        ]}
      />,
    );

    const toggle = screen.getByRole("switch", {
      name: "Toggle platform subdomain",
    });
    expect(toggle).toBeDisabled();
    expect(toggle).toHaveAccessibleDescription(
      "qa-20260925.example.com must finish verifying before the platform subdomain can be turned off.",
    );
  });

  it("lets a service with a verified domain turn the subdomain off", () => {
    render(
      <PlatformSubdomainRow
        serviceId="my-svc"
        url="https://web.onbex.co"
        renderSubdomainPolicy="enabled"
        domains={[{ name: "www.example.com", ownershipVerified: true }]}
      />,
    );
    expect(
      screen.getByRole("switch", { name: "Toggle platform subdomain" }),
    ).toBeEnabled();
    expect(screen.queryByText(/must finish verifying/)).not.toBeInTheDocument();
  });

  it("never blocks turning a disabled subdomain back on", () => {
    render(
      <PlatformSubdomainRow
        serviceId="my-svc"
        url="https://web.onbex.co"
        renderSubdomainPolicy="disabled"
        domains={[]}
      />,
    );
    expect(
      screen.getByRole("switch", { name: "Toggle platform subdomain" }),
    ).toBeEnabled();
  });
});
