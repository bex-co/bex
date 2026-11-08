import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { AutomationActionBar } from "@/features/agent-automations/components/automation-action-bar";
import { automation } from "@/features/agent-automations/components/__tests__/fixtures";
import {
  automationActions,
  NO_ACTIONS,
  type AutomationActions,
} from "@/features/agent-automations/lib/display";

vi.mock("@tanstack/react-router", async (importOriginal) => {
  const actual =
    await importOriginal<typeof import("@tanstack/react-router")>();
  return {
    ...actual,
    Link: ({ children }: { children: ReactNode }) => (
      <a href="#edit">{children}</a>
    ),
  };
});

function setup(
  actions: AutomationActions,
  auto: ReturnType<typeof automation> | null = automation(),
) {
  const handlers = {
    onRunNow: vi.fn(),
    onToggle: vi.fn(),
    onTakeOver: vi.fn(async () => true),
    onDelete: vi.fn(async () => true),
  };
  render(
    <AutomationActionBar
      automation={auto}
      actions={actions}
      busy={false}
      {...handlers}
    />,
  );
  return { user: userEvent.setup(), ...handlers };
}

const enabled = (name: string) =>
  expect(screen.getByRole("button", { name })).toBeEnabled();
const disabled = (name: string) =>
  expect(screen.getByRole("button", { name })).toBeDisabled();

describe("AutomationActionBar role gating (ADR101 D4)", () => {
  it("the creator can run, pause, edit and delete but not take over", () => {
    setup(
      automationActions({ canCreate: true, canManage: false, isCreator: true }),
    );
    enabled("Run now");
    enabled("Pause");
    expect(screen.getByRole("link", { name: "Edit" })).toBeInTheDocument();
    enabled("Delete");
    disabled("Take over");
  });

  it("an admin who is not the creator cannot edit but can take over", () => {
    setup(
      automationActions({ canCreate: true, canManage: true, isCreator: false }),
    );
    enabled("Run now");
    enabled("Delete");
    enabled("Take over");
    disabled("Edit");
    expect(
      screen.queryByRole("link", { name: "Edit" }),
    ).not.toBeInTheDocument();
  });

  it("another developer sees every action disabled", () => {
    setup(
      automationActions({
        canCreate: true,
        canManage: false,
        isCreator: false,
      }),
    );
    for (const name of ["Run now", "Pause", "Edit", "Take over", "Delete"]) {
      disabled(name);
    }
  });

  it("shows Resume for a paused automation and refuses Run now until it is resumed", () => {
    setup(
      automationActions({ canCreate: true, canManage: true, isCreator: true }),
      automation({ enabled: true, pausedReason: "repeated_failures" }),
    );
    enabled("Resume");
    disabled("Run now");
  });

  it("while loading, every action is present and none is enabled", () => {
    setup(NO_ACTIONS, null);
    for (const name of ["Run now", "Pause", "Edit", "Take over", "Delete"]) {
      disabled(name);
    }
  });

  it("asks before taking over and before deleting, and only acts on confirm", async () => {
    const { user, onTakeOver, onDelete } = setup(
      automationActions({ canCreate: true, canManage: true, isCreator: false }),
    );
    await user.click(screen.getByRole("button", { name: "Take over" }));
    const takeOver = await screen.findByRole("alertdialog");
    expect(takeOver).toHaveTextContent("Take over nightly-deps?");
    expect(onTakeOver).not.toHaveBeenCalled();
    await user.click(
      within(takeOver).getByRole("button", { name: "Take over" }),
    );
    expect(onTakeOver).toHaveBeenCalledTimes(1);

    await user.click(screen.getByRole("button", { name: "Delete" }));
    const del = await screen.findByRole("alertdialog");
    expect(del).toHaveTextContent("Delete nightly-deps?");
    await user.click(within(del).getByRole("button", { name: "Cancel" }));
    expect(onDelete).not.toHaveBeenCalled();
  });
});
