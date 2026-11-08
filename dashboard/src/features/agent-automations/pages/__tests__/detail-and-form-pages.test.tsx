import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { AutomationDetailPage } from "@/features/agent-automations/pages/automation-detail-page";
import {
  AutomationCreatePage,
  AutomationEditPage,
} from "@/features/agent-automations/pages/automation-form-page";
import { AutomationDetailSkeleton } from "@/features/agent-automations/components/automation-detail-skeleton";
import { AutomationFormSkeleton } from "@/features/agent-automations/components/automation-form-skeleton";
import { automation } from "@/features/agent-automations/components/__tests__/fixtures";
import {
  automationActions,
  NO_ACTIONS,
} from "@/features/agent-automations/lib/display";
import { ROUTE_SKELETON_MANIFEST } from "@/common/lib/route-skeleton-manifest";

const state = vi.hoisted(() => ({
  automation: null as unknown,
  loading: false,
  creator: true,
  admin: false,
  canCreate: true,
  resolving: false,
}));

vi.mock("@tanstack/react-router", async (importOriginal) => {
  const actual =
    await importOriginal<typeof import("@tanstack/react-router")>();
  return {
    ...actual,
    useNavigate: () => vi.fn(),
    Link: ({ children, to }: { children: ReactNode; to: string }) => (
      <a href={to}>{children}</a>
    ),
  };
});
vi.mock("@/common/components/dashboard-layout", () => ({
  DashboardLayout: ({ children }: { children: ReactNode }) => children,
}));
vi.mock("@/common/hooks/use-workspace-subject-label", () => ({
  useWorkspaceSubjectLabel: (s: string) => `label:${s}`,
}));
vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({ currentWorkspaceId: "tea-1" }),
}));
vi.mock("@/features/services/hooks/use-repos", () => ({
  useRepos: () => ({
    repos: [],
    loading: false,
    error: undefined,
    refetch: vi.fn(),
  }),
}));
vi.mock("@/features/agent-sessions/components/composer-secrets", () => ({
  ComposerSecrets: () => <button type="button">secrets</button>,
}));
vi.mock("@/features/agent-sessions/components/composer-skills", () => ({
  ComposerSkills: () => <button type="button">skills</button>,
  ComposerSkillChips: () => null,
}));
vi.mock("@/features/agent-automations/hooks/use-agent-automation", () => ({
  useAgentAutomation: () => ({
    automation: state.automation,
    loading: state.loading,
    error: undefined,
    refetch: vi.fn(),
  }),
  useAutomationMutations: () => ({
    create: vi.fn(),
    update: vi.fn(),
    busy: false,
  }),
}));
vi.mock("@/features/agent-automations/hooks/use-agent-automation-runs", () => ({
  useAutomationRuns: () => ({
    runs: [],
    loading: false,
    error: undefined,
    refetch: vi.fn(),
    loadMore: vi.fn(),
    loadingMore: false,
    hasMore: false,
  }),
}));
vi.mock("@/features/agent-automations/hooks/use-automation-operations", () => ({
  useAutomationOperations: () => ({
    toggle: vi.fn(),
    runNow: vi.fn(),
    takeOver: vi.fn(),
    remove: vi.fn(),
    busy: false,
  }),
}));
vi.mock("@/features/agent-automations/hooks/use-automation-viewer", () => ({
  useAutomationViewer: () => ({
    canCreate: state.canCreate,
    canManage: state.admin,
    resolving: state.resolving,
    actionsFor: () =>
      state.canCreate || state.admin
        ? automationActions({
            canCreate: state.canCreate,
            canManage: state.admin,
            isCreator: state.creator,
          })
        : NO_ACTIONS,
  }),
}));

beforeEach(() => {
  state.automation = automation();
  state.loading = false;
  state.creator = true;
  state.admin = false;
  state.canCreate = true;
  state.resolving = false;
});

const texts = (nodes: Iterable<Element>) =>
  [...nodes].map((n) => n.textContent?.trim());

describe("AutomationDetailPage", () => {
  it("shows the settings, task, run history and the creator's actions", () => {
    render(<AutomationDetailPage automationId="aut-1" />);
    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent(
      "nightly-deps",
    );
    expect(
      screen.getByText("Every day at 09:00 · Europe/Berlin"),
    ).toBeInTheDocument();
    expect(screen.getByText("label:usr-1")).toBeInTheDocument();
    expect(screen.getByText("Update the dependencies")).toBeInTheDocument();
    expect(screen.getByText("No runs yet.")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Edit" })).toBeInTheDocument();
  });

  it("explains a platform pause and offers Resume", () => {
    state.automation = automation({
      pausedReason: "repeated_failures",
      consecutiveFailures: 5,
    });
    render(<AutomationDetailPage automationId="aut-1" />);
    expect(screen.getByTestId("automation-paused-alert")).toHaveTextContent(
      "paused after 5 failed runs in a row",
    );
    expect(screen.getByRole("button", { name: "Resume" })).toBeEnabled();
  });

  it("an admin viewing another member's automation can take it over but not edit it", () => {
    state.creator = false;
    state.admin = true;
    render(<AutomationDetailPage automationId="aut-1" />);
    expect(screen.getByRole("button", { name: "Take over" })).toBeEnabled();
    expect(screen.getByRole("button", { name: "Edit" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Delete" })).toBeEnabled();
  });

  it("shows the pending skeleton while loading", () => {
    state.loading = true;
    state.automation = null;
    const { container } = render(<AutomationDetailPage automationId="aut-1" />);
    expect(
      container.querySelector('[data-route-skeleton="automation-detail"]'),
    ).not.toBeNull();
  });

  it("detail skeleton shares the ready page's labels, buttons and table header, with nothing enabled", () => {
    const ready = render(<AutomationDetailPage automationId="aut-1" />);
    const pick = (root: HTMLElement) => ({
      terms: texts(root.querySelectorAll("dt")),
      buttons: texts(root.querySelectorAll("button, a[href]")).filter((t) =>
        ["Run now", "Pause", "Edit", "Take over", "Delete"].includes(t ?? ""),
      ),
      headers: texts(root.querySelectorAll("th")),
      cards: texts(root.querySelectorAll("[data-slot=card-title]")),
    });
    const readyShape = pick(ready.container);
    ready.unmount();

    const pending = render(<AutomationDetailSkeleton />);
    expect(pick(pending.container)).toEqual(readyShape);
    for (const name of ["Run now", "Pause", "Edit", "Take over", "Delete"]) {
      expect(screen.getByRole("button", { name })).toBeDisabled();
    }
    const entry = ROUTE_SKELETON_MANIFEST["/automations/$automationId"];
    if (entry.kind !== "render") throw new Error("expected a render route");
    for (const region of entry.regions) {
      expect(
        pending.container.querySelector(`[data-skeleton-region="${region}"]`),
      ).not.toBeNull();
    }
  });
});

describe("form pages", () => {
  it("the create page refuses a member who cannot create", () => {
    state.canCreate = false;
    render(<AutomationCreatePage />);
    expect(screen.getByRole("alert")).toHaveTextContent(
      "developer role or higher",
    );
  });

  it("the edit page is for the creator only", () => {
    state.creator = false;
    state.admin = true;
    render(<AutomationEditPage automationId="aut-1" />);
    expect(screen.getByTestId("automation-edit-denied")).toHaveTextContent(
      "Only the member who created this automation can edit it.",
    );
    expect(screen.queryByLabelText("Name")).not.toBeInTheDocument();
  });

  it("the edit page opens the creator's form filled from the stored automation", () => {
    render(<AutomationEditPage automationId="aut-1" />);
    expect(screen.getByLabelText("Name")).toHaveValue("nightly-deps");
    expect(screen.getByLabelText("Task")).toHaveValue(
      "Update the dependencies",
    );
    expect(screen.getByLabelText(/Spend limit/)).toHaveValue("5");
    expect(
      screen.getByRole("button", { name: "Save changes" }),
    ).toBeInTheDocument();
  });

  it.each(["create", "edit"] as const)(
    "the %s skeleton lays out the ready form's sections and labels",
    (mode) => {
      const page =
        mode === "create" ? (
          <AutomationCreatePage />
        ) : (
          <AutomationEditPage automationId="aut-1" />
        );
      const ready = render(page);
      const pick = (root: HTMLElement) => ({
        title: root.querySelector("h1")?.textContent,
        legends: texts(root.querySelectorAll("legend")),
        labels: texts(root.querySelectorAll("label")),
        submit: texts(root.querySelectorAll("button")).filter(
          (t) => t === "Create automation" || t === "Save changes",
        ),
      });
      const readyShape = pick(ready.container);
      const frame = ready.container.firstElementChild?.className;
      ready.unmount();

      const pending = render(<AutomationFormSkeleton mode={mode} />);
      expect(pick(pending.container)).toEqual(readyShape);
      expect(pending.container.firstElementChild?.className).toBe(frame);
      const entry =
        ROUTE_SKELETON_MANIFEST[
          mode === "create"
            ? "/automations/new"
            : "/automations/$automationId/edit"
        ];
      if (entry.kind !== "render") throw new Error("expected a render route");
      for (const region of entry.regions) {
        expect(
          pending.container.querySelector(`[data-skeleton-region="${region}"]`),
        ).not.toBeNull();
      }
      expect(
        screen.getByRole("button", {
          name: pick(pending.container).submit[0] ?? "",
        }),
      ).toBeDisabled();
    },
  );
});
