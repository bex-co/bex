import { describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ServiceRowActions } from "@/features/services/components/service-row-actions";
import type { ServiceView } from "@/features/services/types";

vi.mock("@/features/projects/hooks/use-move-to-project", () => ({
  useMoveToProject: () => ({
    projects: [],
    currentProjectId: () => null,
    moveTo: vi.fn(),
    removeFromProject: vi.fn(),
    busyId: null,
  }),
}));

const service = {
  id: "app",
  name: "app",
  type: "web_service",
  suspended: false,
  phase: "Running",
  url: null,
  createdAt: null,
  replicas: 1,
  revision: "r1",
} as ServiceView;

describe("ServiceRowActions", () => {
  it("retries suspend only after the exact protected-environment phrase", async () => {
    const onRun = vi
      .fn()
      .mockResolvedValueOnce({
        status: "confirmation_required",
        confirmation: "sudo suspend service app",
      })
      .mockResolvedValueOnce({ status: "success" });
    const user = userEvent.setup();
    render(
      <ServiceRowActions service={service} pending={null} onRun={onRun} />,
    );

    await user.click(screen.getByRole("button", { name: "Open actions menu" }));
    await user.click(screen.getByRole("menuitem", { name: "Suspend" }));
    const ordinaryConfirm = await screen.findByRole("alertdialog");
    await user.click(
      within(ordinaryConfirm).getByRole("button", { name: "Suspend" }),
    );

    expect(onRun).toHaveBeenNthCalledWith(1, "suspend", service);
    const protectedDialog = await screen.findByRole("dialog");
    const input = within(protectedDialog).getByLabelText(
      /sudo suspend service app/,
    );
    const retry = within(protectedDialog).getByRole("button", {
      name: "Suspend",
    });
    await user.type(input, "sudo suspend service ap");
    expect(retry).toBeDisabled();
    await user.type(input, "p");
    expect(retry).toBeEnabled();
    await user.click(retry);

    expect(onRun).toHaveBeenNthCalledWith(
      2,
      "suspend",
      service,
      "sudo suspend service app",
    );
  });
});
