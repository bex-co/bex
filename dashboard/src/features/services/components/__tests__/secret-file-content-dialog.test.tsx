import { describe, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import { SecretFileContentDialog } from "@/features/services/components/secret-file-content-dialog";

function deferred() {
  let resolve!: (value: string) => void;
  let reject!: (reason: unknown) => void;
  const promise = new Promise<string>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

function renderDialog(
  props: Partial<Parameters<typeof SecretFileContentDialog>[0]> = {},
) {
  return render(
    <SecretFileContentDialog
      open
      name="config.txt"
      content={null}
      onOpenChange={vi.fn()}
      onSave={vi.fn()}
      {...props}
    />,
  );
}

// The loading placeholder must reserve exactly the editor's fixed region, and
// the editor must not auto-grow with its contents, so the dialog keeps its size
// and its footer stays put when the read settles.
function heightClasses(el: Element) {
  return [...el.classList].filter((c) => /^(min-|max-)?h-/.test(c)).sort();
}

describe("SecretFileContentDialog", () => {
  it("reserves the ready editor's fixed region while the contents load", async () => {
    const read = deferred();
    renderDialog({ reveal: () => read.promise });

    const pending = document.querySelector(
      '[data-slot="secret-file-content-pending"]',
    );
    expect(pending).toHaveTextContent("Loading contents…");
    expect(screen.getByRole("button", { name: "Done" })).toBeDisabled();
    const pendingHeights = heightClasses(pending!);

    read.resolve("line one\nline two\n");
    const editor = await screen.findByRole("textbox", { name: "Contents" });
    expect(editor).toHaveValue("line one\nline two\n");
    expect(screen.getByRole("button", { name: "Done" })).toBeEnabled();

    expect(pendingHeights).toEqual(["h-64"]);
    expect(heightClasses(editor).filter((c) => c !== "min-h-16")).toEqual(
      pendingHeights,
    );
    expect(editor).toHaveClass("field-sizing-fixed");
    expect(editor).not.toHaveClass("field-sizing-content");
  });

  it("opens supplied contents ready, at the same fixed size", () => {
    renderDialog({ content: "already staged", reveal: vi.fn() });

    const editor = screen.getByRole("textbox", { name: "Contents" });
    expect(editor).toHaveValue("already staged");
    expect(editor).toHaveClass("h-64", "field-sizing-fixed");
    expect(
      document.querySelector('[data-slot="secret-file-content-pending"]'),
    ).toBeNull();
    expect(screen.getByRole("button", { name: "Done" })).toBeEnabled();
  });

  it("keeps Done disabled when the read fails", async () => {
    const read = deferred();
    renderDialog({ reveal: () => read.promise });

    read.reject(new Error("boom"));
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Couldn't load this secret file's contents.",
    );
    await waitFor(() =>
      expect(screen.getByRole("button", { name: "Done" })).toBeDisabled(),
    );
  });
});
