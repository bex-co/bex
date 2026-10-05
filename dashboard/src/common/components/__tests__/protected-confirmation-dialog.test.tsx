import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { ProtectedConfirmationDialog } from "../protected-confirmation-dialog";

const base = {
  open: true,
  resourceName: "api",
  requiredConfirmation: "sudo delete service api",
  actionLabel: "Delete",
  busy: false,
  onOpenChange: vi.fn(),
  onConfirm: vi.fn(async () => {}),
};

describe("ProtectedConfirmationDialog", () => {
  it("keeps the protected-environment copy by default", () => {
    render(<ProtectedConfirmationDialog {...base} />);
    expect(
      screen.getByText("Protected environment confirmation required"),
    ).toBeInTheDocument();
  });

  // w4/189: a Blueprint takeover shares the typed-phrase contract but must say
  // what it replaces, not that a protected environment was involved.
  it("lets a takeover replace the title and body", () => {
    render(
      <ProtectedConfirmationDialog
        {...base}
        requiredConfirmation="takeover blueprint blp-1"
        title="Replace Blueprint “bex”?"
        description="“bex” already tracks render.yaml on bex-co/bex@main."
      />,
    );
    expect(screen.getByText("Replace Blueprint “bex”?")).toBeInTheDocument();
    expect(
      screen.queryByText("Protected environment confirmation required"),
    ).not.toBeInTheDocument();
    expect(screen.getByText(/already tracks render.yaml/)).toBeInTheDocument();
  });
});
