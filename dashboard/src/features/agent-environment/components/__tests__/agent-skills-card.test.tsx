import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { toast } from "sonner";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { mockCapabilities } from "@/test/mocks/capabilities";
import { codedGraphQLError } from "@/test/mocks/apollo";
import { downloadTextFile } from "@/common/lib/download-file";
import { AgentSkillsCard } from "@/features/agent-environment/components/agent-skills-card";
import { AgentSkillsCardSkeleton } from "@/features/agent-environment/components/agent-skills-card-skeleton";
import type {
  AgentSkillDetail,
  AgentSkillSummary,
  UseAgentSkillsResult,
} from "@/features/agent-environment/hooks/use-agent-skills";

const hook = vi.hoisted(() => ({
  current: null as unknown as UseAgentSkillsResult,
}));

vi.mock("@/features/agent-environment/hooks/use-agent-skills", () => ({
  useAgentSkills: () => hook.current,
}));
vi.mock("@/common/lib/download-file", () => ({ downloadTextFile: vi.fn() }));
vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

const RUN_TESTS: AgentSkillSummary = {
  __typename: "AgentSkill",
  id: "skl-1",
  name: "run-tests",
  description: "Run the integration suite before opening a PR",
  triggers: ["user", "model"],
  enabled: true,
  version: 2,
};
const RELEASE: AgentSkillSummary = {
  __typename: "AgentSkill",
  id: "skl-2",
  name: "release",
  description: "Cut a release",
  triggers: ["user"],
  enabled: false,
  version: 5,
};

function detail(skill: AgentSkillSummary): AgentSkillDetail {
  return {
    ...skill,
    body: "Run `make test`.",
    content: `---\nname: ${skill.name}\ndescription: ${skill.description}\n---\n\nRun \`make test\`.\n`,
  };
}

function setup(
  skills: AgentSkillSummary[] = [RUN_TESTS, RELEASE],
  loading = false,
) {
  const read = vi.fn(async (id: string) =>
    detail(skills.find((s) => s.id === id)!),
  );
  const create = vi.fn(async () => undefined);
  const update = vi.fn(async () => undefined);
  const remove = vi.fn(async () => undefined);
  hook.current = {
    skills,
    loading,
    error: undefined,
    refetch: vi.fn(async () => undefined),
    read,
    create,
    update,
    remove,
    saving: false,
  };
  const view = render(<AgentSkillsCard ownerId="tea-1" />);
  return { read, create, update, remove, user: userEvent.setup(), ...view };
}

describe("AgentSkillsCard (ADR099)", () => {
  beforeEach(() => {
    vi.mocked(useCapabilities).mockReturnValue(mockCapabilities());
    vi.mocked(toast.success).mockClear();
    vi.mocked(downloadTextFile).mockClear();
  });

  it("renders the skeleton while loading", () => {
    setup([], true);
    expect(screen.getByTestId("agent-skills-skeleton")).toBeInTheDocument();
  });

  it("lists skills with the only-when-asked badge and their enabled state", () => {
    setup();
    const rows = within(screen.getByTestId("agent-skills")).getAllByRole(
      "listitem",
    );
    expect(rows).toHaveLength(2);
    expect(rows[0]).toHaveTextContent("run-tests");
    expect(rows[0]).not.toHaveTextContent("Only when asked");
    expect(rows[1]).toHaveTextContent("Only when asked");
    expect(
      screen.getByRole("switch", { name: "Enable run-tests" }),
    ).toBeChecked();
    expect(
      screen.getByRole("switch", { name: "Enable release" }),
    ).not.toBeChecked();
  });

  it("creates a skill from fields with the only-when-asked triggers", async () => {
    const { user, create } = setup([]);
    expect(screen.getByText("No agent skills yet.")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "New skill" }));
    const dialog = screen.getByTestId("agent-skill-dialog");
    await user.type(within(dialog).getByLabelText("Name"), "deploy-staging");
    await user.type(
      within(dialog).getByLabelText("Description"),
      "Deploy to staging",
    );
    await user.click(within(dialog).getByRole("switch"));
    await user.type(
      within(dialog).getByLabelText("Instructions"),
      "Run the deploy script.",
    );
    await user.click(
      within(dialog).getByRole("button", { name: "Save skill" }),
    );
    expect(create).toHaveBeenCalledWith({
      name: "deploy-staging",
      description: "Deploy to staging",
      triggers: ["user"],
      body: "Run the deploy script.",
    });
    await waitFor(() =>
      expect(screen.queryByTestId("agent-skill-dialog")).toBeNull(),
    );
    expect(toast.success).toHaveBeenCalledWith("Saved deploy-staging");
  });

  it("refuses an invalid name before saving", async () => {
    const { user, create } = setup([]);
    await user.click(screen.getByRole("button", { name: "New skill" }));
    const dialog = screen.getByTestId("agent-skill-dialog");
    await user.type(within(dialog).getByLabelText("Name"), "Deploy_Staging");
    await user.type(within(dialog).getByLabelText("Description"), "x");
    expect(within(dialog).getByRole("alert")).toHaveTextContent(
      "Use lowercase letters, digits and single hyphens",
    );
    expect(
      within(dialog).getByRole("button", { name: "Save skill" }),
    ).toBeDisabled();
    expect(create).not.toHaveBeenCalled();
  });

  it("edits with the opened version and maps a concurrent change", async () => {
    const { user, read, update } = setup();
    update.mockRejectedValueOnce(codedGraphQLError("AGENT_SKILL_CONFLICT"));
    await user.click(screen.getByRole("button", { name: "Edit run-tests" }));
    expect(read).toHaveBeenCalledWith("skl-1");
    const dialog = await screen.findByTestId("agent-skill-dialog");
    expect(within(dialog).getByLabelText("Instructions")).toHaveValue(
      "Run `make test`.",
    );
    await user.type(within(dialog).getByLabelText("Instructions"), " Twice.");
    await user.click(
      within(dialog).getByRole("button", { name: "Save skill" }),
    );
    expect(update).toHaveBeenCalledWith(
      "skl-1",
      expect.objectContaining({
        body: "Run `make test`. Twice.",
        ifVersion: 2,
      }),
    );
    expect(await within(dialog).findByRole("alert")).toHaveTextContent(
      "This skill changed since you opened it",
    );
    // A refused save keeps the dialog and the user's edit.
    expect(within(dialog).getByLabelText("Instructions")).toHaveValue(
      "Run `make test`. Twice.",
    );
  });

  it("toggles enabled with the listed version and maps a limit refusal", async () => {
    const { user, update } = setup();
    update.mockRejectedValueOnce(
      codedGraphQLError("AGENT_SKILL_LIMIT_EXCEEDED"),
    );
    await user.click(screen.getByRole("switch", { name: "Enable release" }));
    expect(update).toHaveBeenCalledWith("skl-2", {
      enabled: true,
      ifVersion: 5,
    });
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "This reached a limit on agent skills",
    );
  });

  it("imports a SKILL.md into the whole-file editor and saves it as content", async () => {
    const { user, create } = setup([]);
    const file = new File(
      ["---\nname: lint\ndescription: Lint the repo\n---\n\nRun eslint.\n"],
      "SKILL.md",
      { type: "text/markdown" },
    );
    await user.upload(screen.getByTestId("agent-skill-import"), file);
    const dialog = await screen.findByTestId("agent-skill-dialog");
    expect(within(dialog).getByLabelText("SKILL.md")).toHaveValue(
      "---\nname: lint\ndescription: Lint the repo\n---\n\nRun eslint.\n",
    );
    await user.click(
      within(dialog).getByRole("button", { name: "Save skill" }),
    );
    expect(create).toHaveBeenCalledWith({
      content:
        "---\nname: lint\ndescription: Lint the repo\n---\n\nRun eslint.\n",
    });
  });

  it("exports the skill's SKILL.md", async () => {
    const { user } = setup();
    await user.click(
      screen.getByRole("button", { name: "Download run-tests as SKILL.md" }),
    );
    await waitFor(() =>
      expect(downloadTextFile).toHaveBeenCalledWith(
        "run-tests-SKILL.md",
        detail(RUN_TESTS).content,
        "text/markdown;charset=utf-8",
      ),
    );
  });

  it("deletes after confirming", async () => {
    const { user, remove } = setup();
    await user.click(screen.getByRole("button", { name: "Delete release" }));
    await user.click(screen.getByRole("button", { name: "Delete skill" }));
    expect(remove).toHaveBeenCalledWith("skl-2");
    await waitFor(() =>
      expect(toast.success).toHaveBeenCalledWith("Deleted release"),
    );
  });

  it("is read-only for members: view, export, no writes", async () => {
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({ role: "DEVELOPER", canManage: false }),
    );
    const { user, read } = setup();
    expect(screen.getByRole("button", { name: "New skill" })).toBeDisabled();
    expect(
      screen.getByRole("button", { name: "Import SKILL.md" }),
    ).toBeDisabled();
    expect(
      screen.getByRole("switch", { name: "Enable run-tests" }),
    ).toBeDisabled();
    expect(
      screen.queryByRole("button", { name: "Delete run-tests" }),
    ).toBeNull();
    expect(
      screen.getByText("Only workspace admins can add or change agent skills."),
    ).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "View run-tests" }));
    expect(read).toHaveBeenCalledWith("skl-1");
    const dialog = await screen.findByTestId("agent-skill-dialog");
    expect(within(dialog).getByLabelText("Instructions")).toHaveAttribute(
      "readonly",
    );
    expect(
      within(dialog).queryByRole("button", { name: "Save skill" }),
    ).toBeNull();
  });

  it("reports a failed read instead of opening an empty editor", async () => {
    const { user, read } = setup();
    read.mockRejectedValueOnce(new Error("boom"));
    await user.click(screen.getByRole("button", { name: "Edit run-tests" }));
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Couldn't open the skill.",
    );
    expect(screen.queryByTestId("agent-skill-dialog")).toBeNull();
  });
});

describe("AgentSkillsCardSkeleton geometry", () => {
  // dashboard/AGENTS.md: the skeleton previews the ready card — a header, one
  // h-14 skill row (the empty state holds the same height), and the
  // New / Import actions row with its help line, separated by a top border.
  it("matches the ready card's regions and heights", () => {
    const { container: skeleton } = render(<AgentSkillsCardSkeleton />);
    const row = skeleton.querySelector('[data-skeleton-region="skill-row"]');
    expect(row).toHaveClass("h-14", "w-full");
    const actions = skeleton.querySelector(".border-t.pt-4");
    expect(actions?.querySelectorAll(".h-9")).toHaveLength(2);
    hook.current = {
      skills: [],
      loading: false,
      error: undefined,
      refetch: vi.fn(),
      read: vi.fn(),
      create: vi.fn(),
      update: vi.fn(),
      remove: vi.fn(),
      saving: false,
    };
    vi.mocked(useCapabilities).mockReturnValue(mockCapabilities());
    const { container: ready } = render(<AgentSkillsCard ownerId="tea-1" />);
    expect(ready.querySelector("p.h-14")).toHaveTextContent(
      "No agent skills yet.",
    );
    const readyActions = ready.querySelector(".border-t.pt-4");
    expect(readyActions?.querySelectorAll("button")).toHaveLength(2);
  });
});
