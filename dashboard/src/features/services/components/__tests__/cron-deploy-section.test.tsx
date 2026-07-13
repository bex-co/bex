import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { CronDeploySection } from "@/features/services/components/cron-deploy-section";

const updateCronJob = vi.fn(async () => true);

vi.mock("@/features/services/hooks/use-cron-job", () => ({
  useCronJob: () => ({ updateCronJob, busy: false }),
}));

beforeEach(() => {
  updateCronJob.mockReset();
  updateCronJob.mockResolvedValue(true);
});

describe("CronDeploySection", () => {
  it("shows the current schedule and command in view mode", () => {
    render(
      <CronDeploySection
        serviceId="nightly"
        schedule="0 6 * * *"
        command="node daily.js"
      />,
    );
    expect(screen.getByText("0 6 * * *")).toBeInTheDocument();
    expect(screen.getByText("node daily.js")).toBeInTheDocument();
  });

  it("shows a placeholder when command is not set", () => {
    render(
      <CronDeploySection serviceId="nightly" schedule="0 6 * * *" command={null} />,
    );
    expect(
      screen.getByText("Uses the image's own default command."),
    ).toBeInTheDocument();
  });

  it("edit → save calls updateCronJob with the new schedule and command", async () => {
    const user = userEvent.setup();
    render(
      <CronDeploySection
        serviceId="nightly"
        schedule="0 6 * * *"
        command="node old.js"
      />,
    );

    await user.click(screen.getByRole("button", { name: /edit/i }));
    const schedInput = screen.getByDisplayValue("0 6 * * *");
    await user.clear(schedInput);
    await user.type(schedInput, "0 8 * * 1");
    await user.click(screen.getByRole("button", { name: /save/i }));

    expect(updateCronJob).toHaveBeenCalledWith("nightly", "0 8 * * 1", "node old.js");
  });

  it("blocks save and shows an error for an invalid cron expression", async () => {
    const user = userEvent.setup();
    render(
      <CronDeploySection serviceId="nightly" schedule="0 6 * * *" command={null} />,
    );

    await user.click(screen.getByRole("button", { name: /edit/i }));
    const schedInput = screen.getByDisplayValue("0 6 * * *");
    await user.clear(schedInput);
    await user.type(schedInput, "not valid");
    await user.click(screen.getByRole("button", { name: /save/i }));

    expect(updateCronJob).not.toHaveBeenCalled();
    expect(screen.getByText(/valid.*5-field/i)).toBeInTheDocument();
  });

  it("blocks save and shows a required error for an empty schedule", async () => {
    const user = userEvent.setup();
    render(
      <CronDeploySection serviceId="nightly" schedule="0 6 * * *" command={null} />,
    );

    await user.click(screen.getByRole("button", { name: /edit/i }));
    await user.clear(screen.getByDisplayValue("0 6 * * *"));
    await user.click(screen.getByRole("button", { name: /save/i }));

    expect(updateCronJob).not.toHaveBeenCalled();
    expect(screen.getByText(/required/i)).toBeInTheDocument();
  });

  it("cancel discards the draft without calling updateCronJob", async () => {
    const user = userEvent.setup();
    render(
      <CronDeploySection serviceId="nightly" schedule="0 6 * * *" command={null} />,
    );

    await user.click(screen.getByRole("button", { name: /edit/i }));
    const schedInput = screen.getByDisplayValue("0 6 * * *");
    await user.clear(schedInput);
    await user.type(schedInput, "0 0 * * *");
    await user.click(screen.getByRole("button", { name: /cancel/i }));

    expect(updateCronJob).not.toHaveBeenCalled();
    // Returns to view mode showing the original schedule.
    expect(screen.getByText("0 6 * * *")).toBeInTheDocument();
  });

  it("remains in edit mode when updateCronJob resolves false (server error)", async () => {
    updateCronJob.mockResolvedValue(false);
    const user = userEvent.setup();
    render(
      <CronDeploySection serviceId="nightly" schedule="0 6 * * *" command={null} />,
    );

    await user.click(screen.getByRole("button", { name: /edit/i }));
    await user.click(screen.getByRole("button", { name: /save/i }));

    // The Save button should still be visible (still in editing state).
    expect(screen.getByRole("button", { name: /save/i })).toBeInTheDocument();
  });
});
