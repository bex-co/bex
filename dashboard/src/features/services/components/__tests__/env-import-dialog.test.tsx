import { describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
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
});
