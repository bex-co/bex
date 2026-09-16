import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { config } from "@/config/config";
import { formatDateTime } from "@/common/lib/format";
import { WebhookDeliveriesCard } from "@/features/webhooks/components/webhook-deliveries-card";
import type { WebhookDeliveryView } from "@/features/webhooks/types";
import type { ServiceView } from "@/features/services/types";
import { hydrateAcrossBoundary } from "@/test/hydration";

const deliveries: WebhookDeliveryView[] = [
  {
    id: "whd-success",
    eventId: "evt-success",
    eventType: "deploy_started",
    serviceId: "srv-api",
    serviceName: "api",
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
    // Recorded at send time; this service has since been deleted, so it is
    // absent from the live service list below.
    serviceName: "worker",
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

// The workspace's live services: srv-api still exists, srv-worker was deleted
// after its delivery was sent.
const liveService = {
  id: "srv-api",
  name: "api",
  type: "web_service",
} as unknown as ServiceView;
let liveServices: ServiceView[] = [liveService];
let servicesLoading = false;

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
// Keeps the rendered href assertable — the shared Link stub in other suites
// drops `to`/`params`, which is exactly what this suite needs to check.
vi.mock("@tanstack/react-router", () => ({
  Link: ({
    to,
    params,
    children,
    ...rest
  }: {
    to: string;
    params: Record<string, string>;
    children: React.ReactNode;
  }) => (
    <a
      href={Object.entries(params).reduce(
        (path, [key, value]) => path.replace(`$${key}`, value),
        to,
      )}
      {...rest}
    >
      {children}
    </a>
  ),
}));
vi.mock("@/features/services/hooks/use-services", () => ({
  useServices: () => ({
    services: liveServices,
    loading: servicesLoading,
    error: undefined,
    refetch: vi.fn(),
  }),
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
    liveServices = [liveService];
    servicesLoading = false;
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
    ).toHaveAttribute(
      "href",
      `${config.apiBaseUrl}/v1/events/evt-failed?ownerId=tea-1`,
    );
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

  // w2/m96/t004: the Service column used to be a bare truncated `srv-…`, which
  // reads as an opaque token. The recorded name is the label; the id stays on
  // screen (and selectable, so it is still copyable) as secondary text.
  it("links a delivery's recorded service name and keeps the id visible", () => {
    render(<WebhookDeliveriesCard endpointId="whk-1" endpointEnabled={true} />);

    expect(screen.getByRole("link", { name: "api" })).toHaveAttribute(
      "href",
      "/services/srv-api",
    );
    expect(screen.getByText("srv-api")).toBeInTheDocument();
  });

  it("keeps the recorded name of a since-deleted service, unlinked", () => {
    render(<WebhookDeliveriesCard endpointId="whk-1" endpointEnabled={true} />);

    // srv-worker is absent from the live service list — deleted since this
    // attempt was delivered. Its recorded name survives in the payload.
    expect(screen.getByText("worker")).toBeInTheDocument();
    expect(screen.getByText("(deleted)")).toBeInTheDocument();
    expect(
      screen.queryByRole("link", { name: /worker/ }),
    ).not.toBeInTheDocument();
    expect(screen.getByText("srv-worker")).toBeInTheDocument();
  });

  // Fail-open: "absent from a list we have not loaded" is not a deletion.
  it("does not call a service deleted while the service list is still loading", () => {
    servicesLoading = true;
    liveServices = [];
    render(<WebhookDeliveriesCard endpointId="whk-1" endpointEnabled={true} />);

    expect(screen.getByText("api")).toBeInTheDocument();
    expect(screen.queryByText("(deleted)")).not.toBeInTheDocument();
  });

  it("falls back to the id when an older attempt recorded no name", () => {
    useWebhookDeliveries.mockReturnValue({
      deliveries: [{ ...deliveries[0], serviceName: "" }],
      loading: false,
      loadingMore: false,
      error: undefined,
      hasMore: false,
      loadMore,
      refresh,
    });
    render(<WebhookDeliveriesCard endpointId="whk-1" endpointEnabled={true} />);

    expect(screen.getByText("srv-api")).toBeInTheDocument();
    expect(screen.queryByText("(deleted)")).not.toBeInTheDocument();
  });

  it("encodes the selected workspace in source-event links", () => {
    currentWorkspaceId = "tea-workspace+one";
    render(<WebhookDeliveriesCard endpointId="whk-1" endpointEnabled={true} />);

    expect(
      screen.getByRole("link", { name: "Open source event evt-failed" }),
    ).toHaveAttribute(
      "href",
      `${config.apiBaseUrl}/v1/events/evt-failed?ownerId=tea-workspace%2Bone`,
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

  // w6/m102 regression: the live repro. A delivery that landed ~55s before the
  // page loaded crosses the "now" → "1m" bucket boundary between the blocking
  // SSR render and client hydration, so the same node renders different text on
  // each side — React error #418. RelativeAge carries the guard. (The exact
  // timestamp beside it diverges by timezone instead, a different mechanism
  // this probe can't see — same process, same TZ on both passes — and a
  // different owner: w6/m107.)
  it("hydrates a just-sent delivery across an age boundary without a React #418", () => {
    const sentAt = "2026-08-26T12:00:00.000Z";
    const serverNow = Date.parse(sentAt) + 55_000; // renders "now"
    const clientNow = Date.parse(sentAt) + 61_000; // renders "1m"
    useWebhookDeliveries.mockReturnValue({
      deliveries: [{ ...deliveries[0], sentAt }],
      loading: false,
      loadingMore: false,
      error: undefined,
      hasMore: false,
      loadMore,
      refresh,
    });

    const { html, recovered } = hydrateAcrossBoundary(
      <WebhookDeliveriesCard endpointId="whk-1" endpointEnabled />,
      { serverNow, clientNow },
    );

    // The delivery age really is rendered in the blocking SSR pass — the
    // precondition for a hydration mismatch to be possible here at all.
    expect(html).toContain(">now<");
    expect(recovered).toEqual([]);
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
