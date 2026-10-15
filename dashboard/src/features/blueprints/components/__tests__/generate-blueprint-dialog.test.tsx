import { describe, it, expect, vi, beforeEach } from "vitest";
import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import i18n from "@/i18n/init";
import { GenerateBlueprintDialog } from "../generate-blueprint-dialog";

const generate = vi.fn(async () => ({
  data: {
    generateBlueprint: {
      manifest: "services:\n  - name: web\n",
      filename: "render.yaml",
    },
  },
}));
const { toastSuccess, toastError } = vi.hoisted(() => ({
  toastSuccess: vi.fn(),
  toastError: vi.fn(),
}));
vi.mock("sonner", () => ({
  toast: { success: toastSuccess, error: toastError },
}));
vi.mock("@apollo/client/react", () => ({
  useLazyQuery: () => [generate, { loading: false }],
}));
vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({ currentWorkspaceId: "tea-a" }),
}));
vi.mock("@/features/services/hooks/use-services", () => ({
  useServices: () => ({ services: [{ id: "srv-1", name: "web" }] }),
}));
vi.mock("@/features/databases/hooks/use-databases", () => ({
  useDatabases: () => ({ databases: [{ id: "dpg-1", name: "app-db" }] }),
}));
vi.mock("@/features/keyvalue/hooks/use-key-values", () => ({
  useKeyValues: () => ({ keyValues: [] }),
}));
vi.mock("@/features/env-groups/hooks/use-env-groups", () => ({
  useEnvGroups: () => ({
    groups: [{ id: "evg-1", name: "shared-config" }],
  }),
}));

beforeEach(() => generate.mockClear());

describe("GenerateBlueprintDialog", () => {
  it("disables Generate until a resource is selected, then previews the yaml", async () => {
    const user = userEvent.setup();
    render(<GenerateBlueprintDialog open onOpenChange={() => {}} />);

    const button = screen.getByRole("button", { name: /^generate$/i });
    expect(button).toBeDisabled();
    expect(
      screen.getByText(/select at least one resource/i),
    ).toBeInTheDocument();

    await user.click(screen.getByText("web"));
    await user.click(screen.getByText("app-db"));
    await user.click(screen.getByText("shared-config"));
    expect(button).toBeEnabled();
    await user.click(button);

    expect(generate).toHaveBeenCalledWith({
      variables: {
        ownerId: "tea-a",
        serviceIds: ["srv-1"],
        postgresIds: ["dpg-1"],
        keyValueIds: [],
        envGroupIds: ["evg-1"],
      },
    });
    expect(await screen.findByText(/services:/)).toBeInTheDocument();
    // w4/248: selected group keys export as generateValue (generated if
    // missing, never prompted) — only service sync: false keys are prompted.
    const note = screen.getByText(/stored values are never exported/i);
    expect(note).toHaveTextContent(
      "Service variables marked sync: false are prompted for",
    );
    expect(note).toHaveTextContent(
      "including exported environment-group keys — keep an existing value, or get a generated one if missing",
    );
    expect(
      screen.getByRole("button", { name: /download render\.yaml/i }),
    ).toBeInTheDocument();
  });
});

// A long dockerCommand line made the dialog's implicit grid track grow to the
// YAML's intrinsic width (632px in a 526px frame), pushing the title and the
// Download action past the modal edge. The dialog now bounds its column so
// only the preview pans; geometry itself is verified in a real browser.
describe("GenerateBlueprintDialog with a wide manifest", () => {
  it("bounds the dialog column so only the preview scrolls horizontally", async () => {
    const wide = `services:\n- dockerCommand: sh -c '${"x".repeat(200)}'\n`;
    generate.mockResolvedValueOnce({
      data: { generateBlueprint: { manifest: wide, filename: "render.yaml" } },
    });
    const user = userEvent.setup();
    render(<GenerateBlueprintDialog open onOpenChange={() => {}} />);

    await user.click(screen.getByText("web"));
    await user.click(screen.getByRole("button", { name: /^generate$/i }));

    const pre = await screen.findByText(/dockerCommand/);
    expect(pre.tagName).toBe("PRE");
    expect(pre.textContent).toBe(wide);
    expect(pre).toHaveClass("overflow-auto");
    expect(pre.closest('[data-slot="dialog-content"]')).toHaveClass(
      "grid-cols-[minmax(0,1fr)]",
    );
  });
});

// w4/314: Copy announced success before the clipboard answered, so a denied
// write showed "render.yaml copied" and left an unhandled rejection.
describe("GenerateBlueprintDialog Copy", () => {
  async function generated(generate: RegExp | string = /^generate$/i) {
    const user = userEvent.setup();
    render(<GenerateBlueprintDialog open onOpenChange={() => {}} />);
    await user.click(screen.getByText("shared-config"));
    await user.click(screen.getByRole("button", { name: generate }));
    await screen.findByText(/services:/);
    return user;
  }

  beforeEach(() => {
    toastSuccess.mockClear();
    toastError.mockClear();
  });

  it("toasts success only after the write resolves, and a failure as a failure", async () => {
    const user = await generated();
    let resolve!: () => void;
    const write = vi
      .spyOn(navigator.clipboard, "writeText")
      .mockImplementationOnce(
        () => new Promise<void>((done) => (resolve = done)),
      )
      .mockRejectedValueOnce(
        new DOMException("Write permission denied.", "NotAllowedError"),
      )
      .mockResolvedValueOnce(undefined);
    const copy = screen.getByRole("button", { name: /^copy$/i });

    await user.click(copy);
    expect(write).toHaveBeenLastCalledWith("services:\n  - name: web\n");
    expect(toastSuccess).not.toHaveBeenCalled();
    await act(async () => resolve());
    expect(toastSuccess).toHaveBeenCalledTimes(1);
    expect(toastSuccess).toHaveBeenLastCalledWith("render.yaml copied");

    toastSuccess.mockClear();
    await user.click(copy);
    await waitFor(() =>
      expect(toastError).toHaveBeenCalledWith(
        "Couldn't copy render.yaml. Allow clipboard access and try again, or download it.",
      ),
    );
    expect(toastSuccess).not.toHaveBeenCalled();
    // The preview, Copy and Download stay usable, and a retry succeeds.
    expect(screen.getByText(/services:/)).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /download render\.yaml/i }),
    ).toBeEnabled();
    await user.click(copy);
    await waitFor(() => expect(toastSuccess).toHaveBeenCalledTimes(1));
    write.mockRestore();
  });

  it("has a Chinese failure message", async () => {
    await i18n.changeLanguage("zh");
    try {
      const user = await generated("生成");
      vi.spyOn(navigator.clipboard, "writeText").mockRejectedValueOnce(
        new DOMException("denied", "NotAllowedError"),
      );
      await user.click(screen.getByRole("button", { name: "复制" }));
      await waitFor(() =>
        expect(toastError).toHaveBeenCalledWith(
          "无法复制 render.yaml。请允许访问剪贴板后重试，或直接下载。",
        ),
      );
    } finally {
      await i18n.changeLanguage("en");
    }
  });
});
