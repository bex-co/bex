import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { mockCapabilities } from "@/test/mocks/capabilities";
import { codedGraphQLError } from "@/test/mocks/apollo";
import { WorkspaceIPRulesCard } from "@/features/workspaces/components/workspace-ip-rules-card";
import { WorkspaceIPRulesCardSkeleton } from "@/features/workspaces/components/workspace-ip-rules-card-skeleton";
import type { WorkspaceView } from "@/features/workspaces/types";
import type { IPAllowListEntryDraft } from "@/common/lib/ip-allow-list";

// The real save hook runs; only Apollo's mutate is faked, so the variables the
// card sends are asserted at the GraphQL boundary. The card's own read is
// pinned in the hooks' test; here it answers per workspace id.
const mutate = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useMutation: () => [mutate],
}));
vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));
const read = vi.hoisted(() => ({
  byId: {} as Record<string, IPAllowListEntryDraft[] | null>,
  loading: false,
}));
vi.mock("@/features/workspaces/hooks/use-workspace-ip-allow-list", () => ({
  useWorkspaceIPAllowList: (id: string) => ({
    entries: read.loading ? null : (read.byId[id] ?? null),
    loading: read.loading,
    error: undefined,
  }),
}));

const SEEDED: IPAllowListEntryDraft[] = [
  { cidrBlock: "0.0.0.0/0", description: "Allow all (default)" },
  { cidrBlock: "::/0", description: "Allow all (default)" },
];
const OFFICE: IPAllowListEntryDraft[] = [
  { cidrBlock: "203.0.113.0/24", description: "office" },
];

/** A workspace whose rules read answers `ipAllowList` (null: the read failed). */
function workspace(
  plan: string,
  ipAllowList: IPAllowListEntryDraft[] | null = SEEDED,
  id = "tea-1",
): WorkspaceView {
  read.byId[id] = ipAllowList;
  return {
    id,
    name: "acme",
    slug: "acme",
    plan,
    role: "admin",
    createdAt: null,
    requireTwoFactor: false,
  };
}

function setup(ws: WorkspaceView) {
  const onChangePlan = vi.fn();
  const view = render(
    <WorkspaceIPRulesCard workspace={ws} onChangePlan={onChangePlan} />,
  );
  return { onChangePlan, user: userEvent.setup(), ...view };
}

function sent() {
  return mutate.mock.calls.map(([options]) => options.variables);
}

beforeEach(() => {
  read.byId = {};
  read.loading = false;
  mutate.mockReset();
  mutate.mockResolvedValue({ data: { setWorkspaceIPAllowList: null } });
  vi.mocked(useCapabilities).mockReturnValue(mockCapabilities());
});

describe("WorkspaceIPRulesCard (w9/m388)", () => {
  it("lets an Admin on Scale add a rule and save the full list", async () => {
    const { user } = setup(workspace("scale"));

    expect(
      screen.getByRole("heading", { name: "Inbound IP Rules" }),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/on top of environment and service rules/),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("textbox", { name: "CIDR block for rule 1" }),
    ).toHaveValue("0.0.0.0/0");
    expect(screen.getByRole("button", { name: "Save rules" })).toBeDisabled();

    await user.type(
      screen.getByRole("textbox", { name: "New CIDR block" }),
      "198.51.100.0/24",
    );
    await user.type(
      screen.getByRole("textbox", { name: "New rule description" }),
      "vpn",
    );
    await user.click(screen.getByRole("button", { name: "Add source" }));
    await user.click(screen.getByRole("button", { name: "Save rules" }));

    expect(sent()).toEqual([
      {
        id: "tea-1",
        ipAllowList: [
          { cidrBlock: "0.0.0.0/0", description: "Allow all (default)" },
          { cidrBlock: "::/0", description: "Allow all (default)" },
          { cidrBlock: "198.51.100.0/24", description: "vpn" },
        ],
      },
    ]);
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("discards an unsaved draft on Cancel", async () => {
    const { user } = setup(workspace("enterprise", OFFICE));
    const cancel = screen.getByRole("button", { name: "Cancel" });
    expect(cancel).toBeDisabled();

    await user.click(
      screen.getByRole("button", { name: "Remove 203.0.113.0/24" }),
    );
    expect(
      screen.queryByRole("textbox", { name: "CIDR block for rule 1" }),
    ).not.toBeInTheDocument();
    await user.click(cancel);

    expect(
      screen.getByRole("textbox", { name: "CIDR block for rule 1" }),
    ).toHaveValue("203.0.113.0/24");
    expect(screen.getByRole("button", { name: "Save rules" })).toBeDisabled();
    expect(mutate).not.toHaveBeenCalled();
  });

  // An empty list is deny-all, as on Render: the warning shows on the draft
  // before Save, and the empty list is still what Save sends.
  it("warns that an empty list blocks everything before it is saved", async () => {
    const { user } = setup(workspace("scale"));
    expect(screen.queryByText(/No rules: all inbound traffic/)).toBeNull();

    await user.click(screen.getByRole("button", { name: "Remove 0.0.0.0/0" }));
    await user.click(screen.getByRole("button", { name: "Remove ::/0" }));

    const warning = screen.getByText(
      "No rules: all inbound traffic to this workspace's services and datastores is blocked. Add 0.0.0.0/0 and ::/0 to allow all sources.",
    );
    expect(warning).toHaveClass("text-destructive");
    await user.click(screen.getByRole("button", { name: "Save rules" }));
    expect(sent()).toEqual([{ id: "tea-1", ipAllowList: [] }]);
  });

  it("shows a saved deny-all list with the same warning", () => {
    setup(workspace("scale", []));
    expect(screen.getByText(/No rules: all inbound traffic/)).toHaveClass(
      "text-destructive",
    );
  });

  it("shows a non-Admin the rules read-only", () => {
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({ role: "DEVELOPER", canManage: false }),
    );
    setup(workspace("scale", OFFICE));

    expect(
      screen.getByText("Only workspace Admins can change these rules."),
    ).toBeInTheDocument();
    expect(screen.getByText("203.0.113.0/24")).toBeInTheDocument();
    expect(screen.queryByRole("textbox")).not.toBeInTheDocument();
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
  });

  it("stays read-only while the grant is still loading (fail-closed)", () => {
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({ canManage: false, loading: true, loaded: false }),
    );
    setup(workspace("scale"));
    expect(screen.queryByRole("textbox")).not.toBeInTheDocument();
    expect(screen.getByText("Open to all source IPs")).toBeInTheDocument();
  });

  it("shows a Pro Admin the plan prompt instead of the editor", async () => {
    const { user, onChangePlan } = setup(workspace("pro"));

    expect(
      screen.getByText(/require the Scale plan or higher/),
    ).toBeInTheDocument();
    expect(screen.getByText("Open to all source IPs")).toBeInTheDocument();
    expect(screen.queryByRole("textbox")).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Add source" }),
    ).not.toBeInTheDocument();
    // Nothing is restricted, so there is nothing to reset either.
    expect(
      screen.queryByRole("button", { name: "Reset to allow all" }),
    ).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Change plan" }));
    expect(onChangePlan).toHaveBeenCalledTimes(1);
  });

  // A workspace downgraded below Scale keeps its stored rules enforced and can
  // always lift them: the reset writes Render's seeded pair, which bex-api
  // accepts on every plan.
  it("lets a downgraded workspace reset its stored rules to allow all", async () => {
    const { user } = setup(workspace("hobby", OFFICE));
    expect(screen.getByText("203.0.113.0/24")).toBeInTheDocument();

    await user.click(
      screen.getByRole("button", { name: "Reset to allow all" }),
    );

    expect(sent()).toEqual([
      {
        id: "tea-1",
        ipAllowList: [
          { cidrBlock: "0.0.0.0/0", description: "Allow all (default)" },
          { cidrBlock: "::/0", description: "Allow all (default)" },
        ],
      },
    ]);
  });

  it("offers a non-Admin below Scale neither the reset nor the upgrade", () => {
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({ role: "DEVELOPER", canManage: false }),
    );
    setup(workspace("pro", OFFICE));
    expect(
      screen.getByText(/require the Scale plan or higher/),
    ).toHaveTextContent("Only workspace Admins can change these rules.");
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
  });

  it("explains a PLAN_FEATURE_REQUIRED refusal with the plan, inline", async () => {
    mutate.mockRejectedValue(
      codedGraphQLError("PLAN_FEATURE_REQUIRED", {
        plan: "pro",
        feature: "inboundIPRules",
        requiredPlan: "scale",
        field: "ipAllowList",
      }),
    );
    // The plan read as Scale here (a downgrade raced the save).
    const { user, onChangePlan } = setup(workspace("scale"));
    await user.click(screen.getByRole("button", { name: "Remove ::/0" }));
    await user.click(screen.getByRole("button", { name: "Save rules" }));

    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent("Couldn't save inbound IP rules");
    expect(alert).toHaveTextContent(
      "This workspace's plan doesn't include inbound IP rules.",
    );
    expect(alert).not.toHaveTextContent("reworded by the server");
    await user.click(
      within(alert).getByRole("button", { name: "Change plan" }),
    );
    expect(onChangePlan).toHaveBeenCalledTimes(1);
  });

  it("explains a forbidden refusal as Admin-only, and relays any other reason", async () => {
    mutate.mockRejectedValueOnce(codedGraphQLError("FORBIDDEN"));
    const { user } = setup(workspace("scale"));
    await user.click(screen.getByRole("button", { name: "Remove ::/0" }));
    await user.click(screen.getByRole("button", { name: "Save rules" }));
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Only workspace Admins can change these rules.",
    );

    mutate.mockRejectedValueOnce(
      codedGraphQLError("BAD_REQUEST", {}, "bad request: rule limit reached"),
    );
    await user.click(screen.getByRole("button", { name: "Save rules" }));
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Rule limit reached",
    );
  });

  it("never carries a draft over to another workspace with the same rules", async () => {
    const { user, rerender, onChangePlan } = setup(workspace("scale"));
    await user.click(screen.getByRole("button", { name: "Remove ::/0" }));
    expect(
      screen.queryByRole("textbox", { name: "CIDR block for rule 2" }),
    ).not.toBeInTheDocument();

    rerender(
      <WorkspaceIPRulesCard
        workspace={workspace("scale", SEEDED, "tea-2")}
        onChangePlan={onChangePlan}
      />,
    );
    expect(
      screen.getByRole("textbox", { name: "CIDR block for rule 2" }),
    ).toHaveValue("::/0");
  });

  // A failed read is not "no rules": an empty list is deny-all.
  it("says the rules could not be read instead of claiming none", () => {
    setup(workspace("scale", null));
    expect(
      screen.getByText("Couldn't load this workspace's inbound IP rules."),
    ).toBeInTheDocument();
    expect(screen.queryByText(/No rules/)).toBeNull();
    expect(screen.queryByRole("textbox")).not.toBeInTheDocument();
  });
});

// While the card's own read is in flight the plan (from the workspace list)
// and the grant are known, so it previews the variant it is about to render.
describe("WorkspaceIPRulesCard loading (w9/m388)", () => {
  it.each([
    ["an Admin on Scale", "scale", {}, "editor"],
    ["a member on Scale", "scale", { canManage: false }, "read-only"],
    [
      "an Admin while the grant loads",
      "enterprise",
      { canManage: false, loading: true, loaded: false },
      "read-only",
    ],
    ["an Admin on Pro", "pro", {}, "read-only"],
    ["a member on Hobby", "hobby", { canManage: false }, "read-only"],
  ] as const)(
    "shows %s the matching skeleton",
    (_who, plan, capabilities, variant) => {
      vi.mocked(useCapabilities).mockReturnValue(
        mockCapabilities(capabilities),
      );
      read.loading = true;
      setup(workspace(plan));
      expect(screen.getByTestId("workspace-ip-rules-skeleton")).toHaveAttribute(
        "data-variant",
        variant,
      );
      expect(
        screen.queryByTestId("workspace-ip-rules-card"),
      ).not.toBeInTheDocument();
    },
  );
});

// dashboard/AGENTS.md: the skeleton previews the ready card class for class,
// for Render's seeded allow-all pair, in each variant the card can render.
describe("WorkspaceIPRulesCardSkeleton geometry", () => {
  function pair(plan: string, canManage: boolean) {
    vi.mocked(useCapabilities).mockReturnValue(mockCapabilities({ canManage }));
    const ws = workspace(plan);
    const { container: skeleton } = render(
      <WorkspaceIPRulesCardSkeleton
        belowScale={plan === "hobby" || plan === "pro"}
        canManage={canManage}
      />,
    );
    const { container: ready } = render(
      <WorkspaceIPRulesCard workspace={ws} onChangePlan={vi.fn()} />,
    );
    const content = (root: HTMLElement) =>
      root.querySelector("[data-slot=card-content]")!;
    expect(skeleton.querySelector("[data-slot=card-title]")?.textContent).toBe(
      ready.querySelector("[data-slot=card-title]")?.textContent,
    );
    expect(
      skeleton.querySelector("[data-slot=card-description]")?.textContent,
    ).toBe(ready.querySelector("[data-slot=card-description]")?.textContent);
    expect(content(skeleton).className).toBe(content(ready).className);
    expect(content(skeleton).children).toHaveLength(
      content(ready).children.length,
    );
    const preview = content(skeleton).firstElementChild!;
    const editor = content(ready).firstElementChild!;
    expect(preview.className).toBe(editor.className);
    expect(preview.children).toHaveLength(editor.children.length);
    return { preview, editor };
  }

  it("matches the Scale Admin editor: hint, two rule rows, add row", () => {
    const { preview, editor } = pair("scale", true);
    expect(preview.children[0]).toHaveClass("text-xs");
    expect(editor.children[0]).toHaveClass("text-xs");
    expect(preview.children[0].textContent).toBe(
      editor.children[0].textContent,
    );

    const readyRows = editor.children[1].children;
    const skeletonRows = preview.querySelector(
      '[data-skeleton-region="ip-rule-rows"]',
    )!.children;
    expect(skeletonRows).toHaveLength(readyRows.length);
    for (let i = 0; i < readyRows.length; i++) {
      expect(skeletonRows[i].className).toBe(readyRows[i].className);
      expect(skeletonRows[i].children).toHaveLength(
        readyRows[i].children.length,
      );
    }

    const readyActions = editor.children[2];
    const skeletonActions = preview.querySelector(
      '[data-skeleton-region="ip-rule-actions"]',
    )!;
    expect(skeletonActions.className).toBe(readyActions.className);
    expect(readyActions.querySelectorAll("input")).toHaveLength(2);
    expect(readyActions.querySelectorAll("button")).toHaveLength(3);
    expect(skeletonActions.children).toHaveLength(readyActions.children.length);
  });

  it.each([
    ["a member on Scale", "scale", false],
    ["an Admin below Scale", "pro", true],
    ["a member below Scale", "hobby", false],
  ] as const)(
    "matches the read-only view for %s: notice, then the open line",
    (_who, plan, canManage) => {
      const { preview, editor } = pair(plan, canManage);
      // The notice wraps the same text (the Change plan link included).
      expect(preview.children[0].tagName).toBe(editor.children[0].tagName);
      expect(preview.children[0]).toHaveClass("text-xs");
      expect(editor.children[0]).toHaveClass("text-xs");
      expect(preview.children[0].textContent).toBe(
        editor.children[0].textContent,
      );
      // The "open to all" line: an inline span of the same size and text.
      expect(preview.children[1].tagName).toBe(editor.children[1].tagName);
      expect(preview.children[1]).toHaveClass("text-sm");
      expect(editor.children[1]).toHaveClass("text-sm");
      expect(preview.children[1].textContent).toBe(
        editor.children[1].textContent,
      );
      expect(editor.querySelector("input")).toBeNull();
    },
  );
});
