import { useState } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, render, screen, waitFor } from "@testing-library/react";
import { CombinedGraphQLErrors } from "@apollo/client/errors";
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
// The real useEnvGroupEnvironmentPatch runs against this mutation, so these
// tests see the exact variables a save would put on the wire (w4/m161).
const mutate = vi.fn();
const toastError = vi.fn();

vi.mock("@apollo/client/react", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@apollo/client/react")>()),
  useMutation: () => [mutate, { loading: false }],
}));

vi.mock("sonner", () => ({
  toast: {
    success: vi.fn(),
    warning: vi.fn(),
    error: (...args: unknown[]) => toastError(...args),
  },
}));

vi.mock(
  "@/features/env-groups/hooks/use-env-groups",
  async (importOriginal) => {
    const actual =
      await importOriginal<
        typeof import("@/features/env-groups/hooks/use-env-groups")
      >();
    return {
      ...actual,
      useRevealEnvGroupVar: () => revealEnv,
      useRevealEnvGroupSecretFile: () => revealFile,
    };
  },
);

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

// Stands in for the route: `setGroup` is how a test delivers what an
// EnvGroup poll would — the same group, re-read with a newer revision.
let setGroup: (group: EnvGroupView) => void = () => {};
const refetch = vi.fn();

function Harness({ initial }: { initial: EnvGroupView }) {
  const [group, set] = useState(initial);
  setGroup = set;
  return (
    <EnvGroupEditors
      group={group}
      loading={false}
      error={undefined}
      refetch={refetch}
    />
  );
}

function renderEditors(initial: EnvGroupView = GROUP) {
  const root = createRootRoute();
  const route = createRoute({
    getParentRoute: () => root,
    path: "/",
    component: () => <Harness initial={initial} />,
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
  mutate.mockReset();
  toastError.mockReset();
  refetch.mockReset().mockResolvedValue(null);
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

// w4/139's pass-5 retest reproduced the same coupled rows on an environment
// GROUP, which is the editor's second production caller. The fix lives in the
// shared editor, so this asserts the group wrapper reaches it — including that
// the group's persisted row (FOO) is untouched, as the live retest observed.
describe("EnvGroupEditors restored-draft row identity (w4/139)", () => {
  beforeEach(() => {
    sessionStorage.clear();
  });

  it("keeps a restored variable independent of one added afterwards", async () => {
    const user = userEvent.setup();
    const first = renderEditors();

    await user.click(await screen.findByRole("button", { name: "Edit" }));
    await user.click(screen.getByRole("button", { name: "Add variable" }));
    await user.click(
      await screen.findByRole("menuitem", { name: "Add variable" }),
    );
    const initialKeys = screen.getAllByRole("textbox", { name: "Key" });
    await user.type(initialKeys[initialKeys.length - 1]!, "QA_FIRST");

    first.unmount();
    renderEditors();
    expect(await screen.findByText("Restored unsaved changes")).toBeVisible();

    await user.click(screen.getByRole("button", { name: "Add variable" }));
    await user.click(
      await screen.findByRole("menuitem", { name: "Add variable" }),
    );
    const keys = screen.getAllByRole("textbox", { name: "Key" });
    await user.type(keys[keys.length - 1]!, "QA_SECOND");

    const values = keys.map((input) => (input as HTMLInputElement).value);
    expect(values).toContain("QA_FIRST");
    expect(values).toContain("QA_SECOND");
    // The group's saved row stays as it was — the live retest's control.
    expect(
      screen.getByRole("textbox", { name: "Value for FOO" }),
    ).toBeInTheDocument();
  });
});

// w4/m161: two tabs edit one group. B saves first; A's 30-second poll then
// delivers B's revision. A's draft must keep the revision it was opened from,
// so A's save is refused instead of silently overwriting B's value.
describe("EnvGroupEditors draft base revision (w4/m161)", () => {
  const conflict = () =>
    new CombinedGraphQLErrors({
      errors: [
        {
          message:
            "the environment group changed; refresh it before saving again",
          extensions: { code: "ENV_GROUP_REVISION_CONFLICT" },
        },
      ],
    });
  const accepted = (revision: string, failedServiceIds: string[] = []) => ({
    data: {
      patchEnvGroupEnvironment: {
        envVarKeys: ["FOO"],
        secretFileNames: [],
        revision,
        affectedServiceIds: failedServiceIds,
        failedServiceIds,
        rolledOut: failedServiceIds.length === 0,
      },
    },
  });
  const sentRevisions = () =>
    mutate.mock.calls.map(([call]) => call.variables.expectedRevision);
  const STALE_DRAFT =
    "This environment group may have changed since your draft began, so the draft wasn't saved. Copy anything you need, discard the draft, and edit again to start from the latest version.";

  async function saveOnly(user: ReturnType<typeof userEvent.setup>) {
    await user.click(
      screen.getByRole("button", { name: "Environment save options" }),
    );
    await user.click(
      await screen.findByRole("menuitem", { name: "Save only" }),
    );
  }

  function poll(revision: string) {
    act(() => setGroup({ ...GROUP, revision }));
  }

  beforeEach(() => {
    sessionStorage.clear();
  });

  it("saves against the revision Edit captured, not a later poll's, and keeps the refused draft", async () => {
    mutate.mockRejectedValueOnce(conflict());
    const user = userEvent.setup();
    renderEditors({ ...GROUP, revision: "egr1_A" });

    await user.click(await screen.findByRole("button", { name: "Edit" }));
    const value = screen.getByRole("textbox", { name: "Value for FOO" });
    await user.type(value, "tab-a");

    poll("egr1_B"); // tab B saved; tab A's poll reads it
    // The poll neither drops the draft nor steals its focus.
    expect(value).toHaveValue("tab-a");
    expect(value).toHaveFocus();

    await saveOnly(user);
    await waitFor(() => expect(mutate).toHaveBeenCalledTimes(1));
    expect(mutate).toHaveBeenCalledWith({
      variables: {
        id: "eg-1",
        envVars: [{ key: "FOO", value: "tab-a" }],
        secretFiles: [],
        saveMode: "save_only",
        expectedRevision: "egr1_A",
      },
    });
    // Refused honestly: no automatic retry with the polled token, the typed
    // draft is still there to copy, and no refresh pretends it saved.
    expect(toastError).toHaveBeenCalledWith(STALE_DRAFT);
    expect(screen.getByRole("textbox", { name: "Value for FOO" })).toHaveValue(
      "tab-a",
    );
    expect(refetch).not.toHaveBeenCalled();

    // A second attempt still carries the original base — no silent rebase.
    mutate.mockRejectedValueOnce(conflict());
    await saveOnly(user);
    await waitFor(() => expect(mutate).toHaveBeenCalledTimes(2));
    expect(sentRevisions()).toEqual(["egr1_A", "egr1_A"]);

    // Explicit discard, then a fresh edit starts from the latest read.
    await user.click(screen.getByRole("button", { name: "Cancel" }));
    await user.click(screen.getByRole("button", { name: "Edit" }));
    await user.type(
      screen.getByRole("textbox", { name: "Value for FOO" }),
      "fresh",
    );
    mutate.mockResolvedValueOnce(accepted("egr1_C"));
    await saveOnly(user);
    await waitFor(() => expect(mutate).toHaveBeenCalledTimes(3));
    expect(sentRevisions()).toEqual(["egr1_A", "egr1_A", "egr1_B"]);
    expect(await screen.findByRole("button", { name: "Edit" })).toBeEnabled();
    expect(refetch).toHaveBeenCalledTimes(1);
  });

  it("pins the base when Edit opens a still-clean draft", async () => {
    mutate.mockResolvedValueOnce(accepted("egr1_C"));
    const user = userEvent.setup();
    renderEditors({ ...GROUP, revision: "egr1_A" });

    await user.click(await screen.findByRole("button", { name: "Edit" }));
    poll("egr1_B");
    await user.type(
      screen.getByRole("textbox", { name: "Value for FOO" }),
      "late",
    );
    await saveOnly(user);
    await waitFor(() => expect(sentRevisions()).toEqual(["egr1_A"]));
  });

  it("pins the base for a file-only draft", async () => {
    mutate.mockRejectedValueOnce(conflict());
    const user = userEvent.setup();
    renderEditors({
      ...GROUP,
      revision: "egr1_A",
      envVarKeys: [],
      secretFileNames: ["cert.pem"],
    });

    await user.click(await screen.findByRole("button", { name: "Edit" }));
    await user.click(
      screen.getByRole("button", { name: "Delete secret file cert.pem" }),
    );
    poll("egr1_B");
    await saveOnly(user);
    await waitFor(() =>
      expect(mutate).toHaveBeenCalledWith({
        variables: {
          id: "eg-1",
          envVars: [],
          secretFiles: [{ name: "cert.pem", delete: true }],
          saveMode: "save_only",
          expectedRevision: "egr1_A",
        },
      }),
    );
    expect(toastError).toHaveBeenCalledWith(STALE_DRAFT);
  });

  it("retries an incomplete rollout as an empty patch at the committed revision", async () => {
    mutate
      .mockResolvedValueOnce(accepted("egr1_saved", ["web"]))
      .mockResolvedValueOnce(accepted("egr1_retried"));
    const user = userEvent.setup();
    renderEditors({ ...GROUP, revision: "egr1_A" });

    await user.click(await screen.findByRole("button", { name: "Edit" }));
    await user.type(
      screen.getByRole("textbox", { name: "Value for FOO" }),
      "rolled",
    );
    await user.click(screen.getByRole("button", { name: "Save and deploy" }));
    const retry = await screen.findByRole("button", { name: "Retry rollout" });

    poll("egr1_polled"); // a later read must not become the retry's token
    await user.click(retry);
    await waitFor(() => expect(mutate).toHaveBeenCalledTimes(2));
    expect(mutate.mock.calls[1]![0].variables).toEqual({
      id: "eg-1",
      envVars: [],
      secretFiles: [],
      saveMode: "deploy",
      expectedRevision: "egr1_saved",
    });
  });

  it("closes an accepted draft even when the follow-up refresh fails", async () => {
    mutate.mockResolvedValueOnce(accepted("egr1_C"));
    refetch.mockRejectedValueOnce(new Error("network"));
    const user = userEvent.setup();
    renderEditors({ ...GROUP, revision: "egr1_A" });

    await user.click(await screen.findByRole("button", { name: "Edit" }));
    await user.type(
      screen.getByRole("textbox", { name: "Value for FOO" }),
      "kept",
    );
    await saveOnly(user);
    expect(await screen.findByRole("button", { name: "Edit" })).toBeEnabled();
    expect(mutate).toHaveBeenCalledTimes(1);
    expect(toastError).not.toHaveBeenCalled();
  });

  it("keeps Edit unavailable until the group's revision has been read", async () => {
    renderEditors({ ...GROUP, revision: null });
    expect(await screen.findByRole("button", { name: "Edit" })).toBeDisabled();
  });

  it("restores a draft after reauthentication with its original base", async () => {
    mutate.mockRejectedValueOnce(conflict());
    const user = userEvent.setup();
    const first = renderEditors({ ...GROUP, revision: "egr1_A" });
    await user.click(await screen.findByRole("button", { name: "Edit" }));
    await user.type(
      screen.getByRole("textbox", { name: "Value for FOO" }),
      "before-login",
    );

    first.unmount();
    renderEditors({ ...GROUP, revision: "egr1_B" });
    expect(await screen.findByText("Restored unsaved changes")).toBeVisible();
    await saveOnly(user);
    await waitFor(() => expect(sentRevisions()).toEqual(["egr1_A"]));
    expect(screen.getByRole("textbox", { name: "Value for FOO" })).toHaveValue(
      "before-login",
    );
  });

  it("refuses to save a restored draft stored before drafts carried a base", async () => {
    // The pre-w4/m161 stored shape: rows only, no baseRevision.
    sessionStorage.setItem(
      "bex:reauth-draft:env:eg-1",
      JSON.stringify({
        at: Date.now(),
        value: {
          envVars: [
            {
              id: "env:FOO",
              originalKey: "FOO",
              key: "FOO",
              value: "legacy",
              valueChanged: true,
              deleted: false,
            },
          ],
          secretFiles: [],
        },
      }),
    );
    const user = userEvent.setup();
    renderEditors({ ...GROUP, revision: "egr1_B" });
    expect(await screen.findByText("Restored unsaved changes")).toBeVisible();

    await saveOnly(user);
    expect(toastError).toHaveBeenCalledWith(STALE_DRAFT);
    expect(mutate).not.toHaveBeenCalled();
    // Still visible to copy from.
    expect(screen.getByRole("textbox", { name: "Value for FOO" })).toHaveValue(
      "legacy",
    );

    await user.click(
      screen.getByRole("button", { name: "Discard restored changes" }),
    );
    await user.click(screen.getByRole("button", { name: "Edit" }));
    await user.type(
      screen.getByRole("textbox", { name: "Value for FOO" }),
      "fresh",
    );
    mutate.mockResolvedValueOnce(accepted("egr1_C"));
    await saveOnly(user);
    await waitFor(() => expect(sentRevisions()).toEqual(["egr1_B"]));
  });
});
