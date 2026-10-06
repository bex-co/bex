import { afterEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import i18n from "@/i18n/init";
import { BlueprintConfirmationDialog } from "../blueprint-confirmation-dialog";
import type { BlueprintConfirmationRequired } from "@/features/blueprints/lib/takeover";

const PHRASE = "takeover blueprint blp-db136288mmqc73d4hpug";

// What blueprintConfirmationFromError returns for bex-api's refusals.
const resourceTakeover: BlueprintConfirmationRequired = {
  status: "confirmation_required",
  confirmation: PHRASE,
  takeover: {
    kind: "resource",
    phrase: PHRASE,
    resource: "docs",
    resourceKind: "key value",
    owningBlueprintId: "blp-db136288mmqc73d4hpug",
  },
};
const protectedEnvironment: BlueprintConfirmationRequired = {
  status: "confirmation_required",
  confirmation: "sudo deploy service api",
};

function renderDialog(pending: BlueprintConfirmationRequired | null) {
  const onDismiss = vi.fn();
  const onConfirm = vi.fn(async () => {});
  render(
    <BlueprintConfirmationDialog
      pending={pending}
      actionLabel="Deploy Blueprint"
      busy={false}
      onDismiss={onDismiss}
      onConfirm={onConfirm}
    />,
  );
  return { onDismiss, onConfirm };
}

describe("BlueprintConfirmationDialog", () => {
  afterEach(async () => {
    await i18n.changeLanguage("en");
  });

  it("stays closed with nothing pending", () => {
    renderDialog(null);
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("names the takeover and runs the server's phrase once typed", async () => {
    const { onConfirm } = renderDialog(resourceTakeover);
    const dialog = screen.getByRole("dialog");
    expect(dialog).toHaveTextContent(
      "Key Value “docs” is managed by Blueprint blp-db136288mmqc73d4hpug.",
    );
    const action = screen.getByRole("button", { name: "Deploy Blueprint" });
    expect(action).toBeDisabled();
    await userEvent.type(screen.getByRole("textbox"), PHRASE);
    await userEvent.click(action);
    expect(onConfirm).toHaveBeenCalledWith(PHRASE);
  });

  // w5/m125: zh reads the zh sentence, with the kind's product name rather
  // than bex-api's English display text ("key value"). Product names (Key
  // Value, Blueprint) stay as the zh copy writes them.
  it("renders a takeover in zh without English fragments", async () => {
    await i18n.changeLanguage("zh");
    renderDialog(resourceTakeover);
    const text = screen.getByRole("dialog").textContent ?? "";
    expect(text).toContain(
      "Key Value “docs” 由 Blueprint blp-db136288mmqc73d4hpug 管理。",
    );
    expect(text).not.toMatch(/key value|managed by|Continuing|Type the command/);
  });

  it("keeps the protected-environment copy for a protected refusal", () => {
    renderDialog(protectedEnvironment);
    expect(screen.getByRole("dialog")).toHaveTextContent("api");
    expect(screen.getByRole("dialog")).not.toHaveTextContent(/takeover/i);
  });

  it("dismisses through onDismiss", async () => {
    const { onDismiss } = renderDialog(protectedEnvironment);
    await userEvent.keyboard("{Escape}");
    expect(onDismiss).toHaveBeenCalled();
  });
});
