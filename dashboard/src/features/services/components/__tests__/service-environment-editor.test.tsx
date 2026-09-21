import { beforeEach, describe, expect, it, vi } from "vitest";
import { CombinedGraphQLErrors } from "@apollo/client/errors";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { mockCapabilities } from "@/test/mocks/capabilities";
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
  classifyEnvVarError: () => null,
}));

vi.mock("@/features/services/hooks/use-secret-files", () => ({
  useSecretFileNames: () => ({
    names: fileNames,
    loading: false,
    error: undefined,
    refetch: refetchFiles,
  }),
  useRevealSecretFile: () => revealFile,
  classifySecretFileError: () => null,
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
    Object.defineProperty(certificate, "text", {
      value: vi.fn().mockResolvedValue("certificate"),
    });
    await user.upload(upload!, certificate);
    expect(await screen.findByDisplayValue("cert.pem")).toBeInTheDocument();
    expect(save).not.toHaveBeenCalled();
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
      "Use letters, digits, dot, dash and underscore; not '.' or '..'.";
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
      new CombinedGraphQLErrors({
        errors: [
          {
            message: refusal,
            extensions: { code: "ENV_VAR_MANIFEST_MANAGED" },
          },
        ],
      } as never),
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
