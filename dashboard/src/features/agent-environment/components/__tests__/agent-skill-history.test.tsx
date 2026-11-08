import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { AgentSkillDialog } from "@/features/agent-environment/components/agent-skill-dialog";
import { AgentSkillHistory } from "@/features/agent-environment/components/agent-skill-history";
import { AgentSkillHistorySkeleton } from "@/features/agent-environment/components/agent-skill-history-skeleton";
import type {
  AgentSkillVersionDetail,
  AgentSkillVersionSummary,
  UseAgentSkillVersionResult,
  UseAgentSkillVersionsResult,
} from "@/features/agent-environment/hooks/use-agent-skill-versions";
import type { AgentSkillDetail } from "@/features/agent-environment/hooks/use-agent-skills";

const hooks = vi.hoisted(() => ({
  list: null as unknown as UseAgentSkillVersionsResult,
  one: null as unknown as (n: number | null) => UseAgentSkillVersionResult,
}));
vi.mock("@/features/agent-environment/hooks/use-agent-skill-versions", () => ({
  useAgentSkillVersions: () => hooks.list,
  useAgentSkillVersion: (_o: string, _s: string, n: number | null) =>
    hooks.one(n),
}));

function summary(version: number, current = false): AgentSkillVersionSummary {
  return {
    __typename: "AgentSkillVersion",
    version,
    shortId: `abc000${version}`,
    message:
      version === 1 ? "Add skills/run/SKILL.md" : "Update skills/run/SKILL.md",
    createdBy: "usr-1",
    createdAt: "2026-11-06T10:00:00Z",
    current,
  };
}

const DETAILS: Record<number, Partial<AgentSkillVersionDetail>> = {
  1: {
    content: "one\n",
    previousVersion: null,
    diff: "@@ -0,0 +1,1 @@\n+one\n",
  },
  2: {
    content: "one\ntwo\n",
    previousVersion: 1,
    diff: "@@ -1,1 +1,2 @@\n one\n+two\n",
  },
  3: { content: "one\ntwo\n", previousVersion: 2, diff: null },
};

function detailFor(n: number): AgentSkillVersionDetail {
  return {
    ...summary(n, n === 3),
    skillId: "skl-1",
    ...DETAILS[n],
  } as AgentSkillVersionDetail;
}

function setList(patch: Partial<UseAgentSkillVersionsResult> = {}) {
  hooks.list = {
    versions: [summary(3, true), summary(2), summary(1)],
    loading: false,
    error: undefined,
    retry: vi.fn(),
    hasMore: false,
    loadingMore: false,
    loadMore: vi.fn(async () => undefined),
    ...patch,
  };
}

beforeEach(() => {
  setList();
  hooks.one = (n) => ({
    version: n == null ? null : detailFor(n),
    loading: false,
    error: undefined,
    retry: vi.fn(),
  });
});

function renderHistory(canLoad = true) {
  const onLoad = vi.fn();
  render(
    <AgentSkillHistory
      ownerId="tea-1"
      skillId="skl-1"
      canLoad={canLoad}
      onLoad={onLoad}
    />,
  );
  return { onLoad, user: userEvent.setup() };
}

describe("AgentSkillHistory", () => {
  it("lists versions newest first with message, author, short id and one Current badge", () => {
    renderHistory();
    const items = within(
      screen.getByTestId("agent-skill-versions"),
    ).getAllByRole("listitem");
    expect(items).toHaveLength(3);
    expect(items[0]).toHaveTextContent("Current");
    expect(items[0]).toHaveTextContent("usr-1");
    expect(items[0]).toHaveTextContent("abc0003");
    expect(items[2]).toHaveTextContent("Add skills/run/SKILL.md");
    expect(screen.getAllByText("Current")).toHaveLength(1);
  });

  it("shows the newest version's changes first and toggles to its content", async () => {
    const { user } = renderHistory();
    expect(
      screen.getByText("This version changed nothing."),
    ).toBeInTheDocument();
    await user.click(screen.getByTestId("version-2"));
    const diff = screen.getByTestId("agent-skill-diff");
    expect(within(diff).getAllByTestId("diff-add")).toHaveLength(1);
    expect(within(diff).getByTestId("diff-add")).toHaveTextContent("two");
    await user.click(screen.getByRole("button", { name: "Content" }));
    expect(screen.queryByTestId("agent-skill-diff")).toBeNull();
    expect(screen.getByTestId("agent-skill-content")).toHaveTextContent(
      "1one2two",
    );
    await user.click(screen.getByRole("button", { name: "Changes" }));
    expect(screen.getByTestId("agent-skill-diff")).toBeInTheDocument();
  });

  it("renders the first version as all additions", async () => {
    const { user } = renderHistory();
    await user.click(screen.getByTestId("version-1"));
    expect(screen.getAllByTestId("diff-add")).toHaveLength(1);
    expect(screen.queryByTestId("diff-remove")).toBeNull();
  });

  it("loads the selected version's content for admins", async () => {
    const { user, onLoad } = renderHistory();
    await user.click(screen.getByTestId("version-2"));
    await user.click(screen.getByRole("button", { name: "Load into editor" }));
    expect(onLoad).toHaveBeenCalledWith("one\ntwo\n", 2);
  });

  it("lets non-admins browse but not load", () => {
    renderHistory(false);
    expect(
      screen.getByRole("button", { name: "Load into editor" }),
    ).toBeDisabled();
    expect(screen.getAllByTestId(/^version-/)).toHaveLength(3);
  });

  it("pages with Load more only while a next cursor exists", async () => {
    setList({ hasMore: true });
    const { user } = renderHistory();
    await user.click(screen.getByRole("button", { name: "Load more" }));
    expect(hooks.list.loadMore).toHaveBeenCalledTimes(1);
  });

  it("hides Load more on the last page", () => {
    renderHistory();
    expect(screen.queryByRole("button", { name: "Load more" })).toBeNull();
  });

  it("shows the skeleton while the first page loads", () => {
    setList({ versions: [], loading: true });
    renderHistory();
    expect(
      screen.getByTestId("agent-skill-history-skeleton"),
    ).toBeInTheDocument();
  });

  it("offers a retry when the list fails", async () => {
    setList({ versions: [], error: new Error("boom") });
    const { user } = renderHistory();
    expect(screen.getByRole("alert")).toHaveTextContent(
      "Couldn't load the version history.",
    );
    await user.click(screen.getByRole("button", { name: "Retry" }));
    expect(hooks.list.retry).toHaveBeenCalled();
  });
});

describe("AgentSkillHistorySkeleton geometry", () => {
  // dashboard/AGENTS.md: the skeleton shares the ready tab's frame, so the list
  // and the version panel keep the same fixed heights at md and at 375 px.
  it("uses the ready frame's regions and heights", () => {
    const { container: skeleton } = render(<AgentSkillHistorySkeleton />);
    const ready = render(
      <AgentSkillHistory
        ownerId="tea-1"
        skillId="skl-1"
        canLoad
        onLoad={vi.fn()}
      />,
    ).container;
    for (const region of ["version-list", "version-panel"]) {
      const a = skeleton.querySelector(`[data-skeleton-region="${region}"]`);
      const b = ready.querySelector(`[data-skeleton-region="${region}"]`);
      expect(a).not.toBeNull();
      expect(a?.className).toBe(b?.className);
    }
    expect(
      skeleton.querySelector('[data-skeleton-region="version-list"]'),
    ).toHaveClass("h-44", "md:h-96");
    expect(
      skeleton.querySelector('[data-skeleton-region="version-panel"]'),
    ).toHaveClass("h-72", "md:h-96");
    expect(
      skeleton.querySelectorAll('[data-skeleton-region="version-list"] .h-16'),
    ).not.toHaveLength(0);
  });
});

describe("AgentSkillDialog version history", () => {
  const skill: AgentSkillDetail = {
    __typename: "AgentSkill",
    id: "skl-1",
    name: "run",
    description: "Run it",
    triggers: ["user", "model"],
    enabled: true,
    version: 3,
    body: "Current body",
    content: "---\nname: run\n---\n\nCurrent body\n",
  };

  function open(kind: "edit" | "view") {
    const onSave = vi.fn(async () => undefined);
    render(
      <AgentSkillDialog
        ownerId="tea-1"
        target={{ kind, skill }}
        onOpenChange={vi.fn()}
        onSave={onSave}
        saving={false}
      />,
    );
    return { onSave, user: userEvent.setup() };
  }

  it("has no history tab when creating", () => {
    render(
      <AgentSkillDialog
        ownerId="tea-1"
        target={{ kind: "create" }}
        onOpenChange={vi.fn()}
        onSave={vi.fn()}
        saving={false}
      />,
    );
    expect(screen.queryByRole("tab", { name: "Version history" })).toBeNull();
  });

  it("loads a version into the editor, marks it dirty, and saves against the current version", async () => {
    const { user, onSave } = open("edit");
    await user.click(screen.getByRole("tab", { name: "Version history" }));
    await user.click(screen.getByTestId("version-2"));
    await user.click(screen.getByRole("button", { name: "Load into editor" }));
    const dialog = screen.getByTestId("agent-skill-dialog");
    expect(screen.getByRole("tab", { name: "Edit" })).toHaveAttribute(
      "data-state",
      "active",
    );
    expect(within(dialog).getByLabelText("SKILL.md")).toHaveValue("one\ntwo\n");
    expect(dialog.querySelector("form")).toHaveAttribute("data-dirty", "true");
    expect(screen.getByTestId("agent-skill-loaded-notice")).toHaveTextContent(
      "version 2",
    );
    await user.click(
      within(dialog).getByRole("button", { name: "Save skill" }),
    );
    expect(onSave).toHaveBeenCalledWith({
      content: "one\ntwo\n",
      ifVersion: 3,
    });
  });

  it("lets a member browse history read-only", async () => {
    const { user } = open("view");
    await user.click(screen.getByRole("tab", { name: "Version history" }));
    expect(
      screen.getByRole("button", { name: "Load into editor" }),
    ).toBeDisabled();
    expect(screen.queryByRole("button", { name: "Save skill" })).toBeNull();
  });
});
