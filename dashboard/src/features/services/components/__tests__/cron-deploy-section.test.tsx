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
  it("shows the current schedule and command in disabled inputs", () => {
    render(
      <CronDeploySection
        serviceId="nightly"
        schedule="0 6 * * *"
        command="node daily.js"
      />,
    );
    const sched = screen.getByRole("textbox", { name: "Schedule" });
    const cmd = screen.getByRole("textbox", { name: "Command" });
    expect(sched).toHaveValue("0 6 * * *");
    expect(sched).toBeDisabled();
    expect(cmd).toHaveValue("node daily.js");
  });

  it("shows an empty command input when command is not set", () => {
    render(
      <CronDeploySection
        serviceId="nightly"
        schedule="0 6 * * *"
        command={null}
      />,
    );
    expect(screen.getByRole("textbox", { name: "Command" })).toHaveValue("");
  });

  it("edits the schedule and saves both fields via updateCronJob", async () => {
    const user = userEvent.setup();
    render(
      <CronDeploySection
        serviceId="nightly"
        schedule="0 6 * * *"
        command="node old.js"
      />,
    );

    await user.click(screen.getByRole("button", { name: "Edit schedule" }));
    const schedInput = screen.getByRole("textbox", { name: "Schedule" });
    await user.clear(schedInput);
    await user.type(schedInput, "0 8 * * 1");
    await user.click(screen.getByRole("button", { name: "Save changes" }));

    expect(updateCronJob).toHaveBeenCalledWith(
      "nightly",
      "0 8 * * 1",
      "node old.js",
    );
  });

  it("edits the command and carries the persisted schedule", async () => {
    const user = userEvent.setup();
    render(
      <CronDeploySection
        serviceId="nightly"
        schedule="0 6 * * *"
        command="node old.js"
      />,
    );

    await user.click(screen.getByRole("button", { name: "Edit command" }));
    const cmdInput = screen.getByRole("textbox", { name: "Command" });
    await user.clear(cmdInput);
    await user.type(cmdInput, "node new.js");
    await user.click(screen.getByRole("button", { name: "Save changes" }));

    expect(updateCronJob).toHaveBeenCalledWith(
      "nightly",
      "0 6 * * *",
      "node new.js",
    );
  });

  it("blocks save and shows an error for an invalid cron expression", async () => {
    const user = userEvent.setup();
    render(
      <CronDeploySection
        serviceId="nightly"
        schedule="0 6 * * *"
        command={null}
      />,
    );

    await user.click(screen.getByRole("button", { name: "Edit schedule" }));
    const schedInput = screen.getByRole("textbox", { name: "Schedule" });
    await user.clear(schedInput);
    await user.type(schedInput, "not valid");

    expect(screen.getByRole("button", { name: "Save changes" })).toBeDisabled();
    expect(screen.getByText(/valid.*5-field/i)).toBeInTheDocument();
    expect(updateCronJob).not.toHaveBeenCalled();
  });

  it("blocks save and shows a required error for an empty schedule", async () => {
    const user = userEvent.setup();
    render(
      <CronDeploySection
        serviceId="nightly"
        schedule="0 6 * * *"
        command={null}
      />,
    );

    await user.click(screen.getByRole("button", { name: "Edit schedule" }));
    await user.clear(screen.getByRole("textbox", { name: "Schedule" }));

    expect(screen.getByRole("button", { name: "Save changes" })).toBeDisabled();
    expect(screen.getByText(/required/i)).toBeInTheDocument();
    expect(updateCronJob).not.toHaveBeenCalled();
  });

  it("cancel discards the schedule draft without calling updateCronJob", async () => {
    const user = userEvent.setup();
    render(
      <CronDeploySection
        serviceId="nightly"
        schedule="0 6 * * *"
        command={null}
      />,
    );

    await user.click(screen.getByRole("button", { name: "Edit schedule" }));
    const schedInput = screen.getByRole("textbox", { name: "Schedule" });
    await user.clear(schedInput);
    await user.type(schedInput, "0 0 * * *");
    await user.click(screen.getByRole("button", { name: "Cancel" }));

    expect(updateCronJob).not.toHaveBeenCalled();
    expect(screen.getByRole("textbox", { name: "Schedule" })).toHaveValue(
      "0 6 * * *",
    );
  });

  it("remains in edit mode when updateCronJob resolves false (server error)", async () => {
    updateCronJob.mockResolvedValue(false);
    const user = userEvent.setup();
    render(
      <CronDeploySection
        serviceId="nightly"
        schedule="0 6 * * *"
        command={null}
      />,
    );

    await user.click(screen.getByRole("button", { name: "Edit schedule" }));
    const schedInput = screen.getByRole("textbox", { name: "Schedule" });
    await user.clear(schedInput);
    await user.type(schedInput, "0 7 * * *");
    await user.click(screen.getByRole("button", { name: "Save changes" }));

    // Still editing — the Save button remains visible after the failed save.
    expect(
      screen.getByRole("button", { name: "Save changes" }),
    ).toBeInTheDocument();
  });
});
