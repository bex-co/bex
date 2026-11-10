import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { WebhookDetailHeader } from "@/features/webhooks/components/webhook-detail-header";
import type { WebhookEndpointView } from "@/features/webhooks/types";

// The back button is a router <Link>; the header itself never navigates in
// these tests, so a bare anchor stands in.
vi.mock("@tanstack/react-router", () => ({
  Link: ({
    children,
    to,
    ...props
  }: React.ComponentProps<"a"> & { to?: string }) => (
    <a href={to} {...props}>
      {children}
    </a>
  ),
}));

function endpoint(
  overrides: Partial<WebhookEndpointView> = {},
): WebhookEndpointView {
  return {
    id: "whe-1",
    name: "deploy alerts",
    url: "https://example.com/hook",
    eventTypes: ["deploy_started"],
    enabled: true,
    disabledReason: "",
    secret: "",
    // Zone-less input parses as local time, so the long-form date holds in
    // any runner TZ.
    createdAt: "2026-07-16T12:00:00",
    createdBy: "alice@example.com",
    ...overrides,
  };
}

describe("WebhookDetailHeader", () => {
  it("shows the provenance line with the long-form created date", () => {
    render(<WebhookDetailHeader endpoint={endpoint()} />);
    expect(
      screen.getByText("Created by alice@example.com on July 16, 2026"),
    ).toBeInTheDocument();
  });

  it("drops the creator but keeps the date when the API recorded none", () => {
    render(<WebhookDetailHeader endpoint={endpoint({ createdBy: "" })} />);
    expect(screen.getByText("Created on July 16, 2026")).toBeInTheDocument();
  });

  // w21/022: the link's accessible name carries the visible state.
  it("names the status link with the state and translates a known reason", () => {
    render(
      <WebhookDetailHeader
        endpoint={endpoint({
          enabled: false,
          disabledReason:
            "disabled automatically after repeated delivery failures",
        })}
      />,
    );
    const link = screen.getByRole("link", { name: "Status: Disabled" });
    expect(link.querySelector("[data-slot=badge]")).toHaveAttribute(
      "title",
      "Disabled automatically after repeated delivery failures.",
    );
  });

  it("shows no reason for a manual disable", () => {
    render(
      <WebhookDetailHeader
        endpoint={endpoint({
          enabled: false,
          disabledReason: "disabled manually",
        })}
      />,
    );
    const link = screen.getByRole("link", { name: "Status: Disabled" });
    expect(link.querySelector("[data-slot=badge]")).not.toHaveAttribute(
      "title",
    );
  });

  it("omits the provenance line entirely without a created date", () => {
    render(<WebhookDetailHeader endpoint={endpoint({ createdAt: null })} />);
    expect(screen.queryByText(/^Created/)).not.toBeInTheDocument();
  });

  it("links the destination and settings status and uses human event labels", () => {
    render(<WebhookDetailHeader endpoint={endpoint()} />);
    expect(
      screen.getByRole("link", { name: "Status: Enabled" }),
    ).toHaveAttribute("href", "/webhook/$webhookId/settings");
    expect(screen.getByRole("link", { name: endpoint().url })).toHaveAttribute(
      "href",
      endpoint().url,
    );
    expect(screen.getByText("Deploy Started")).toBeInTheDocument();
    expect(screen.queryByText("deploy_started")).not.toBeInTheDocument();
  });

  it("renders an empty stored event filter as future-inclusive All events", () => {
    render(<WebhookDetailHeader endpoint={endpoint({ eventTypes: [] })} />);
    expect(screen.getByText("All events")).toBeInTheDocument();
  });

  it("uses the resolved creator identity instead of the stored subject", () => {
    render(
      <WebhookDetailHeader
        endpoint={endpoint({ createdBy: "identity-123" })}
        creatorIdentity="alice@example.com"
      />,
    );
    expect(
      screen.getByText("Created by alice@example.com on July 16, 2026"),
    ).toBeInTheDocument();
    expect(screen.queryByText(/identity-123/)).not.toBeInTheDocument();
  });
});
