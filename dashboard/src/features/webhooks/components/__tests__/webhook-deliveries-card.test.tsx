import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { formatDateTime } from "@/common/lib/format";
import { WebhookDeliveriesCard } from "@/features/webhooks/components/webhook-deliveries-card";
import type { WebhookDeliveryView } from "@/features/webhooks/types";

const deliveries: WebhookDeliveryView[] = [
  {
    id: "whd-success",
    eventId: "evt-success",
    eventType: "deploy_started",
    serviceId: "srv-api",
    status: "delivered",
    attemptNumber: 1,
    statusCode: 204,
    transportError: "",
    responseBody: "accepted",
    requestBody: '{"type":"deploy_started"}',
    sentAt: "2026-08-16T12:00:00Z",
    nextAttemptAt: null,
    parentStatus: "delivered",
    cursor: "one",
  },
  {
    id: "whd-failed",
    eventId: "evt-failed",
    eventType: "build_ended",
    serviceId: "srv-worker",
    status: "failed",
    attemptNumber: 2,
    statusCode: 502,
    transportError: "endpoint answered 502",
    responseBody: "upstream unavailable",
    requestBody: '{"type":"build_ended"}',
    sentAt: "2026-08-15T12:00:00Z",
    nextAttemptAt: "2026-08-15T12:01:00Z",
    parentStatus: "pending",
    cursor: "two",
  },
];

const loadMore = vi.fn();
const refresh = vi.fn();
const resend = vi.fn();
let hasMore = false;
let currentRole = "admin";
let currentWorkspaceId = "tea-1";
const { useWebhookDeliveries } = vi.hoisted(() => ({
  useWebhookDeliveries: vi.fn(),
}));

vi.mock("@/features/webhooks/hooks/use-webhook-deliveries", () => ({
  useWebhookDeliveries,
}));
vi.mock("@/features/webhooks/hooks/use-resend-webhook-delivery", () => ({
  useResendWebhookDelivery: () => ({
    resend,
    resendingAttemptId: null,
  }),
}));
vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({
    currentWorkspace: { id: currentWorkspaceId, role: currentRole },
    currentWorkspaceId,
  }),
}));
vi.mock("@/features/capabilities/hooks/use-capabilities", () => ({
  useCapabilities: () => ({ canManage: currentRole === "admin", loaded: true }),
}));

describe("WebhookDeliveriesCard", () => {
  beforeEach(() => {
    currentRole = "admin";
    currentWorkspaceId = "tea-1";
    hasMore = false;
    loadMore.mockReset();
    refresh.mockReset();
    refresh.mockResolvedValue(undefined);
    resend.mockReset();
    resend.mockResolvedValue(true);
    useWebhookDeliveries.mockImplementation(
      (_endpointId: string, filter: { status?: string } = {}) => ({
        deliveries: filter.status
          ? deliveries.filter((delivery) => delivery.status === filter.status)
          : deliveries,
        loading: false,
        loadingMore: false,
        error: undefined,
        hasMore,
        loadMore,
        refresh,
      }),
    );
  });

  it("renders immutable attempts with source links, exact time, and request/response evidence", async () => {
    const user = userEvent.setup();
    render(<WebhookDeliveriesCard endpointId="whk-1" endpointEnabled={true} />);

    expect(screen.getByText("Deploy Started")).toBeInTheDocument();
    expect(screen.getByText("Build Ended")).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Open source event evt-failed" }),
    ).toHaveAttribute("href", "/v1/events/evt-failed?ownerId=tea-1");
    expect(
      screen.getByText(formatDateTime("2026-08-15T12:00:00Z")!),
    ).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: /HTTP 502/ }));
    expect(screen.getByText("Request payload")).toBeInTheDocument();
    expect(screen.getByText('{"type":"build_ended"}')).toBeInTheDocument();
    expect(screen.getByText("upstream unavailable")).toBeInTheDocument();
    expect(screen.getByText("endpoint answered 502")).toBeInTheDocument();
    expect(screen.getByText(/Next automatic attempt:/)).toBeInTheDocument();
  });

  it("encodes the selected workspace in source-event links", () => {
    currentWorkspaceId = "tea-workspace+one";
    render(<WebhookDeliveriesCard endpointId="whk-1" endpointEnabled={true} />);

    expect(
      screen.getByRole("link", { name: "Open source event evt-failed" }),
    ).toHaveAttribute(
      "href",
      "/v1/events/evt-failed?ownerId=tea-workspace%2Bone",
    );
  });

  it("requests failed deliveries from the server", async () => {
    const user = userEvent.setup();
    render(<WebhookDeliveriesCard endpointId="whk-1" endpointEnabled={true} />);
    await user.click(screen.getByRole("tab", { name: "Failed" }));
    expect(screen.getByText("Build Ended")).toBeInTheDocument();
    expect(screen.queryByText("Deploy Started")).not.toBeInTheDocument();
    expect(useWebhookDeliveries).toHaveBeenLastCalledWith("whk-1", {
      status: "failed",
      sentAfter: undefined,
      sentBefore: undefined,
    });
  });

  it("does not crawl every history page when a filter is active", async () => {
    hasMore = true;
    loadMore.mockReset();
    const user = userEvent.setup();
    render(<WebhookDeliveriesCard endpointId="whk-1" endpointEnabled={true} />);
    await user.click(screen.getByRole("tab", { name: "Failed" }));
    expect(loadMore).not.toHaveBeenCalled();
    hasMore = false;
  });

  it("confirms an admin resend and refreshes the newest page without a navigation", async () => {
    const user = userEvent.setup();
    render(<WebhookDeliveriesCard endpointId="whk-1" endpointEnabled={true} />);

    await user.click(screen.getByRole("button", { name: "Resend" }));
    const dialog = screen.getByRole("alertdialog");
    expect(
      within(dialog).getByText(/same source event and request payload/i),
    ).toBeInTheDocument();
    await user.click(within(dialog).getByRole("button", { name: "Resend" }));

    await waitFor(() =>
      expect(resend).toHaveBeenCalledWith("whk-1", "whd-failed"),
    );
    expect(refresh).toHaveBeenCalledTimes(1);
  });

  it("does not expose Resend to a read-only workspace member", () => {
    currentRole = "viewer";
    render(<WebhookDeliveriesCard endpointId="whk-1" endpointEnabled={true} />);
    expect(
      screen.queryByRole("button", { name: "Resend" }),
    ).not.toBeInTheDocument();
  });

  it("does not expose Resend while the endpoint is disabled", () => {
    render(
      <WebhookDeliveriesCard endpointId="whk-1" endpointEnabled={false} />,
    );
    expect(
      screen.queryByRole("button", { name: "Resend" }),
    ).not.toBeInTheDocument();
  });
});
