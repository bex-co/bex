import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { PlatformSubdomainSection } from "@/features/services/components/platform-subdomain-section";

describe("PlatformSubdomainSection", () => {
  it("shows the service URL as an external link", () => {
    render(<PlatformSubdomainSection url="https://web.onbex.co" />);

    const link = screen.getByRole("link", { name: /web\.onbex\.co/ });
    expect(link).toHaveAttribute("href", "https://web.onbex.co");
    expect(link).toHaveAttribute("target", "_blank");
    // the "always on" badge (bex keeps the platform subdomain reachable)
    expect(screen.getByText("Always enabled")).toBeInTheDocument();
  });

  it("shows a pending note when the service has no URL yet", () => {
    render(<PlatformSubdomainSection url={null} />);

    expect(screen.queryByRole("link")).not.toBeInTheDocument();
    expect(
      screen.getByText(/platform URL is assigned once the service is running/),
    ).toBeInTheDocument();
  });
});
