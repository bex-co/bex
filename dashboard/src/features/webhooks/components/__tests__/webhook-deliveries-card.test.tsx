import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { WebhookDeliveriesCard } from "@/features/webhooks/components/webhook-deliveries-card";
import type { WebhookDeliveryView } from "@/features/webhooks/types";

const deliveries: WebhookDeliveryView[] = [
  {
    id: "whd-success",
    eventType: "deploy_started",
    serviceId: "srv-api",
    status: "delivered",
    attemptCount: 1,
    lastStatusCode: 204,
    lastError: "",
    responseBody: "accepted",
    sentAt: "2026-08-16T12:00:00Z",
    nextAttemptAt: null,
    deliveredAt: "2026-08-16T12:00:01Z",
    createdAt: "2026-08-16T11:59:59Z",
    cursor: "one",
  },
  {
    id: "whd-failed",
    eventType: "build_ended",
    serviceId: "srv-worker",
    status: "failed",
    attemptCount: 8,
    lastStatusCode: 502,
    lastError: "endpoint answered 502",
    responseBody: "upstream unavailable",
    sentAt: "2026-08-15T12:00:00Z",
    nextAttemptAt: null,
    deliveredAt: null,
    createdAt: "2026-08-15T11:59:59Z",
    cursor: "two",
  },
];

const loadMore = vi.fn();
let hasMore = false;
const { useWebhookDeliveries } = vi.hoisted(() => ({
  useWebhookDeliveries: vi.fn(),
}));

vi.mock("@/features/webhooks/hooks/use-webhook-deliveries", () => ({
  useWebhookDeliveries,
}));

describe("WebhookDeliveriesCard", () => {
  beforeEach(() => {
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
        refresh: vi.fn(),
      }),
    );
  });

  it("uses human event labels and exposes bounded response/error evidence", async () => {
    const user = userEvent.setup();
    render(<WebhookDeliveriesCard endpointId="whk-1" />);

    expect(screen.getByText("Deploy Started")).toBeInTheDocument();
    expect(screen.getByText("Build Ended")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: /HTTP 502/ }));
    expect(screen.getByText("upstream unavailable")).toBeInTheDocument();
    expect(screen.getByText("endpoint answered 502")).toBeInTheDocument();
  });

  it("requests failed deliveries from the server", async () => {
    const user = userEvent.setup();
    render(<WebhookDeliveriesCard endpointId="whk-1" />);
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
    render(<WebhookDeliveriesCard endpointId="whk-1" />);
    await user.click(screen.getByRole("tab", { name: "Failed" }));
    expect(loadMore).not.toHaveBeenCalled();
    hasMore = false;
  });
});
