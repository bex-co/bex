import { describe, expect, it, vi } from "vitest";
import { useState } from "react";
import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { EnvImportDialog } from "../env-import-dialog";

function paste(text: string) {
  fireEvent.change(screen.getByRole("textbox"), { target: { value: text } });
}

// w4/159: the import accepts a literal multi-line quoted value (a PEM key),
// and a refusal names what is wrong instead of "not a valid assignment".
describe("EnvImportDialog", () => {
  it("imports a PEM block pasted as a multi-line quoted value", async () => {
    const onImport = vi.fn();
    render(<EnvImportDialog open onOpenChange={vi.fn()} onImport={onImport} />);

    paste('QA_OK=1\nQA_PEM="-----BEGIN KEY-----\nMIIB\n-----END KEY-----"');
    await userEvent.click(
      screen.getByRole("button", { name: "Add variables" }),
    );

    expect(onImport).toHaveBeenCalledWith([
      { key: "QA_OK", value: "1", line: 1 },
      {
        key: "QA_PEM",
        value: "-----BEGIN KEY-----\nMIIB\n-----END KEY-----",
        line: 2,
      },
    ]);
  });

  it("says a quote was never closed, naming the line it opened on", async () => {
    render(<EnvImportDialog open onOpenChange={vi.fn()} onImport={vi.fn()} />);

    paste('QA_OK=1\nQA_PEM="-----BEGIN KEY-----\nMIIB');
    await userEvent.click(
      screen.getByRole("button", { name: "Add variables" }),
    );

    expect(screen.getByRole("alert")).toHaveTextContent(
      "The quote opened on line 2 is never closed.",
    );
  });

  it("says when a variable name is invalid", async () => {
    render(<EnvImportDialog open onOpenChange={vi.fn()} onImport={vi.fn()} />);

    paste("1BAD=x");
    await userEvent.click(
      screen.getByRole("button", { name: "Add variables" }),
    );

    expect(screen.getByRole("alert")).toHaveTextContent(
      /Line 1: the name must be letters, digits, and underscores/,
    );
  });

  // w4/273: an uploaded file reaches the parser untouched, so the opening
  // line's in-quote whitespace survives.
  it("keeps opening-line whitespace inside a multi-line quote from an uploaded file", async () => {
    const user = userEvent.setup();
    const onImport = vi.fn();
    render(<EnvImportDialog open onOpenChange={vi.fn()} onImport={onImport} />);

    const contents = 'export QA_V="line1  \nline2"\n';
    const dotenv = new File([contents], ".env", { type: "text/plain" });
    // jsdom's File lacks text(); the dialog reads the file with it.
    Object.defineProperty(dotenv, "text", {
      value: vi.fn().mockResolvedValue(contents),
    });
    await user.upload(
      document.querySelector<HTMLInputElement>('input[type="file"]')!,
      dotenv,
    );
    await waitFor(() =>
      expect(screen.getByRole("textbox")).toHaveValue(contents),
    );
    await user.click(screen.getByRole("button", { name: "Add variables" }));
    expect(onImport).toHaveBeenCalledWith([
      { key: "QA_V", value: "line1  \nline2", line: 1 },
    ]);
  });
});

// w4/m222 (a861): each file read completed on its own and wrote the preview,
// so a slow earlier file replaced a later choice, and a read still pending at
// Cancel repopulated the reopened dialog.
describe("EnvImportDialog read ownership (w4/m222)", () => {
  function deferredFile(name: string) {
    let resolve!: (text: string) => void;
    let reject!: (error: Error) => void;
    const file = new File(["x"], name, { type: "text/plain" });
    Object.defineProperty(file, "text", {
      value: () =>
        new Promise<string>((res, rej) => {
          resolve = res;
          reject = rej;
        }),
    });
    return {
      file,
      resolve: (text: string) => act(async () => resolve(text)),
      reject: () => act(async () => reject(new Error("unreadable"))),
    };
  }
  const fileInput = () =>
    document.querySelector<HTMLInputElement>('input[type="file"]')!;
  const textbox = () => screen.getByRole("textbox");
  const add = () => screen.getByRole("button", { name: "Add variables" });

  function Host({ onImport }: { onImport: (e: unknown) => void }) {
    const [open, setOpen] = useState(true);
    return (
      <>
        <button type="button" onClick={() => setOpen(true)}>
          Reopen
        </button>
        <button type="button" onClick={() => setOpen(false)}>
          Force close
        </button>
        <EnvImportDialog
          open={open}
          onOpenChange={setOpen}
          onImport={onImport}
        />
      </>
    );
  }

  it("keeps the later file when an earlier read finishes last", async () => {
    const user = userEvent.setup();
    const onImport = vi.fn();
    render(<Host onImport={onImport} />);
    const first = deferredFile("first.env");
    const second = deferredFile("second.env");

    await user.upload(fileInput(), first.file);
    await user.upload(fileInput(), second.file);
    await second.resolve("SOURCE=second\nEMPTY=\n");
    expect(textbox()).toHaveValue("SOURCE=second\nEMPTY=\n");
    await first.resolve("SOURCE=first\n");
    expect(textbox()).toHaveValue("SOURCE=second\nEMPTY=\n");

    await user.click(add());
    expect(onImport).toHaveBeenCalledWith([
      { key: "SOURCE", value: "second", line: 1 },
      { key: "EMPTY", value: "", line: 2 },
    ]);
  });

  it("does not let an earlier read's failure replace the later preview", async () => {
    const user = userEvent.setup();
    render(<Host onImport={vi.fn()} />);
    const first = deferredFile("first.env");
    const second = deferredFile("second.env");
    await user.upload(fileInput(), first.file);
    await user.upload(fileInput(), second.file);
    await second.resolve("SOURCE=second\n");
    await first.reject();
    expect(textbox()).toHaveValue("SOURCE=second\n");
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it.each(["Cancel", "Force close"])(
    "reopens empty after %s while a read is pending",
    async (closeWith) => {
      const user = userEvent.setup();
      render(<Host onImport={vi.fn()} />);
      const first = deferredFile("first.env");
      await user.upload(fileInput(), first.file);
      // "Force close" stands in for the parent closing the dialog itself
      // (a capability change); the modal hides it from the a11y tree.
      fireEvent.click(
        screen.getByRole("button", { name: closeWith, hidden: true }),
      );
      await first.resolve("SOURCE=first\n");
      await user.click(screen.getByRole("button", { name: "Reopen" }));
      expect(textbox()).toHaveValue("");
    },
  );

  it("waits for a replacement read before Add or the keyboard shortcut commit", async () => {
    const user = userEvent.setup();
    const onImport = vi.fn();
    render(<Host onImport={onImport} />);
    const first = deferredFile("first.env");
    await user.upload(fileInput(), first.file);
    await first.resolve("SOURCE=first\n");

    const second = deferredFile("second.env");
    await user.upload(fileInput(), second.file);
    expect(screen.getByRole("status")).toHaveTextContent("Reading file…");
    expect(add()).toBeDisabled();
    textbox().focus();
    await user.keyboard("{Control>}{Enter}{/Control}");
    expect(onImport).not.toHaveBeenCalled();

    await second.resolve("SOURCE=second\n");
    expect(add()).toBeEnabled();
    await user.click(add());
    expect(onImport).toHaveBeenCalledWith([
      { key: "SOURCE", value: "second", line: 1 },
    ]);
  });

  it("lets a failed later choice win over an earlier pending read", async () => {
    const user = userEvent.setup();
    const onImport = vi.fn();
    render(<Host onImport={onImport} />);
    const first = deferredFile("first.env");
    await user.upload(fileInput(), first.file);
    await first.resolve("SOURCE=first\n");

    const second = deferredFile("second.env");
    await user.upload(fileInput(), second.file);
    const oversize = new File([new Uint8Array(1024 * 1024 + 1)], "big.env", {
      type: "text/plain",
    });
    await user.upload(fileInput(), oversize);
    await second.resolve("SOURCE=second\n");

    expect(screen.getByRole("alert")).toHaveTextContent("1 MiB");
    expect(textbox()).toHaveValue("SOURCE=first\n");
    expect(add()).toBeDisabled();
    textbox().focus();
    await user.keyboard("{Control>}{Enter}{/Control}");
    expect(onImport).not.toHaveBeenCalled();
    // An explicit edit takes ownership back.
    await user.type(textbox(), "EXTRA=1");
    expect(add()).toBeEnabled();
  });

  it("lets a text edit supersede a pending read", async () => {
    const user = userEvent.setup();
    render(<Host onImport={vi.fn()} />);
    const first = deferredFile("first.env");
    await user.upload(fileInput(), first.file);
    await user.type(textbox(), "MANUAL=1");
    expect(screen.queryByRole("status")).not.toBeInTheDocument();
    await first.resolve("SOURCE=first\n");
    expect(textbox()).toHaveValue("MANUAL=1");
  });
});
