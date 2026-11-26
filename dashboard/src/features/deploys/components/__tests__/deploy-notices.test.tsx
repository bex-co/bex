import { afterEach, describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import i18n from "@/i18n/init";
import type { DeployView } from "@/features/deploys/hooks/use-deploy";
import type { ServiceView } from "@/features/services/types";
import { ServiceBaseProvider } from "@/features/services/lib/service-base";
import {
  DeployNotices,
  FREE_PLANS_URL,
  HEALTH_CHECK_DOCS_URL,
  TROUBLESHOOT_DOCS_URL,
} from "../deploy-notices";

// The deploy page's Notices (w1/m659), Render's words and links as
// docs/render-artifacts/deploy-detail-page.md § Notices records them.

vi.mock("@tanstack/react-router", () => ({
  Link: ({
    to,
    params,
    hash,
    children,
    ...rest
  }: {
    to: string;
    params?: Record<string, string>;
    hash?: string;
    children?: React.ReactNode;
    [key: string]: unknown;
  }) => {
    const path = Object.entries(params ?? {}).reduce(
      (acc, [key, value]) => acc.replace(`$${key}`, value),
      to,
    );
    return (
      <a href={hash ? `${path}#${hash}` : path} {...rest}>
        {children}
      </a>
    );
  },
}));

const queryData: Record<string, unknown> = {};
const queryOptions: Record<string, { skip?: boolean; variables?: unknown }> =
  {};
vi.mock("@apollo/client/react", () => ({
  useQuery: vi.fn(
    (
      doc: { definitions?: Array<{ name?: { value?: string } }> },
      options: { skip?: boolean; variables?: unknown },
    ) => {
      const name = doc.definitions?.[0]?.name?.value ?? "";
      queryOptions[name] = options;
      return { data: options.skip ? undefined : queryData[name] };
    },
  ),
}));

afterEach(() => {
  for (const key of Object.keys(queryData)) delete queryData[key];
  for (const key of Object.keys(queryOptions)) delete queryOptions[key];
});

function deploy(over: Partial<DeployView> = {}): DeployView {
  return {
    id: "dep-1",
    status: "live",
    trigger: "manual",
    image: "",
    imageSha: "",
    rollbackOf: "",
    buildAvailable: null,
    commitId: "",
    commitMessage: "",
    commitCreatedAt: null,
    createdAt: "2026-11-26T00:00:00Z",
    updatedAt: "2026-11-26T00:01:00Z",
    startedAt: null,
    finishedAt: "2026-11-26T00:01:00Z",
    preDeployStatus: "",
    initialDeployHookStatus: "",
    initialDeployHookFinishedAt: null,
    failureReason: "",
    cancelReason: "",
    stallReason: "",
    failureReasonTemplate: null,
    stallReasonTemplate: null,
    cancelReasonTemplate: null,
    ...over,
  };
}

const service = (over: Partial<ServiceView> = {}) =>
  ({
    plan: "starter",
    healthCheckPath: null,
    port: 10000,
    ...over,
  }) as ServiceView;

function renderNotices(
  d: DeployView,
  svc: ServiceView = service(),
  base: "/services" | "/static" = "/services",
) {
  return render(
    <ServiceBaseProvider value={base}>
      <DeployNotices
        serviceId="srv-1"
        deploy={d}
        service={svc}
        ownerId="tea-1"
      />
    </ServiceBaseProvider>,
  );
}

const notices = () =>
  within(screen.getByRole("region", { name: "Notices" }))
    .getAllByRole("listitem")
    .map((li) => li.textContent);

describe("DeployNotices", () => {
  it("renders no section for a deploy with nothing to say, or while it loads", () => {
    const { container } = renderNotices(deploy());
    expect(container).toBeEmptyDOMElement();
    const loading = render(
      <DeployNotices serviceId="srv-1" deploy={null} service={service()} />,
    );
    expect(loading.container).toBeEmptyDOMElement();
  });

  it("says a deploy cleared the build cache, was a rollback, or moved to the free plan", () => {
    renderNotices(
      deploy({
        clearCache: true,
        isTransitionToFreeTier: true,
        rollbackOf: "dep-target",
      }),
      service(),
      "/static",
    );
    expect(notices()).toEqual([
      "Build cache cleared",
      "Updated to a free compute plan",
      "Rolled back to dep-target",
    ]);
    expect(screen.getByRole("link", { name: "dep-target" })).toHaveAttribute(
      "href",
      "/static/srv-1/deploys/dep-target",
    );
  });

  it("links a Blueprint sync's deploy to its Blueprint", () => {
    renderNotices(
      deploy({
        trigger: "blueprint",
        blueprintId: "exs-abc123",
        blueprintSyncId: "exe-def456",
      }),
    );
    expect(notices()).toEqual(["Synced from exs-abc123"]);
    expect(screen.getByRole("link", { name: "exs-abc123" })).toHaveAttribute(
      "href",
      "/blueprints/exs-abc123",
    );
  });

  it("puts the failure reason first, with bex's troubleshooting docs", () => {
    renderNotices(
      deploy({
        status: "update_failed",
        failureReason:
          "the deploy did not become healthy within the health-gate window; check the service logs",
        failureReasonTemplate: {
          code: "health_timed_out",
          params: {},
          parts: [],
          remainder: "",
        },
        clearCache: true,
      }),
    );
    expect(notices()).toEqual([
      "the deploy did not become healthy within the health-gate window; check the service logs" +
        "Read our docs for common ways to troubleshoot your deploy.",
      "Build cache cleared",
    ]);
    expect(screen.getByRole("link", { name: "docs" })).toHaveAttribute(
      "href",
      TROUBLESHOOT_DOCS_URL,
    );
  });

  it("says a past deploy cannot be rolled back to once its build is gone", () => {
    renderNotices(deploy({ status: "deactivated", buildAvailable: false }));
    expect(notices()).toEqual([
      "Rollback is not available because this build no longer exists.",
    ]);
  });

  it("links a queued deploy to the deploy in progress, and the policy only under Wait", () => {
    queryData.Deploys = {
      deploys: [
        { id: "dep-1", status: "queued" },
        { id: "dep-ahead", status: "build_in_progress" },
      ],
    };
    queryData.WorkspaceOverlappingDeployPolicy = {
      workspace: { id: "tea-1", overlappingDeployPolicy: "wait" },
    };
    const { unmount } = renderNotices(deploy({ status: "queued" }));
    expect(notices()).toEqual([
      "Another deploy is in progress" +
        "To avoid queuing deploys, adjust your overlapping deploy policy.",
    ]);
    expect(
      screen.getByRole("link", { name: "Another deploy" }),
    ).toHaveAttribute("href", "/services/srv-1/deploys/dep-ahead");
    expect(
      screen.getByRole("link", { name: "overlapping deploy policy." }),
    ).toHaveAttribute("href", "/workspace/settings#overlapping-deploy-policy");
    expect(queryOptions.WorkspaceOverlappingDeployPolicy?.variables).toEqual({
      id: "tea-1",
    });
    unmount();

    queryData.WorkspaceOverlappingDeployPolicy = {
      workspace: { id: "tea-1", overlappingDeployPolicy: "override" },
    };
    renderNotices(deploy({ status: "queued" }));
    expect(notices()).toEqual(["Another deploy is in progress"]);
  });

  it("reads neither the deploys nor the policy for a deploy that is not waiting", () => {
    renderNotices(deploy({ clearCache: true }));
    expect(queryOptions.Deploys?.skip).toBe(true);
    expect(queryOptions.WorkspaceOverlappingDeployPolicy?.skip).toBe(true);
  });

  it("keeps a canceled deploy's own reason, with the policy under Override for a supersede", () => {
    queryData.WorkspaceOverlappingDeployPolicy = {
      workspace: { id: "tea-1", overlappingDeployPolicy: "override" },
    };
    renderNotices(
      deploy({
        status: "canceled",
        cancelReason: "Superseded by dep-d4newer",
        cancelReasonTemplate: {
          code: "superseded_by",
          params: { deploy: "dep-d4newer" },
          parts: [],
          remainder: "",
        },
      }),
    );
    expect(notices()).toEqual([
      "Superseded by dep-d4newer" +
        "To avoid canceling active deploys when new ones are triggered, adjust your overlapping deploy policy.",
    ]);
  });

  // w4/591: a zh owner read "Superseded by dep-…" in English.
  it("words a supersede cancel in zh, keeping the deploy id", async () => {
    await i18n.changeLanguage("zh");
    try {
      renderNotices(
        deploy({
          status: "canceled",
          cancelReason: "Superseded by dep-d4newer",
          cancelReasonTemplate: {
            code: "superseded_by",
            params: { deploy: "dep-d4newer" },
            parts: [],
            remainder: "",
          },
        }),
      );
      expect(screen.getByRole("heading", { name: "通知" })).toBeInTheDocument();
      expect(screen.getByText("已被 dep-d4newer 取代")).toBeInTheDocument();
      expect(
        screen.queryByText("Superseded by dep-d4newer"),
      ).not.toBeInTheDocument();
    } finally {
      await i18n.changeLanguage("en");
    }
  });

  // w4/m112: a rollout gated on a failing health check used to show a bare
  // "In Progress" for the full 900s budget.
  it("names the health check a rollout waits on, and what the operator found", () => {
    const stall =
      "the container is running but its readiness health check has not " +
      "succeeded, so the rollout is waiting: GET /qa-bogus-health on port 3000.";
    renderNotices(
      deploy({ status: "update_in_progress", stallReason: stall }),
      service({ healthCheckPath: "/qa-bogus-health", port: 3000 }),
    );
    expect(notices()).toEqual([
      "Waiting for internal health check to return a successful response code at: 0.0.0.0:3000/qa-bogus-health",
      stall,
    ]);
    expect(screen.getByRole("link", { name: "health check" })).toHaveAttribute(
      "href",
      HEALTH_CHECK_DOCS_URL,
    );
  });

  it("says a deploy timed out waiting for its health check", () => {
    renderNotices(
      deploy({ status: "update_failed", timedOutSeconds: 900 }),
      service({ healthCheckPath: "/healthz", port: 8080 }),
    );
    expect(notices()).toEqual([
      "Timed out after waiting for internal health check to return a successful response code at: 0.0.0.0:8080/healthz",
    ]);
  });

  it("warns a free service that serves HTTP on a reserved port, as Render does", () => {
    renderNotices(
      deploy({ status: "update_in_progress", reservedPort: 18012 }),
      service({ plan: "free" }),
    );
    const warning = screen.getByTestId("deploy-reserved-port-warning");
    expect(warning).toHaveTextContent(
      "It looks like your service might be listening on port 18012, which is a reserved port for free compute plans. " +
        "Please configure your service to listen on a port that is not part of our reserved ports: 18012, 18013, 19099.",
    );
    expect(
      within(warning).getByRole("link", { name: "free compute plans." }),
    ).toHaveAttribute("href", FREE_PLANS_URL);
    expect(screen.queryByRole("region", { name: "Notices" })).toBeNull();
  });

  it("does not warn a paid service, or a deploy that went live", () => {
    const paid = renderNotices(
      deploy({ status: "update_failed", reservedPort: 18012 }),
      service({ plan: "starter" }),
    );
    expect(paid.container).toBeEmptyDOMElement();
    paid.unmount();
    const live = renderNotices(
      deploy({ status: "live", reservedPort: 18012 }),
      service({ plan: "free" }),
    );
    expect(live.container).toBeEmptyDOMElement();
  });
});
