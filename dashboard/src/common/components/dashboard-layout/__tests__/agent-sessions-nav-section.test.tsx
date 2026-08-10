import { describe, it, expect, vi, beforeEach } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  RouterProvider,
  createRouter,
  createRootRoute,
  createRoute,
  createMemoryHistory,
} from "@tanstack/react-router";
import { SidebarProvider } from "@/common/components/ui/sidebar.tsx";
import { DashboardSidebar } from "../dashboard-sidebar";
import type {
  AgentSessionPhase,
  AgentSessionView,
} from "@/features/agent-sessions/types";

const sessionsState: { sessions: AgentSessionView[]; loading: boolean } = {
  sessions: [],
  loading: false,
};
/** Counts how many components mount a polling `useAgentSessions`. */
let pollingMounts = 0;
vi.mock("@/features/agent-sessions/hooks/use-agent-sessions", () => ({
  useAgentSessions: (opts?: { poll?: boolean }) => {
    if (opts?.poll !== false) pollingMounts += 1;
    return sessionsState;
  },
}));

function view(over: Partial<AgentSessionView> = {}): AgentSessionView {
  const phase = (over.phase ?? "completed") as AgentSessionPhase;
  return {
    id: "as-1",
    ownerId: "tea-1",
    repo: "acme/widgets",
    branch: "bex-agent/fix",
    agentConfig: {
      agent: "claude",
      model: null,
      modelEndpoint: null,
      task: "refactor the mapper",
      template: null,
    },
    sandboxId: null,
    sshAddress: null,
    phase,
    status: phase,
    headSha: null,
    prUrl: null,
    prNumber: null,
    evidence: null,
    turns: 1,
    deliveryMode: null,
    failureReason: null,
    createdAt: "2026-08-05T00:00:00Z",
    updatedAt: "2026-08-05T00:01:00Z",
    canceledAt: null,
    isTerminal:
      phase === "completed" || phase === "failed" || phase === "canceled",
    isSteerable: phase === "completed" || phase === "failed",
    ...over,
  };
}

/** Renders the real rail at a real route, so route-param-derived state
 *  (the active session) and the contextual branch are genuinely exercised. */
function renderAt(routePath: string, initialEntry = routePath) {
  const rootRoute = createRootRoute();
  const route = createRoute({
    getParentRoute: () => rootRoute,
    path: routePath,
    component: () => (
      <SidebarProvider>
        <DashboardSidebar />
      </SidebarProvider>
    ),
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([route]),
    history: createMemoryHistory({ initialEntries: [initialEntry] }),
    context: { client: {} as never, session: null },
  });
  return render(<RouterProvider router={router} />);
}

beforeEach(() => {
  sessionsState.loading = false;
  sessionsState.sessions = [];
  pollingMounts = 0;
});

// w5/m64: `/agents` used to render a SECOND `<aside>` inside the page body on
// top of DashboardSidebar. The list now lives in the one rail as Devin's
// contextual slot.
describe("AgentSessionsNavSection (w5/m64 — one rail, contextual slot)", () => {
  it("renders the sessions slot alongside the global nav on /agents", async () => {
    sessionsState.sessions = [view()];
    renderAt("/agents");

    // The contextual list AND the global nav, in the same rail.
    expect(await screen.findByText("Recent")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Projects" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Usage" })).toBeInTheDocument();
  });

  it("keeps the slot out of every non-agent route", async () => {
    sessionsState.sessions = [view()];
    for (const path of ["/", "/blueprints", "/usage", "/webhooks"]) {
      const { unmount } = renderAt(path);
      expect(await screen.findByRole("link", { name: "Projects" })).toBeInTheDocument();
      expect(screen.queryByText("Recent")).not.toBeInTheDocument();
      unmount();
    }
  });

  it("marks the open session active from the route param", async () => {
    sessionsState.sessions = [
      view({
        id: "as-a",
        agentConfig: { ...view().agentConfig, task: "wire up metrics" },
      }),
      view({
        id: "as-b",
        agentConfig: { ...view().agentConfig, task: "tighten hero copy" },
      }),
    ];
    renderAt("/agents/$agentSessionId", "/agents/as-b");

    const active = await screen.findByRole("link", { name: /tighten hero copy/ });
    expect(active).toHaveAttribute("data-active", "true");
    expect(
      screen.getByRole("link", { name: /wire up metrics/ }),
    ).not.toHaveAttribute("data-active", "true");
  });

  it("shows human status phrases and links the PR number straight to GitHub", async () => {
    sessionsState.sessions = [
      view({
        id: "as-pr",
        prNumber: 6,
        prUrl: "https://github.com/acme/widgets/pull/6",
        agentConfig: { ...view().agentConfig, task: "ship the PR one" },
      }),
      view({
        id: "as-run",
        phase: "running",
        agentConfig: { ...view().agentConfig, task: "still working one" },
      }),
      view({
        id: "as-fail",
        phase: "failed",
        agentConfig: { ...view().agentConfig, task: "broken one" },
      }),
    ];
    renderAt("/agents");

    expect(await screen.findByText("PR is ready")).toBeInTheDocument();
    expect(screen.getByText("Working…")).toBeInTheDocument();
    expect(screen.getByText("Failed")).toBeInTheDocument();

    // The PR number is a DIRECT external GitHub link, not an internal route,
    // and it is a SIBLING of the row link — never nested inside it.
    const pr = screen.getByRole("link", { name: /#6/ });
    expect(pr).toHaveAttribute("href", "https://github.com/acme/widgets/pull/6");
    expect(pr.closest("a[href='/agents/as-pr']")).toBeNull();
  });

  it("filters the Recent list by title/repo through the search toggle", async () => {
    sessionsState.sessions = [
      view({
        id: "as-a",
        agentConfig: { ...view().agentConfig, task: "wire up metrics" },
      }),
      view({
        id: "as-b",
        agentConfig: { ...view().agentConfig, task: "tighten hero copy" },
      }),
    ];
    renderAt("/agents");
    expect(await screen.findByText("wire up metrics")).toBeInTheDocument();
    expect(screen.getByText("tighten hero copy")).toBeInTheDocument();

    await userEvent.click(
      screen.getByRole("button", { name: /search sessions/i }),
    );
    fireEvent.change(screen.getByRole("textbox"), {
      target: { value: "metrics" },
    });

    expect(screen.getByText("wire up metrics")).toBeInTheDocument();
    expect(screen.queryByText("tighten hero copy")).not.toBeInTheDocument();
  });

  it("exposes a view-all action reaching the standalone list", async () => {
    sessionsState.sessions = [view()];
    renderAt("/agents");
    const viewAll = await screen.findByRole("link", {
      name: /view all sessions/i,
    });
    expect(viewAll.getAttribute("href")).toContain("view=list");
  });

  it("degrades the list without taking the global nav down with it", async () => {
    sessionsState.sessions = [];
    renderAt("/agents");

    // Empty/failed list state renders, and the nav above it still works.
    expect(await screen.findByText("No sessions yet")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Projects" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Settings" })).toBeInTheDocument();
  });

  it("owns exactly one polling useAgentSessions on the rail", async () => {
    sessionsState.sessions = [view()];
    renderAt("/agents");
    await screen.findByText("Recent");
    expect(pollingMounts).toBe(1);
  });
});
