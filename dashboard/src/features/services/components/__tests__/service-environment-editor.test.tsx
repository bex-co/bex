import { beforeEach, describe, expect, it, vi } from "vitest";
import { useSyncExternalStore } from "react";
import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { mockCapabilities } from "@/test/mocks/capabilities";
import { codedGraphQLError } from "@/test/mocks/apollo";
import {
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from "@tanstack/react-router";

const revealEnv = vi.fn();
const revealFile = vi.fn();
const refetchEnv = vi.fn();
const refetchFiles = vi.fn();
const save = vi.fn();
const trigger = vi.fn();
const toastSuccess = vi.fn();
const toastError = vi.fn();
const toastWarning = vi.fn();
// Mutable so individual tests can render the secret-files empty state.
let fileNames: Array<{ id: string; name: string }> = [];
// Mutable so individual tests can flag a key as manifest-managed (w4/m120) or
// hold the keys read open while the flag has not arrived.
let envKeys: Array<{ id: string; key: string; managedBy?: string | null }> = [];
let envKeysLoading = false;

vi.mock("sonner", () => ({
  toast: {
    success: (...a: unknown[]) => toastSuccess(...a),
    error: (...a: unknown[]) => toastError(...a),
    warning: (...a: unknown[]) => toastWarning(...a),
  },
}));

vi.mock("@/features/services/hooks/use-env-vars", () => ({
  useEnvVarKeys: () => ({
    keys: envKeys,
    loading: envKeysLoading,
    error: undefined,
    refetch: refetchEnv,
  }),
  useRevealEnvVar: () => revealEnv,
}));

vi.mock("@/features/services/hooks/use-secret-files", () => ({
  useSecretFileNames: () => ({
    names: fileNames,
    loading: false,
    error: undefined,
    refetch: refetchFiles,
  }),
  useRevealSecretFile: () => revealFile,
}));

vi.mock("@/features/services/hooks/use-environment-draft-save", () => ({
  useEnvironmentDraftSave: () => ({ save, saving: false }),
}));

vi.mock("@/features/services/hooks/use-trigger-deploy", () => ({
  useTriggerDeploy: () => ({ trigger, deploying: false }),
}));

import { ServiceEnvironmentEditor } from "../service-environment-editor";

function renderEditor() {
  const root = createRootRoute();
  const route = createRoute({
    getParentRoute: () => root,
    path: "/",
    component: () => <ServiceEnvironmentEditor serviceId="web" />,
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
  fileNames = [{ id: "token.txt", name: "token.txt" }];
  envKeys = [
    { id: "ALPHA", key: "ALPHA", managedBy: "" },
    { id: "BETA", key: "BETA", managedBy: "" },
  ];
  envKeysLoading = false;
  toastSuccess.mockReset();
  toastError.mockReset();
  toastWarning.mockReset();
  revealEnv
    .mockReset()
    .mockImplementation(async (key: string) => `${key}-value`);
  revealFile.mockReset().mockResolvedValue("file-value");
  refetchEnv.mockReset().mockResolvedValue([]);
  refetchFiles.mockReset().mockResolvedValue([]);
  save.mockReset().mockResolvedValue({
    envVarKeys: [],
    secretFileNames: [],
    rolledOut: false,
  });
  trigger.mockReset().mockResolvedValue("dep-1");
});

describe("ServiceEnvironmentEditor", () => {
  it("disables every write entry for a contributor while sensitive reads remain available", async () => {
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({ role: "CONTRIBUTOR", canCreate: false }),
    );
    const user = userEvent.setup();
    renderEditor();

    const edit = await screen.findByRole("button", { name: "Edit" });
    expect(edit).toBeDisabled();
    expect(screen.getByRole("button", { name: "Export" })).toBeEnabled();
    expect(screen.getAllByRole("button", { name: "Reveal" })[0]).toBeEnabled();

    await user.hover(edit.parentElement!);
    expect(await screen.findByRole("tooltip")).toHaveTextContent(
      "Your role can’t make this change.",
    );
    await user.click(edit);
    expect(screen.queryByRole("textbox", { name: /Value for / })).toBeNull();
    expect(save).not.toHaveBeenCalled();

    await user.click(screen.getAllByRole("button", { name: "Reveal" })[0]);
    expect(await screen.findByText("ALPHA-value")).toBeInTheDocument();
  });

  it("keeps can_view_sensitive independent from can_create", async () => {
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({ canViewSensitive: false }),
    );
    const user = userEvent.setup();
    renderEditor();

    expect(await screen.findByRole("button", { name: "Edit" })).toBeEnabled();
    expect(screen.getByRole("button", { name: "Export" })).toBeDisabled();
    expect(
      screen
        .getAllByRole("button", { name: "Reveal" })
        .every((button) => button.hasAttribute("disabled")),
    ).toBe(true);

    await user.click(screen.getByRole("button", { name: "Edit" }));
    expect(
      screen.getAllByRole("textbox", { name: /Value for / }),
    ).not.toHaveLength(0);
    expect(revealEnv).not.toHaveBeenCalled();
  });

  it("fail-closes edit until capabilities are definitive", async () => {
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({
        role: null,
        canCreate: false,
        canViewSensitive: false,
        loading: true,
        loaded: false,
      }),
    );
    renderEditor();

    expect(await screen.findByRole("button", { name: "Edit" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Export" })).toBeDisabled();
  });

  it("freezes an already-open draft if create permission is revoked", async () => {
    const user = userEvent.setup();
    renderEditor();
    await user.click(await screen.findByRole("button", { name: "Edit" }));
    await user.type(
      screen.getAllByRole("textbox", { name: /Value for / })[0],
      "pending",
    );

    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({ role: "CONTRIBUTOR", canCreate: false }),
    );
    // This last pre-revocation input event schedules the component render that
    // observes the freshly denied capability; all subsequent handlers are the
    // guarded versions.
    await user.type(
      screen.getAllByRole("textbox", { name: /Value for / })[0],
      "x",
    );

    expect(screen.getByRole("status")).toHaveTextContent(
      "Your role can’t make this change.",
    );
    expect(
      screen
        .getAllByRole("textbox", { name: /Value for / })
        .every((input) => input.hasAttribute("disabled")),
    ).toBe(true);
    expect(screen.getByRole("button", { name: "Add variable" })).toBeDisabled();
    expect(
      screen.getByRole("button", { name: "Add secret file" }),
    ).toBeDisabled();
    expect(
      screen.getByRole("button", { name: "Save and deploy" }),
    ).toBeDisabled();

    await user.click(screen.getByRole("button", { name: "Save and deploy" }));
    expect(save).not.toHaveBeenCalled();
    expect(screen.getByRole("button", { name: "Cancel" })).toBeEnabled();
  });

  it("starts masked and reveals only the requested value", async () => {
    const user = userEvent.setup();
    renderEditor();
    expect(await screen.findAllByText("••••••••••••")).toHaveLength(3);
    expect(
      screen.queryByRole("button", { name: "Delete" }),
    ).not.toBeInTheDocument();

    const alpha = screen.getByText("ALPHA").closest("div");
    await user.click(within(alpha!).getByRole("button", { name: "Reveal" }));
    expect(await screen.findByText("ALPHA-value")).toBeInTheDocument();
    expect(revealEnv).toHaveBeenCalledTimes(1);
    expect(revealEnv).toHaveBeenCalledWith("ALPHA");
    expect(revealFile).not.toHaveBeenCalled();
  });

  // A secret file IS a file — a PEM key, a service-account JSON, a certificate
  // chain — and Reveal is the one screen where a user can check a secret they
  // cannot see anywhere else. Rendering it with the default `white-space:
  // normal` collapsed every newline, so a 20-line key showed as one run-on
  // line (w4/108). The bytes were always right; only the rendering was not.
  // Env values share this component and have supported line breaks since
  // w2/m95, so the assertion covers the class, not one field.
  it("reveals a multi-line value with its line breaks intact", async () => {
    const user = userEvent.setup();
    revealEnv.mockResolvedValue("line-one\nline-two pass109\n");
    renderEditor();
    await screen.findAllByText("••••••••••••");

    const alpha = screen.getByText("ALPHA").closest("div");
    await user.click(within(alpha!).getByRole("button", { name: "Reveal" }));
    const revealed = await screen.findByText(/line-two pass109/);

    // The element must PRESERVE the newlines rather than collapse them.
    expect(revealed.textContent).toBe("line-one\nline-two pass109\n");
    expect(revealed.className).toContain("whitespace-pre-wrap");
    // And a 40-line PEM must scroll inside its row, not push the page.
    expect(revealed.className).toContain("overflow-auto");
  });

  it("keeps edits local and Cancel restores the server snapshot", async () => {
    const user = userEvent.setup();
    renderEditor();
    await user.click(await screen.findByRole("button", { name: "Edit" }));
    const valueInputs = screen.getAllByRole("textbox", { name: /Value for / });
    await user.type(valueInputs[0], "changed");
    expect(save).not.toHaveBeenCalled();
    await user.click(screen.getByRole("button", { name: "Cancel" }));
    expect(save).not.toHaveBeenCalled();
    expect(screen.getByRole("button", { name: "Edit" })).toBeInTheDocument();
    expect(screen.queryByDisplayValue("changed")).not.toBeInTheDocument();
  });

  it("enables saving only while an opaque row has a pending rename", async () => {
    const user = userEvent.setup();
    renderEditor();
    await user.click(await screen.findByRole("button", { name: "Edit" }));
    const submit = screen.getByRole("button", { name: "Save and deploy" });
    const key = screen.getByDisplayValue("ALPHA");
    expect(submit).toBeDisabled();

    await user.type(key, "_RENAMED");
    expect(submit).toBeEnabled();
    await user.clear(key);
    await user.type(key, "ALPHA");
    expect(submit).toBeDisabled();
    expect(revealEnv).not.toHaveBeenCalled();
    expect(save).not.toHaveBeenCalled();
  });

  it("commits one combined patch through Save and deploy", async () => {
    const user = userEvent.setup();
    renderEditor();
    await user.click(await screen.findByRole("button", { name: "Edit" }));
    const valueInputs = screen.getAllByRole("textbox", { name: /Value for / });
    await user.type(valueInputs[0], "replacement");
    await user.click(screen.getByRole("button", { name: "Save and deploy" }));

    await waitFor(() =>
      expect(save).toHaveBeenCalledWith(
        "web",
        {
          envVars: [{ key: "ALPHA", value: "replacement" }],
          secretFiles: [],
        },
        "deploy",
      ),
    );
    expect(save).toHaveBeenCalledTimes(1);
    expect(trigger).not.toHaveBeenCalled();
  });

  // w4/216: a deploying save that changed nothing returns rolledOut: false
  // and starts no release, but the toast said a deployment had started.
  it.each([
    [
      false,
      "Environment saved without a deploy. Use a standard deploy to apply any pending changes.",
    ],
    [true, "Environment saved and deployment started"],
  ])(
    "Save and deploy with rolledOut %s reports what happened",
    async (rolledOut, message) => {
      save.mockResolvedValue({
        envVarKeys: [],
        secretFileNames: [],
        rolledOut,
      });
      const user = userEvent.setup();
      renderEditor();
      await user.click(await screen.findByRole("button", { name: "Edit" }));
      await user.type(
        screen.getAllByRole("textbox", { name: /Value for / })[0],
        "same",
      );
      await user.click(screen.getByRole("button", { name: "Save and deploy" }));
      await waitFor(() => expect(toastSuccess).toHaveBeenCalledWith(message));
      expect(trigger).not.toHaveBeenCalled();
    },
  );

  it("explains deferred application after Save only without starting a deploy", async () => {
    const user = userEvent.setup();
    renderEditor();
    await user.click(await screen.findByRole("button", { name: "Edit" }));
    await user.type(
      screen.getByRole("textbox", { name: "Value for ALPHA" }),
      "saved-v2",
    );
    await user.click(
      screen.getByRole("button", { name: "Environment save options" }),
    );
    await user.click(
      await screen.findByRole("menuitem", { name: "Save only" }),
    );

    await waitFor(() =>
      expect(toastSuccess).toHaveBeenCalledWith(
        "Environment saved without a deploy. Use a standard deploy to apply any pending changes.",
      ),
    );
    expect(save).toHaveBeenCalledWith(
      "web",
      {
        envVars: [{ key: "ALPHA", value: "saved-v2" }],
        secretFiles: [],
      },
      "save_only",
    );
    expect(trigger).not.toHaveBeenCalled();
    expect(screen.getByRole("button", { name: "Edit" })).toBeInTheDocument();
    // Only the operator's status can assert a saved/runtime difference: a
    // masked replacement could also be a no-op or a revert to the live value.
    expect(
      screen.queryByText(/Saved changes aren't live yet/),
    ).not.toBeInTheDocument();
  });

  it("closes an accepted draft when refresh fails and explains how to recover", async () => {
    save.mockResolvedValueOnce({ refreshFailed: true });
    const user = userEvent.setup();
    renderEditor();
    await user.click(await screen.findByRole("button", { name: "Edit" }));
    await user.type(
      screen.getByRole("textbox", { name: "Value for ALPHA" }),
      "saved-v2",
    );
    await user.click(
      screen.getByRole("button", { name: "Environment save options" }),
    );
    await user.click(
      await screen.findByRole("menuitem", { name: "Save only" }),
    );

    await waitFor(() =>
      expect(toastWarning).toHaveBeenCalledWith(
        "Environment saved, but this page couldn't refresh. Reload to check the latest values and deployment status.",
      ),
    );
    expect(screen.getByRole("button", { name: "Edit" })).toBeInTheDocument();
    expect(screen.queryByDisplayValue("saved-v2")).not.toBeInTheDocument();
    expect(toastError).not.toHaveBeenCalled();
    expect(toastSuccess).not.toHaveBeenCalled();
    expect(save).toHaveBeenCalledOnce();
    expect(trigger).not.toHaveBeenCalled();
  });

  it("keeps an uncommitted draft on write failure and gives no saved feedback", async () => {
    save.mockRejectedValueOnce(new Error("write unavailable"));
    const user = userEvent.setup();
    renderEditor();
    await user.click(await screen.findByRole("button", { name: "Edit" }));
    await user.type(
      screen.getByRole("textbox", { name: "Value for ALPHA" }),
      "unsaved-v2",
    );
    await user.click(
      screen.getByRole("button", { name: "Environment save options" }),
    );
    await user.click(
      await screen.findByRole("menuitem", { name: "Save only" }),
    );

    await waitFor(() => expect(toastError).toHaveBeenCalled());
    expect(screen.getByDisplayValue("unsaved-v2")).toBeInTheDocument();
    expect(toastSuccess).not.toHaveBeenCalled();
    expect(toastWarning).not.toHaveBeenCalled();
    expect(trigger).not.toHaveBeenCalled();
  });

  // w2/m95 t003: bex owns PORT — the operator injects its own and would drop a
  // user's — so the editor says so as the key is typed and refuses to save it.
  it("flags a reserved key inline and blocks the save", async () => {
    const user = userEvent.setup();
    renderEditor();
    await user.click(await screen.findByRole("button", { name: "Edit" }));
    await user.click(screen.getByRole("button", { name: "Add variable" }));
    await user.click(
      await screen.findByRole("menuitem", { name: "Add variable" }),
    );
    const keys = screen.getAllByRole("textbox", { name: "Key" });
    await user.type(keys[keys.length - 1], "PORT");

    expect(
      await screen.findByText(
        "PORT is set by bex from the service port. Change the service's port field instead.",
      ),
    ).toBeInTheDocument();
    // w4/m121/t003: the sentence names the service's port field, so it links
    // to the control that changes it instead of dead-ending.
    expect(
      screen.getByRole("link", {
        name: "Change the service's port in Settings",
      }),
    ).toHaveAttribute("href", "/services/web/settings#port");
    expect(
      screen.getByRole("button", { name: "Save and deploy" }),
    ).toBeDisabled();
    expect(save).not.toHaveBeenCalled();
  });

  // w2/m95 t002: the value field used to be a single-line <input>, which the
  // browser flattens a pasted line break out of before React ever sees it — a
  // pasted PEM key was saved changed, silently.
  it("keeps the line breaks of a pasted multi-line value all the way to the patch", async () => {
    const user = userEvent.setup();
    renderEditor();
    await user.click(await screen.findByRole("button", { name: "Edit" }));
    const value = screen.getAllByRole("textbox", { name: /Value for / })[0];
    await user.click(value);
    await user.paste("first\nsecond\nthird");

    expect((value as HTMLTextAreaElement).value).toBe("first\nsecond\nthird");

    await user.click(screen.getByRole("button", { name: "Save and deploy" }));
    await waitFor(() =>
      expect(save).toHaveBeenCalledWith(
        "web",
        {
          envVars: [{ key: "ALPHA", value: "first\nsecond\nthird" }],
          secretFiles: [],
        },
        "deploy",
      ),
    );
  });

  // The other half of w1/099: an imported multi-line draft rendered run
  // together even though state held the newlines, because an <input> cannot
  // display them.
  it("displays an imported multi-line draft on its own lines", async () => {
    const user = userEvent.setup();
    renderEditor();
    await user.click(await screen.findByRole("button", { name: "Edit" }));
    await user.click(screen.getByRole("button", { name: "Add variable" }));
    await user.click(
      await screen.findByRole("menuitem", { name: "Import from .env" }),
    );
    await user.click(screen.getByRole("textbox", { name: "Dotenv contents" }));
    await user.paste(
      'PEM="-----BEGIN KEY-----\\nline-one\\nline-two\\n-----END KEY-----"',
    );
    await user.click(screen.getByRole("button", { name: "Add variables" }));

    const imported = screen.getByRole("textbox", { name: "Value for PEM" });
    expect(imported.tagName).toBe("TEXTAREA");
    expect((imported as HTMLTextAreaElement).value).toBe(
      "-----BEGIN KEY-----\nline-one\nline-two\n-----END KEY-----",
    );
  });

  it("stages dotenv import, generated secrets, and file upload without writing", async () => {
    const user = userEvent.setup();
    const { container } = renderEditor();
    await user.click(await screen.findByRole("button", { name: "Edit" }));

    await user.click(screen.getByRole("button", { name: "Add variable" }));
    await user.click(
      await screen.findByRole("menuitem", { name: "Import from .env" }),
    );
    await user.type(
      screen.getByRole("textbox", { name: "Dotenv contents" }),
      "IMPORTED=from-file",
    );
    await user.click(screen.getByRole("button", { name: "Add variables" }));
    expect(screen.getByDisplayValue("IMPORTED")).toBeInTheDocument();
    expect(screen.getByDisplayValue("from-file")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Add variable" }));
    await user.click(
      await screen.findByRole("menuitem", { name: "Generated secret" }),
    );
    const generatedRow =
      screen.getByDisplayValue("NEW_SECRET").parentElement?.parentElement;
    expect(
      (
        within(generatedRow!).getByRole("textbox", {
          name: "Value for NEW_SECRET",
        }) as HTMLInputElement
      ).value,
    ).toHaveLength(44);

    const upload = container.querySelector<HTMLInputElement>(
      'input[type="file"][multiple]',
    );
    expect(upload).not.toBeNull();
    const certificate = new File(["certificate"], "cert.pem", {
      type: "text/plain",
    });
    Object.defineProperty(certificate, "arrayBuffer", {
      value: vi
        .fn()
        .mockResolvedValue(new TextEncoder().encode("certificate").buffer),
    });
    await user.upload(upload!, certificate);
    expect(await screen.findByDisplayValue("cert.pem")).toBeInTheDocument();
    expect(save).not.toHaveBeenCalled();

    // A binary file (w8/029) is refused with the base64 hint, never staged
    // with its invalid bytes replaced by U+FFFD.
    const keystore = new File(["binary"], "keystore.p12", {
      type: "application/x-pkcs12",
    });
    Object.defineProperty(keystore, "arrayBuffer", {
      value: vi
        .fn()
        .mockResolvedValue(new Uint8Array([0x30, 0x82, 0xff, 0xfe]).buffer),
    });
    await user.upload(upload!, keystore);
    expect(
      await screen.findByText(/upload it base64-encoded/),
    ).toBeInTheDocument();
    expect(screen.queryByDisplayValue("keystore.p12")).not.toBeInTheDocument();
  });

  it("retries only deploy after configuration was already saved", async () => {
    trigger.mockResolvedValueOnce(null).mockResolvedValueOnce("dep-2");
    const user = userEvent.setup();
    renderEditor();
    await user.click(await screen.findByRole("button", { name: "Edit" }));
    await user.type(
      screen.getAllByRole("textbox", { name: /Value for / })[0],
      "replacement",
    );
    await user.click(
      screen.getByRole("button", { name: "Environment save options" }),
    );
    await user.click(
      await screen.findByRole("menuitem", {
        name: "Save, rebuild, and deploy",
      }),
    );

    expect(
      await screen.findByText("Configuration saved; rollout incomplete"),
    ).toBeInTheDocument();
    expect(save).toHaveBeenCalledTimes(1);
    expect(save.mock.calls[0][2]).toBe("save_only");
    await user.click(screen.getByRole("button", { name: "Retry rollout" }));
    await waitFor(() => expect(trigger).toHaveBeenCalledTimes(2));
    expect(save).toHaveBeenCalledTimes(1);
  });

  it("disables Edit with a role reason for a contributor without can_create", async () => {
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({ role: "CONTRIBUTOR", canCreate: false }),
    );
    renderEditor();
    expect(await screen.findByRole("button", { name: "Edit" })).toBeDisabled();
    // Reveal stays can_view_sensitive — a contributor who can view still reveals.
    expect(screen.getAllByRole("button", { name: "Reveal" })[0]).toBeEnabled();
  });

  it("lets an admin enter the write draft", async () => {
    const user = userEvent.setup();
    renderEditor();
    const edit = await screen.findByRole("button", { name: "Edit" });
    expect(edit).toBeEnabled();
    await user.click(edit);
    expect(screen.getByRole("button", { name: "Add variable" })).toBeEnabled();
    expect(
      screen.getByRole("button", { name: "Delete ALPHA" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Delete BETA" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("textbox", { name: "Value for ALPHA" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("textbox", { name: "Value for BETA" }),
    ).toBeInTheDocument();
    // Key textboxes keep the generic column name (their value is the identity).
    expect(screen.getAllByRole("textbox", { name: "Key" })).toHaveLength(2);
  });

  it("keeps generic names on a still-empty new env row (m101/t013)", async () => {
    const user = userEvent.setup();
    renderEditor();
    await user.click(await screen.findByRole("button", { name: "Edit" }));
    await user.click(screen.getByRole("button", { name: "Add variable" }));
    await user.click(
      await screen.findByRole("menuitem", { name: "Add variable" }),
    );
    expect(
      screen.getByRole("textbox", { name: /^Value$/ }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /^Delete$/ }),
    ).toBeInTheDocument();
  });

  it("names the per-row copy button and toast after the one value copied (w6/044)", async () => {
    const user = userEvent.setup();
    renderEditor();

    // The bulk "Copy env vars" label belongs to the Export menu only — no
    // per-row button may borrow it.
    const copyAlpha = await screen.findByRole("button", { name: "Copy ALPHA" });
    expect(
      screen.queryByRole("button", { name: "Copy env vars" }),
    ).not.toBeInTheDocument();

    await user.click(copyAlpha);
    await waitFor(() =>
      expect(toastSuccess).toHaveBeenCalledWith("ALPHA copied"),
    );
    expect(revealEnv).toHaveBeenCalledWith("ALPHA");

    // The secret-file row is not an env var at all — its copy is named and
    // toasted by file name too.
    await user.click(screen.getByRole("button", { name: "Copy token.txt" }));
    await waitFor(() =>
      expect(toastSuccess).toHaveBeenCalledWith("token.txt copied"),
    );
    expect(revealFile).toHaveBeenCalledWith("token.txt");
    expect(toastSuccess).not.toHaveBeenCalledWith("Environment copied");
  });

  it("enters the draft from the Secret Files empty state's own affordance (w6/057)", async () => {
    fileNames = [];
    const user = userEvent.setup();
    renderEditor();

    // Read mode: the empty state carries its own Add control (the enabling
    // Edit button lives under the Environment Variables header).
    expect(await screen.findByText("No secret files")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Add secret file" }));

    // Edit mode entered with a blank file row already staged.
    expect(screen.getByRole("textbox", { name: "File name" })).toHaveValue("");
    expect(
      screen.getByRole("button", { name: "Save and deploy" }),
    ).toBeInTheDocument();
    expect(save).not.toHaveBeenCalled();
  });

  it("shows the file-name rule as neutral help until the field is touched (w6/057)", async () => {
    fileNames = [];
    const user = userEvent.setup();
    renderEditor();
    await user.click(
      await screen.findByRole("button", { name: "Add secret file" }),
    );

    const nameInput = screen.getByRole("textbox", { name: "File name" });
    const rule =
      "Use up to 253 letters, digits, dots, dashes and underscores; not '.' or '..'.";
    // Pristine: the rule is visible but neutral — no alert, no aria-invalid.
    expect(screen.getByText(rule)).toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    expect(nameInput).toHaveAttribute("aria-invalid", "false");

    // Typing an invalid name upgrades the same rule to error styling.
    await user.type(nameInput, "bad/name");
    expect(await screen.findByRole("alert")).toHaveTextContent(rule);
    expect(nameInput).toHaveAttribute("aria-invalid", "true");
  });
  // w4/m120 t003: a blueprint service's render.yaml literals live on the App
  // spec, and bex-api refuses every write to them. The row has to say so
  // *before* anyone reveals its value, and offer no control that would produce
  // the refused write.
  it("renders a manifest-managed variable read-only with no edit or delete affordance", async () => {
    envKeys = [
      { id: "MESSAGE", key: "MESSAGE", managedBy: "blueprint" },
      { id: "BETA", key: "BETA", managedBy: "" },
    ];
    const user = userEvent.setup();
    renderEditor();

    // Read mode names the owner and still allows copying the value.
    expect(await screen.findByText("Managed by blueprint")).toBeInTheDocument();
    expect(
      screen.getByText(/declared in the service's render.yaml manifest/),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Copy MESSAGE" })).toBeEnabled();

    await user.click(screen.getByRole("button", { name: "Edit" }));

    // Draft mode: no key field, no value field, no delete for the flagged row.
    expect(screen.queryByDisplayValue("MESSAGE")).not.toBeInTheDocument();
    expect(
      screen.queryByRole("textbox", { name: "Value for MESSAGE" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Delete MESSAGE" }),
    ).not.toBeInTheDocument();
    expect(screen.getByText("Managed by blueprint")).toBeInTheDocument();

    // The ordinary row beside it is untouched.
    expect(screen.getByDisplayValue("BETA")).toBeEnabled();
    expect(
      screen.getByRole("textbox", { name: "Value for BETA" }),
    ).toBeEnabled();
    expect(screen.getByRole("button", { name: "Delete BETA" })).toBeEnabled();
  });

  it("never derives a write for a manifest-managed row from the draft", async () => {
    envKeys = [
      { id: "MESSAGE", key: "MESSAGE", managedBy: "blueprint" },
      { id: "BETA", key: "BETA", managedBy: "" },
    ];
    const user = userEvent.setup();
    renderEditor();
    await user.click(await screen.findByRole("button", { name: "Edit" }));

    // A .env import naming the manifest key must not smuggle the write in.
    await user.click(screen.getByRole("button", { name: "Add variable" }));
    await user.click(
      await screen.findByRole("menuitem", { name: "Import from .env" }),
    );
    await user.type(
      screen.getByRole("textbox", { name: "Dotenv contents" }),
      "MESSAGE=hijacked",
    );
    await user.click(screen.getByRole("button", { name: "Add variables" }));
    expect(screen.queryByDisplayValue("hijacked")).not.toBeInTheDocument();

    await user.type(
      screen.getByRole("textbox", { name: "Value for BETA" }),
      "replacement",
    );
    await user.click(screen.getByRole("button", { name: "Save and deploy" }));

    await waitFor(() => expect(save).toHaveBeenCalledTimes(1));
    expect(save.mock.calls[0][1]).toEqual({
      envVars: [{ key: "BETA", value: "replacement" }],
      secretFiles: [],
    });
  });

  it("fails closed on a row whose manifest flag has not arrived yet", async () => {
    // Old cached data (pre-w4/m120) mid-refetch: the field is absent, not empty.
    envKeys = [{ id: "ALPHA", key: "ALPHA" }];
    envKeysLoading = true;
    const user = userEvent.setup();
    renderEditor();

    await user.click(await screen.findByRole("button", { name: "Edit" }));
    expect(
      screen.queryByRole("textbox", { name: "Value for ALPHA" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Delete ALPHA" }),
    ).not.toBeInTheDocument();
  });

  // The dashboard invents no copy of its own for this refusal — the server names
  // the manifest and the remedy, and that text is what the user reads
  // (mutation-error-toast-invariant.test.ts guards the passthrough).
  it("surfaces the server's manifest refusal verbatim", async () => {
    const refusal =
      "BETA is declared in this service's render.yaml manifest, which owns its value — edit the manifest and sync the blueprint, or declare the variable with `sync: false` so the dashboard owns it instead";
    save.mockRejectedValueOnce(
      codedGraphQLError("ENV_VAR_MANIFEST_MANAGED", {}, refusal),
    );
    const user = userEvent.setup();
    renderEditor();
    await user.click(await screen.findByRole("button", { name: "Edit" }));
    await user.type(
      screen.getByRole("textbox", { name: "Value for BETA" }),
      "replacement",
    );
    await user.click(screen.getByRole("button", { name: "Save and deploy" }));

    await waitFor(() => expect(toastError).toHaveBeenCalledWith(refusal));
  });
});

// w4/139: restoring a stranded draft reinstated its stored row ids while this
// mount's allocator restarted at 0, so the next added row could be handed an id
// a restored row already held. `updateRow` acts on EVERY row whose id matches,
// so one key edit rewrote two rows, one value edit rewrote two, and one delete
// removed two — live, the stored draft showed ids ["new-env:0", "new-env:0"].
// Restoration is exercised for real here (a genuine unmount/remount over
// sessionStorage), because mocking it away is exactly what left this untested.
describe("ServiceEnvironmentEditor restored-draft row identity (w4/139)", () => {
  beforeEach(() => {
    sessionStorage.clear();
    envKeys = [];
    fileNames = [];
  });

  async function addVariable(user: ReturnType<typeof userEvent.setup>) {
    await user.click(screen.getByRole("button", { name: "Add variable" }));
    await user.click(
      await screen.findByRole("menuitem", { name: "Add variable" }),
    );
  }

  it("keeps a restored variable independent of one added afterwards", async () => {
    const user = userEvent.setup();
    const first = renderEditor();

    await user.click(await screen.findByRole("button", { name: "Edit" }));
    await addVariable(user);
    await user.type(screen.getByRole("textbox", { name: "Key" }), "QA_FIRST");
    await user.type(
      screen.getByRole("textbox", { name: "Value for QA_FIRST" }),
      "qa-first-draft",
    );

    // The reload: this mount goes away, the draft survives in sessionStorage.
    first.unmount();
    renderEditor();
    expect(await screen.findByText("Restored unsaved changes")).toBeVisible();

    await addVariable(user);
    const keys = screen.getAllByRole("textbox", { name: "Key" });
    expect(keys).toHaveLength(2);
    await user.type(keys[1]!, "QA_SECOND");

    // The live symptom was BOTH keys becoming QA_SECOND.
    expect(keys[0]).toHaveValue("QA_FIRST");
    expect(keys[1]).toHaveValue("QA_SECOND");

    await user.type(
      screen.getByRole("textbox", { name: "Value for QA_SECOND" }),
      "qa-second-draft",
    );
    expect(
      screen.getByRole("textbox", { name: "Value for QA_FIRST" }),
    ).toHaveValue("qa-first-draft");

    // …and the duplicate-key validation that followed from it.
    expect(
      screen.queryByText("Each variable key must be unique."),
    ).not.toBeInTheDocument();
  });

  it("deletes only the row the user asked to delete", async () => {
    const user = userEvent.setup();
    const first = renderEditor();

    await user.click(await screen.findByRole("button", { name: "Edit" }));
    await addVariable(user);
    await user.type(screen.getByRole("textbox", { name: "Key" }), "QA_FIRST");

    first.unmount();
    renderEditor();
    expect(await screen.findByText("Restored unsaved changes")).toBeVisible();

    await addVariable(user);
    const keys = screen.getAllByRole("textbox", { name: "Key" });
    await user.type(keys[1]!, "QA_SECOND");

    await user.click(screen.getByRole("button", { name: "Delete QA_SECOND" }));

    // Live, this removed both rows and left zero key inputs.
    const remaining = screen.getAllByRole("textbox", { name: "Key" });
    expect(remaining).toHaveLength(1);
    expect(remaining[0]).toHaveValue("QA_FIRST");
  });

  it("keeps a restored secret file independent of one added afterwards", async () => {
    const user = userEvent.setup();
    const first = renderEditor();

    await user.click(await screen.findByRole("button", { name: "Edit" }));
    await user.click(screen.getByRole("button", { name: "Add secret file" }));
    await user.type(
      screen.getByRole("textbox", { name: "File name" }),
      "qa-first.txt",
    );

    first.unmount();
    renderEditor();
    expect(await screen.findByText("Restored unsaved changes")).toBeVisible();

    await user.click(screen.getByRole("button", { name: "Add secret file" }));
    const names = screen.getAllByRole("textbox", { name: "File name" });
    expect(names).toHaveLength(2);
    await user.type(names[1]!, "qa-second.txt");

    expect(names[0]).toHaveValue("qa-first.txt");
    expect(names[1]).toHaveValue("qa-second.txt");
  });

  it("carries every non-id field of a restored draft through untouched", async () => {
    // A server-backed row (existing key), an edited value, and a deletion all
    // have to survive re-keying — the ids are local, everything else is state.
    envKeys = [{ id: "ALPHA", key: "ALPHA", managedBy: "" }];
    const user = userEvent.setup();
    const first = renderEditor();

    await user.click(await screen.findByRole("button", { name: "Edit" }));
    await addVariable(user);
    const keys = screen.getAllByRole("textbox", { name: "Key" });
    await user.type(keys[keys.length - 1]!, "QA_NEW");
    await user.type(
      screen.getByRole("textbox", { name: "Value for QA_NEW" }),
      "qa-new-draft",
    );

    first.unmount();
    renderEditor();
    expect(await screen.findByText("Restored unsaved changes")).toBeVisible();

    // The persisted row is still recognised as persisted (not re-created), and
    // the new row still carries its typed value.
    const restoredKeys = screen.getAllByRole("textbox", { name: "Key" });
    expect(
      restoredKeys.map((input) => (input as HTMLInputElement).value),
    ).toContain("QA_NEW");
    expect(
      screen.getByRole("textbox", { name: "Value for QA_NEW" }),
    ).toHaveValue("qa-new-draft");
    expect(
      screen.getByRole("textbox", { name: "Value for ALPHA" }),
    ).toBeInTheDocument();
  });
});

// w4/260: a background capability refresh passes through "checking", which
// gives each action a reason tooltip for a moment. PermissionTooltip then moves
// its child in the tree; a DropdownMenu root inside it remounted and dropped
// the open menu. The root now stays put — the open menu survives, refuses to
// act while checking, and recovers without replaying anything.
describe("ServiceEnvironmentEditor menus across a capability refresh (w4/260)", () => {
  let current = mockCapabilities();
  const listeners = new Set<() => void>();
  function subscribe(listener: () => void) {
    listeners.add(listener);
    return () => listeners.delete(listener);
  }
  function setCapabilities(next: ReturnType<typeof mockCapabilities>) {
    current = next;
    act(() => listeners.forEach((listener) => listener()));
  }
  const checking = () =>
    mockCapabilities({
      role: null,
      canCreate: false,
      canViewSensitive: false,
      loading: true,
      loaded: false,
      stale: true,
    });

  beforeEach(() => {
    current = mockCapabilities();
    listeners.clear();
    vi.mocked(useCapabilities).mockImplementation(() =>
      useSyncExternalStore(subscribe, () => current),
    );
  });

  async function expectSurvivesChecking(itemName: string) {
    const menu = await screen.findByRole("menu");
    setCapabilities(checking());
    expect(screen.getByRole("menu")).toBe(menu);
    const item = screen.getByRole("menuitem", { name: itemName });
    expect(item).toHaveAttribute("aria-disabled", "true");
    await userEvent.setup().click(item);
    expect(screen.getByRole("menu")).toBe(menu);
    setCapabilities(mockCapabilities());
    expect(screen.getByRole("menu")).toBe(menu);
    expect(
      screen.getByRole("menuitem", { name: itemName }),
    ).not.toHaveAttribute("aria-disabled");
  }

  it("keeps Export open and inert while checking, then copies on request", async () => {
    const user = userEvent.setup();
    renderEditor();
    await user.click(await screen.findByRole("button", { name: "Export" }));
    await expectSurvivesChecking("Copy env vars");
    expect(revealEnv).not.toHaveBeenCalled();

    await user.click(screen.getByRole("menuitem", { name: "Copy env vars" }));
    await waitFor(() =>
      expect(toastSuccess).toHaveBeenCalledWith("Environment copied"),
    );
    expect(revealEnv).toHaveBeenCalledTimes(2);
    expect(screen.queryByRole("menu")).toBeNull();
  });

  it("keeps Add variable open and inert while checking, then closes on Escape", async () => {
    const user = userEvent.setup();
    renderEditor();
    await user.click(await screen.findByRole("button", { name: "Edit" }));
    const rows = screen.getAllByRole("textbox", {
      name: /Value for /,
      hidden: true,
    }).length;
    await user.click(screen.getByRole("button", { name: "Add variable" }));
    await expectSurvivesChecking("Generated secret");
    expect(
      screen.getAllByRole("textbox", { name: /Value for /, hidden: true }),
    ).toHaveLength(rows);
    expect(screen.queryByDisplayValue("NEW_SECRET")).toBeNull();

    await user.keyboard("{Escape}");
    expect(screen.queryByRole("menu")).toBeNull();
    expect(
      screen.getAllByRole("textbox", { name: /Value for /, hidden: true }),
    ).toHaveLength(rows);
  });

  it("keeps Save options open and inert while checking, then saves once on request", async () => {
    const user = userEvent.setup();
    renderEditor();
    await user.click(await screen.findByRole("button", { name: "Edit" }));
    await user.type(
      screen.getByRole("textbox", { name: "Value for ALPHA" }),
      "v2",
    );
    await user.click(
      screen.getByRole("button", { name: "Environment save options" }),
    );
    await expectSurvivesChecking("Save only");
    expect(save).not.toHaveBeenCalled();

    await user.click(screen.getByRole("menuitem", { name: "Save only" }));
    await waitFor(() => expect(save).toHaveBeenCalledTimes(1));
    expect(save).toHaveBeenCalledWith(
      "web",
      { envVars: [{ key: "ALPHA", value: "v2" }], secretFiles: [] },
      "save_only",
    );
  });
});

// w4/m205: two file selections can decode at once. Each used to judge names by
// the draft it started from, so both accepted the same name and the editor
// built a draft its own Save refuses. Reads are held open here so the tests
// choose the completion order.
describe("ServiceEnvironmentEditor overlapping file uploads (w4/m205)", () => {
  function heldFile(name: string, text: string) {
    const file = new File([text], name, { type: "text/plain" });
    let release: (bytes: ArrayBuffer) => void = () => undefined;
    const read = new Promise<ArrayBuffer>((resolve) => {
      release = resolve;
    });
    Object.defineProperty(file, "arrayBuffer", { value: () => read });
    return {
      file,
      land: () =>
        act(async () => {
          release(new TextEncoder().encode(text).buffer as ArrayBuffer);
          await read;
        }),
    };
  }
  function select(container: HTMLElement, ...files: File[]) {
    const input = container.querySelector<HTMLInputElement>(
      'input[type="file"][multiple]',
    )!;
    fireEvent.change(input, { target: { files } });
  }
  const rows = (name: string) => screen.queryAllByDisplayValue(name);
  const skippedNotice = () => screen.queryByText(/upload it base64-encoded/);
  const saveButton = () =>
    screen.getByRole("button", { name: "Save and deploy" });
  async function edit() {
    const user = userEvent.setup();
    const view = renderEditor();
    await user.click(await screen.findByRole("button", { name: "Edit" }));
    return { user, ...view };
  }
  async function savedFiles(user: ReturnType<typeof userEvent.setup>) {
    await user.click(saveButton());
    await waitFor(() => expect(save).toHaveBeenCalled());
    return save.mock.calls[0][1].secretFiles;
  }

  it.each([
    ["first selection lands first", [0, 1]],
    ["second selection lands first", [1, 0]],
  ])(
    "keeps one copy of a name selected twice while decoding (%s)",
    async (_label, order) => {
      const { user, container } = await edit();
      const batches = [
        [heldFile("dup.txt", "from-first"), heldFile("a.txt", "a")],
        [heldFile("dup.txt", "from-second"), heldFile("b.txt", "b")],
      ];
      select(container, ...batches[0].map((held) => held.file));
      select(container, ...batches[1].map((held) => held.file));
      for (const index of order) {
        for (const held of batches[index]) await held.land();
      }

      await waitFor(() => expect(rows("b.txt")).toHaveLength(1));
      expect(rows("a.txt")).toHaveLength(1);
      expect(rows("dup.txt")).toHaveLength(1);
      expect(
        screen.queryByText(/secret file name must be unique/i),
      ).not.toBeInTheDocument();
      expect(skippedNotice()).toBeInTheDocument();
      expect(saveButton()).toBeEnabled();
      // The copy that reached the draft first keeps its contents.
      const kept = order[0] === 0 ? "from-first" : "from-second";
      expect(await savedFiles(user)).toEqual(
        expect.arrayContaining([
          { name: "dup.txt", content: kept },
          { name: "a.txt", content: "a" },
          { name: "b.txt", content: "b" },
        ]),
      );
    },
  );

  it("keeps every distinct file from two overlapping selections without a notice", async () => {
    const { container } = await edit();
    const first = heldFile("one.txt", "1");
    const second = heldFile("two.txt", "2");
    select(container, first.file);
    select(container, second.file);
    await second.land();
    await first.land();
    await waitFor(() => expect(rows("one.txt")).toHaveLength(1));
    expect(rows("two.txt")).toHaveLength(1);
    expect(skippedNotice()).not.toBeInTheDocument();
    expect(saveButton()).toBeEnabled();
  });

  it("skips a duplicate selected after the first batch settled, and one repeated inside a batch", async () => {
    const { container } = await edit();
    const first = heldFile("cert.pem", "first");
    select(container, first.file);
    await first.land();
    await waitFor(() => expect(rows("cert.pem")).toHaveLength(1));
    expect(skippedNotice()).not.toBeInTheDocument();

    const again = heldFile("cert.pem", "second");
    select(container, again.file);
    await again.land();
    await waitFor(() => expect(skippedNotice()).toBeInTheDocument());
    expect(rows("cert.pem")).toHaveLength(1);

    const twin = [heldFile("twin.txt", "x"), heldFile("twin.txt", "y")];
    select(container, ...twin.map((held) => held.file));
    for (const held of twin) await held.land();
    await waitFor(() => expect(rows("twin.txt")).toHaveLength(1));
    expect(saveButton()).toBeEnabled();
  });

  it("treats a padded name already in the draft as taken, and a masked saved file too", async () => {
    const { user, container } = await edit();
    const first = heldFile("cert.pem", "first");
    select(container, first.file);
    await first.land();
    const name = (await screen.findByDisplayValue(
      "cert.pem",
    )) as HTMLInputElement;
    await user.clear(name);
    await user.type(name, "  key.pem ");

    const padded = heldFile("key.pem", "second");
    const masked = heldFile("token.txt", "replacement");
    select(container, padded.file, masked.file);
    await padded.land();
    await masked.land();
    await waitFor(() => expect(skippedNotice()).toBeInTheDocument());
    // Only the padded row itself (the matcher trims) — no second key.pem.
    expect(rows("key.pem")).toHaveLength(1);
    expect(rows("token.txt")).toHaveLength(1);
    expect(revealFile).not.toHaveBeenCalled();
  });

  it("accepts an upload under the name of a file staged for deletion", async () => {
    const { user, container } = await edit();
    const saved = screen.getByDisplayValue("token.txt");
    const row = saved.closest("li, tr, div[data-row], div")!.parentElement!;
    await user.click(
      within(row).getAllByRole("button", { name: /delete|remove/i })[0],
    );
    const replacement = heldFile("token.txt", "new-token");
    select(container, replacement.file);
    await replacement.land();
    await waitFor(() =>
      expect(screen.getAllByDisplayValue("token.txt").length).toBeGreaterThan(
        0,
      ),
    );
    expect(skippedNotice()).not.toBeInTheDocument();
    expect(await savedFiles(user)).toEqual(
      expect.arrayContaining([{ name: "token.txt", content: "new-token" }]),
    );
  });

  it("drops a read that finishes after Cancel, even once a new Edit is open", async () => {
    const { user, container } = await edit();
    const late = heldFile("late.txt", "late");
    select(container, late.file);
    await user.click(screen.getByRole("button", { name: "Cancel" }));
    await user.click(await screen.findByRole("button", { name: "Edit" }));
    await late.land();
    const fresh = heldFile("fresh.txt", "fresh");
    select(container, fresh.file);
    await fresh.land();
    await waitFor(() => expect(rows("fresh.txt")).toHaveLength(1));
    expect(rows("late.txt")).toHaveLength(0);
    expect(skippedNotice()).not.toBeInTheDocument();
  });

  it("ignores a read that finishes after the editor is gone", async () => {
    const { container, unmount } = await edit();
    const late = heldFile("late.txt", "late");
    select(container, late.file);
    unmount();
    await late.land();
    expect(save).not.toHaveBeenCalled();
  });

  it("reports an unreadable file beside an accepted one from the same selection", async () => {
    const { container } = await edit();
    const good = heldFile("good.txt", "ok");
    const binary = new File(["x"], "keystore.p12");
    Object.defineProperty(binary, "arrayBuffer", {
      value: () =>
        Promise.resolve(new Uint8Array([0x30, 0x82, 0xff, 0xfe]).buffer),
    });
    select(container, binary, good.file);
    await good.land();
    await waitFor(() => expect(rows("good.txt")).toHaveLength(1));
    expect(rows("keystore.p12")).toHaveLength(0);
    expect(skippedNotice()).toBeInTheDocument();
  });
});

// w4/285: the upload decoder dropped a leading UTF-8 byte-order mark, so the
// saved file was three bytes shorter than the one selected.
describe("ServiceEnvironmentEditor uploaded file contents (w4/285)", () => {
  function bytesFile(name: string, bytes: number[]) {
    const file = new File([new Uint8Array(bytes)], name);
    Object.defineProperty(file, "arrayBuffer", {
      value: () => Promise.resolve(new Uint8Array(bytes).buffer),
    });
    return file;
  }
  const utf8 = (text: string) => Array.from(new TextEncoder().encode(text));
  const BOM = [0xef, 0xbb, 0xbf];

  it("saves the exact text of each accepted upload, a leading byte-order mark included", async () => {
    const user = userEvent.setup();
    const { container } = renderEditor();
    await user.click(await screen.findByRole("button", { name: "Edit" }));
    const input = container.querySelector<HTMLInputElement>(
      'input[type="file"][multiple]',
    )!;
    fireEvent.change(input, {
      target: {
        files: [
          bytesFile("bom.txt", [...BOM, ...utf8("qa-bom\n")]),
          bytesFile("crlf.txt", utf8("one\r\ntwo\r\n")),
          bytesFile("inner.txt", utf8("a\uFEFFb")),
          bytesFile("empty.txt", []),
        ],
      },
    });
    expect(await screen.findByDisplayValue("empty.txt")).toBeInTheDocument();
    expect(
      screen.queryByText(/upload it base64-encoded/),
    ).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Save and deploy" }));
    await waitFor(() => expect(save).toHaveBeenCalled());
    const saved = save.mock.calls[0][1].secretFiles;
    expect(saved).toEqual([
      { name: "bom.txt", content: "\uFEFFqa-bom\n" },
      { name: "crlf.txt", content: "one\r\ntwo\r\n" },
      { name: "inner.txt", content: "a\uFEFFb" },
      { name: "empty.txt", content: "" },
    ]);
    expect(saved[0].content.codePointAt(0)).toBe(0xfeff);
  });

  it("still refuses invalid UTF-8 and NUL bytes, with or without a byte-order mark", async () => {
    const user = userEvent.setup();
    const { container } = renderEditor();
    await user.click(await screen.findByRole("button", { name: "Edit" }));
    const input = container.querySelector<HTMLInputElement>(
      'input[type="file"][multiple]',
    )!;
    fireEvent.change(input, {
      target: {
        files: [
          bytesFile("bad.bin", [...BOM, 0xff, 0xfe]),
          bytesFile("nul.txt", [...BOM, 0x61, 0x00]),
          bytesFile("good.txt", utf8("ok")),
        ],
      },
    });
    expect(await screen.findByDisplayValue("good.txt")).toBeInTheDocument();
    expect(screen.getByText(/upload it base64-encoded/)).toBeInTheDocument();
    expect(screen.queryByDisplayValue("bad.bin")).not.toBeInTheDocument();
    expect(screen.queryByDisplayValue("nul.txt")).not.toBeInTheDocument();
  });
});

// w4/286: the generator judged names by the raw key text while validation and
// the save patch trim it, so a padded existing key let it add a duplicate that
// disabled Save.
describe("ServiceEnvironmentEditor generated secret names (w4/286)", () => {
  async function generate(user: ReturnType<typeof userEvent.setup>) {
    await user.click(screen.getByRole("button", { name: "Add variable" }));
    await user.click(
      await screen.findByRole("menuitem", { name: "Generated secret" }),
    );
  }
  const keys = () =>
    screen
      .getAllByRole<HTMLInputElement>("textbox")
      .map((input) => input.value)
      .filter((value) => /NEW_SECRET/.test(value));

  it("skips a name an existing padded key already holds, and the draft stays saveable", async () => {
    envKeys = [{ id: "NEW_SECRET", key: "NEW_SECRET", managedBy: "" }];
    const user = userEvent.setup();
    renderEditor();
    await user.click(await screen.findByRole("button", { name: "Edit" }));
    const existing = screen.getByDisplayValue("NEW_SECRET");
    await user.clear(existing);
    await user.type(existing, " NEW_SECRET ");

    await generate(user);
    expect(keys()).toEqual([" NEW_SECRET ", "NEW_SECRET_2"]);
    expect(
      screen.queryByText(/variable key must be unique/i),
    ).not.toBeInTheDocument();
    const submit = screen.getByRole("button", { name: "Save and deploy" });
    expect(submit).toBeEnabled();

    await user.click(submit);
    await waitFor(() => expect(save).toHaveBeenCalled());
    // Only the generated key is written; the padded row is the same saved key.
    const { envVars } = save.mock.calls[0][1];
    expect(envVars).toHaveLength(1);
    expect(envVars[0].key).toBe("NEW_SECRET_2");
    expect(envVars[0].value).toHaveLength(44);
  });

  it("walks past padded suffixes in order", async () => {
    envKeys = [
      { id: "NEW_SECRET", key: "NEW_SECRET", managedBy: "" },
      { id: "NEW_SECRET_2", key: "NEW_SECRET_2", managedBy: "" },
    ];
    const user = userEvent.setup();
    renderEditor();
    await user.click(await screen.findByRole("button", { name: "Edit" }));
    const second = screen.getByDisplayValue("NEW_SECRET_2");
    await user.clear(second);
    await user.type(second, "NEW_SECRET_2  ");

    await generate(user);
    await generate(user);
    expect(keys()).toEqual([
      "NEW_SECRET",
      "NEW_SECRET_2  ",
      "NEW_SECRET_3",
      "NEW_SECRET_4",
    ]);
    expect(
      screen.getByRole("button", { name: "Save and deploy" }),
    ).toBeEnabled();
  });

  it("still starts at NEW_SECRET, and treats a different-case key as another name", async () => {
    envKeys = [{ id: "new_secret", key: "new_secret", managedBy: "" }];
    const user = userEvent.setup();
    renderEditor();
    await user.click(await screen.findByRole("button", { name: "Edit" }));
    await generate(user);
    expect(screen.getByDisplayValue("NEW_SECRET")).toBeInTheDocument();
    expect(screen.queryByDisplayValue("NEW_SECRET_2")).not.toBeInTheDocument();
  });
});
