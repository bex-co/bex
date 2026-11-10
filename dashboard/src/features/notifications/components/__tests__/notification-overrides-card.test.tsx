import type { ReactNode } from "react";
import { describe, expect, it, vi, beforeEach } from "vitest";
import { render, screen, within } from "@testing-library/react";
import { NotificationOverridesCard } from "@/features/notifications/components/notification-overrides-card";

const query = {
  data: undefined as unknown,
  loading: false,
  error: undefined as Error | undefined,
};
vi.mock("@apollo/client/react", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@apollo/client/react")>()),
  useQuery: () => query,
}));
vi.mock("@tanstack/react-router", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@tanstack/react-router")>()),
  Link: ({
    to,
    params,
    children,
  }: {
    to: string;
    params: { serviceId: string };
    children: ReactNode;
  }) => <a href={to.replace("$serviceId", params.serviceId)}>{children}</a>,
}));

const service = (
  id: string,
  name: string,
  type: string,
  notificationsToSend: string,
) => ({
  __typename: "Service",
  id,
  name,
  displayName: "",
  type,
  notificationsToSend,
});

beforeEach(() => {
  query.data = undefined;
  query.loading = false;
  query.error = undefined;
});

// w3/m305: Render's Notification Overrides — the services that do not use the
// workspace default, each linking to its own setting.
describe("NotificationOverridesCard", () => {
  it("lists only overriding services, by name, linking to each one's settings", () => {
    query.data = {
      services: [
        service("srv-b", "worker", "background_worker", "none"),
        service("srv-a", "api", "web_service", "default"),
        service("srv-c", "docs", "static_site", "all"),
      ],
    };
    render(<NotificationOverridesCard ownerId="tea-1" />);

    const items = within(screen.getByRole("list")).getAllByRole("listitem");
    expect(items.map((li) => li.textContent)).toEqual([
      expect.stringMatching(/^docs/),
      expect.stringMatching(/^worker/),
    ]);
    expect(screen.getByRole("link", { name: "docs" })).toHaveAttribute(
      "href",
      "/static/srv-c/settings",
    );
    expect(screen.getByRole("link", { name: "worker" })).toHaveAttribute(
      "href",
      "/services/srv-b/settings",
    );
    expect(screen.queryByText("api")).not.toBeInTheDocument();
  });

  it("says when no service overrides the default", () => {
    query.data = {
      services: [service("srv-a", "api", "web_service", "default")],
    };
    render(<NotificationOverridesCard ownerId="tea-1" />);
    expect(
      screen.getByText("No service overrides the workspace default."),
    ).toBeInTheDocument();
  });

  it("keeps its frame while loading", () => {
    query.loading = true;
    const { container } = render(<NotificationOverridesCard ownerId="tea-1" />);
    expect(container.querySelector('[aria-hidden="true"]')).not.toBeNull();
    expect(screen.queryByRole("list")).not.toBeInTheDocument();
  });
});
