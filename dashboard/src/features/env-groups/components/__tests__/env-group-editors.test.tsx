import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from "@tanstack/react-router";
import { EnvGroupEditors } from "@/features/env-groups/components/env-group-editors";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { mockCapabilities } from "@/test/mocks/capabilities";
import type { EnvGroupView } from "@/features/env-groups/types";

const revealEnv = vi.fn();
const revealFile = vi.fn();
const save = vi.fn();
const retryRollout = vi.fn();

vi.mock("@/features/env-groups/hooks/use-env-groups", async (importOriginal) => {
  const actual =
    await importOriginal<
      typeof import("@/features/env-groups/hooks/use-env-groups")
    >();
  return {
    ...actual,
    useRevealEnvGroupVar: () => revealEnv,
    useRevealEnvGroupSecretFile: () => revealFile,
    useEnvGroupEnvironmentPatch: () => ({
      save,
      retryRollout,
      saving: false,
    }),
  };
});

const GROUP: EnvGroupView = {
  id: "eg-1",
  name: "shared",
  ownerId: "tea-1",
  environmentId: null,
  createdAt: null,
  updatedAt: null,
  revision: "1",
  availability: null,
  serviceLinks: [],
  envVarKeys: ["FOO"],
  secretFileNames: [],
};

function renderEditors() {
  const root = createRootRoute();
  const route = createRoute({
    getParentRoute: () => root,
    path: "/",
    component: () => (
      <EnvGroupEditors
        group={GROUP}
        loading={false}
        error={undefined}
        refetch={vi.fn()}
      />
    ),
  });
  const router = createRouter({
    routeTree: root.addChildren([route]),
    history: createMemoryHistory({ initialEntries: ["/"] }),
    context: { client: {} as never, session: null },
  });
  return render(<RouterProvider router={router} />);
}

beforeEach(() => {
  vi.mocked(useCapabilities).mockReturnValue(mockCapabilities());
  revealEnv.mockReset().mockResolvedValue("secret");
  revealFile.mockReset().mockResolvedValue("file");
  save.mockReset().mockResolvedValue({});
  retryRollout.mockReset().mockResolvedValue(true);
});

describe("EnvGroupEditors — role-gated writes", () => {
  it("disables Edit for a contributor without can_create", async () => {
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({ role: "CONTRIBUTOR", canCreate: false }),
    );
    renderEditors();
    expect(await screen.findByRole("button", { name: "Edit" })).toBeDisabled();
  });

  it("lets an admin enter the write draft", async () => {
    const user = userEvent.setup();
    renderEditors();
    const edit = await screen.findByRole("button", { name: "Edit" });
    expect(edit).toBeEnabled();
    await user.click(edit);
    expect(screen.getByRole("button", { name: "Add variable" })).toBeEnabled();
    expect(screen.getByRole("button", { name: "Delete FOO" })).toBeEnabled();
  });

  // w2/m95 t002: the group editor shares `EnvDraftItem` with the service
  // Environment page, so the multi-line fix must reach it through that seam —
  // this pins the sharing rather than trusting it.
  it("keeps the line breaks of a pasted multi-line value", async () => {
    const user = userEvent.setup();
    renderEditors();
    await user.click(await screen.findByRole("button", { name: "Edit" }));
    const value = screen.getByRole("textbox", { name: "Value for FOO" });
    expect(value.tagName).toBe("TEXTAREA");

    await user.click(value);
    await user.paste("first\nsecond\nthird");
    expect((value as HTMLTextAreaElement).value).toBe("first\nsecond\nthird");
  });
});
