import { beforeEach, describe, expect, it, vi } from "vitest";
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

vi.mock("@/features/services/hooks/use-env-vars", () => ({
  useEnvVarKeys: () => ({
    keys: [
      { id: "ALPHA", key: "ALPHA" },
      { id: "BETA", key: "BETA" },
    ],
    loading: false,
    error: undefined,
    refetch: refetchEnv,
  }),
  useRevealEnvVar: () => revealEnv,
  classifyEnvVarError: () => null,
}));

vi.mock("@/features/services/hooks/use-secret-files", () => ({
  useSecretFileNames: () => ({
    names: [{ id: "token.txt", name: "token.txt" }],
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
    expect(screen.queryByRole("textbox", { name: "Value" })).toBeNull();
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
    expect(screen.getAllByRole("textbox", { name: "Value" })).not.toHaveLength(
      0,
    );
    expect(revealEnv).not.toHaveBeenCalled();
  });

  it("stays permissive until capabilities are definitive", async () => {
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({
        role: null,
        canCreate: false,
        canViewSensitive: false,
        loading: true,
        loaded: false,
      }),
    );
    const user = userEvent.setup();
    renderEditor();

    expect(await screen.findByRole("button", { name: "Edit" })).toBeEnabled();
    expect(screen.getByRole("button", { name: "Export" })).toBeEnabled();
    await user.click(screen.getByRole("button", { name: "Edit" }));
    expect(screen.getAllByRole("textbox", { name: "Value" })).not.toHaveLength(
      0,
    );
  });

  it("freezes an already-open draft if create permission is revoked", async () => {
    const user = userEvent.setup();
    renderEditor();
    await user.click(await screen.findByRole("button", { name: "Edit" }));
    await user.type(
      screen.getAllByRole("textbox", { name: "Value" })[0],
      "pending",
    );

    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({ role: "CONTRIBUTOR", canCreate: false }),
    );
    // This last pre-revocation input event schedules the component render that
    // observes the freshly denied capability; all subsequent handlers are the
    // guarded versions.
    await user.type(screen.getAllByRole("textbox", { name: "Value" })[0], "x");

    expect(screen.getByRole("status")).toHaveTextContent(
      "Your role can’t make this change.",
    );
    expect(
      screen
        .getAllByRole("textbox", { name: "Value" })
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

  it("keeps edits local and Cancel restores the server snapshot", async () => {
    const user = userEvent.setup();
    renderEditor();
    await user.click(await screen.findByRole("button", { name: "Edit" }));
    const valueInputs = screen.getAllByRole("textbox", { name: "Value" });
    await user.type(valueInputs[0], "changed");
    expect(save).not.toHaveBeenCalled();
    await user.click(screen.getByRole("button", { name: "Cancel" }));
    expect(save).not.toHaveBeenCalled();
    expect(screen.getByRole("button", { name: "Edit" })).toBeInTheDocument();
    expect(screen.queryByDisplayValue("changed")).not.toBeInTheDocument();
  });

  it("commits one combined patch through Save and deploy", async () => {
    const user = userEvent.setup();
    renderEditor();
    await user.click(await screen.findByRole("button", { name: "Edit" }));
    const valueInputs = screen.getAllByRole("textbox", { name: "Value" });
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
          name: "Value",
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
      screen.getAllByRole("textbox", { name: "Value" })[0],
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
});
