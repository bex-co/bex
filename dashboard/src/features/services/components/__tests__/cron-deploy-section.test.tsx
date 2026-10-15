import { describe, it, expect, vi, beforeEach } from "vitest";
import { act, render, screen } from "@testing-library/react";
import i18n from "@/i18n/init";
import userEvent from "@testing-library/user-event";
import {
  CronDeploySection,
  type CronDeploySectionProps,
} from "@/features/services/components/cron-deploy-section";

const updateCronJob = vi.fn(async () => true);
const setStartCommand = vi.fn(async () => true);

vi.mock("@/features/services/hooks/use-cron-job", () => ({
  useCronJob: () => ({ updateCronJob, setStartCommand, busy: false }),
}));

beforeEach(() => {
  updateCronJob.mockReset();
  updateCronJob.mockResolvedValue(true);
  setStartCommand.mockReset();
  setStartCommand.mockResolvedValue(true);
});

/** A container cron unless `runtime` names a native one. */
function renderCron(props: Partial<CronDeploySectionProps> = {}) {
  render(
    <CronDeploySection
      serviceId="nightly"
      schedule="0 6 * * *"
      command={null}
      runtime={null}
      startCommand={null}
      {...props}
    />,
  );
}

describe("CronDeploySection", () => {
  it("shows the current schedule and command in disabled inputs", () => {
    renderCron({ schedule: "0 6 * * *", command: "node daily.js" });
    const sched = screen.getByRole("textbox", { name: "Schedule" });
    const cmd = screen.getByRole("textbox", { name: "Command" });
    expect(sched).toHaveValue("0 6 * * *");
    expect(sched).toBeDisabled();
    expect(cmd).toHaveValue("node daily.js");
  });

  it("shows an empty command input when command is not set", () => {
    renderCron({ schedule: "0 6 * * *", command: null });
    expect(screen.getByRole("textbox", { name: "Command" })).toHaveValue("");
  });

  it("edits the schedule and saves both fields via updateCronJob", async () => {
    const user = userEvent.setup();
    renderCron({ schedule: "0 6 * * *", command: "node old.js" });

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
    renderCron({ schedule: "0 6 * * *", command: "node old.js" });

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
    renderCron({ schedule: "0 6 * * *", command: null });

    await user.click(screen.getByRole("button", { name: "Edit schedule" }));
    const schedInput = screen.getByRole("textbox", { name: "Schedule" });
    await user.clear(schedInput);
    await user.type(schedInput, "not valid");

    expect(screen.getByRole("button", { name: "Save changes" })).toBeDisabled();
    expect(screen.getByText(/valid.*5-field/i)).toBeInTheDocument();
    expect(updateCronJob).not.toHaveBeenCalled();
  });

  it("blocks save for a 5-field schedule with out-of-range values (99 99 * * *)", async () => {
    const user = userEvent.setup();
    renderCron({ schedule: "0 6 * * *", command: null });

    await user.click(screen.getByRole("button", { name: "Edit schedule" }));
    const schedInput = screen.getByRole("textbox", { name: "Schedule" });
    await user.clear(schedInput);
    await user.type(schedInput, "99 99 * * *");

    expect(screen.getByRole("button", { name: "Save changes" })).toBeDisabled();
    expect(screen.getByText(/valid.*5-field/i)).toBeInTheDocument();
    expect(updateCronJob).not.toHaveBeenCalled();
  });

  // w5/070: changing "*/30" to "*/15" by deleting the 3 first passes through
  // "*/0 * * * *". Classifying that draft used to expand a zero step forever,
  // freezing the tab; it is a format error like any other unparseable draft.
  it("shows the format error for a zero step instead of hanging", async () => {
    const user = userEvent.setup();
    renderCron({ schedule: "*/30 * * * *", command: null });

    await user.click(screen.getByRole("button", { name: "Edit schedule" }));
    const schedInput = screen.getByRole("textbox", { name: "Schedule" });
    await user.type(schedInput, "{Backspace}", {
      initialSelectionStart: 3,
      initialSelectionEnd: 3,
    });

    expect(schedInput).toHaveValue("*/0 * * * *");
    expect(screen.getByRole("button", { name: "Save changes" })).toBeDisabled();
    expect(screen.getByText(/valid.*5-field/i)).toBeInTheDocument();
    expect(updateCronJob).not.toHaveBeenCalled();
  });

  it("names a schedule that parses but never fires, and an out-of-range day as a format error", async () => {
    const user = userEvent.setup();
    renderCron({ schedule: "0 6 * * *", command: null });

    await user.click(screen.getByRole("button", { name: "Edit schedule" }));
    const schedInput = screen.getByRole("textbox", { name: "Schedule" });
    await user.clear(schedInput);
    await user.type(schedInput, "0 0 31 2 *");
    expect(screen.getByText(/never fires/i)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Save changes" })).toBeDisabled();

    await user.clear(schedInput);
    await user.type(schedInput, "0 0 99 2 *");
    expect(screen.queryByText(/never fires/i)).not.toBeInTheDocument();
    expect(screen.getByText(/valid.*5-field/i)).toBeInTheDocument();
  });

  it("shows a human-readable preview of the current schedule", () => {
    renderCron({ schedule: "0 6 * * *", command: null });
    expect(
      screen.getByText(/Every day at 06:00 · runs in UTC/i),
    ).toBeInTheDocument();
  });

  // w4/m188: the preview interpolated an English phrase into the translated
  // "{description} · 使用 UTC" wrapper; the draft preview now follows the
  // active language and typing it never saves.
  it("localizes the persisted and draft schedule previews", async () => {
    const user = userEvent.setup();
    renderCron({ schedule: "0 0 1 1 *", command: null });
    await user.click(screen.getByRole("button", { name: "Edit schedule" }));
    const schedInput = screen.getByRole("textbox", { name: "Schedule" });
    try {
      await act(() => i18n.changeLanguage("zh"));
      await user.clear(schedInput);
      await user.type(schedInput, "*/5 * * * *");
      expect(screen.getByText("每 5 分钟 · 使用 UTC")).toBeInTheDocument();
      await user.clear(schedInput);
      await user.type(schedInput, "0 0 * * MON");
      expect(screen.getByText("每周一 00:00 · 使用 UTC")).toBeInTheDocument();
      // Non-uniform step: no preview at all, and still saveable.
      await user.clear(schedInput);
      await user.type(schedInput, "*/40 * * * *");
      expect(screen.queryByText(/· 使用 UTC/)).not.toBeInTheDocument();
      expect(updateCronJob).not.toHaveBeenCalled();
    } finally {
      await act(() => i18n.changeLanguage("en"));
    }
  });

  it("blocks save and shows a required error for an empty schedule", async () => {
    const user = userEvent.setup();
    renderCron({ schedule: "0 6 * * *", command: null });

    await user.click(screen.getByRole("button", { name: "Edit schedule" }));
    await user.clear(screen.getByRole("textbox", { name: "Schedule" }));

    expect(screen.getByRole("button", { name: "Save changes" })).toBeDisabled();
    expect(screen.getByText(/required/i)).toBeInTheDocument();
    expect(updateCronJob).not.toHaveBeenCalled();
  });

  it("cancel discards the schedule draft without calling updateCronJob", async () => {
    const user = userEvent.setup();
    renderCron({ schedule: "0 6 * * *", command: null });

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
    renderCron({ schedule: "0 6 * * *", command: null });

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

// w4/137: emptying Command reported "Cron job settings saved." while the old
// command stayed persisted, because the hook converted the explicit empty into
// null — which the backend reads as "keep the existing command". These assert
// the three distinct intents the Command field can express, at the boundary
// where the section decides what to send.
describe("CronDeploySection command clearing (w4/137)", () => {
  it("sends an explicit empty command when the user clears the field", async () => {
    const user = userEvent.setup();
    renderCron({ schedule: "0 6 * * *", command: "sh -c 'echo qa'" });

    await user.click(screen.getByRole("button", { name: "Edit command" }));
    await user.clear(screen.getByRole("textbox", { name: "Command" }));
    await user.click(screen.getByRole("button", { name: "Save changes" }));

    expect(updateCronJob).toHaveBeenCalledWith("nightly", "0 6 * * *", "");
  });

  it("treats a whitespace-only command as a clear, like an empty one", async () => {
    const user = userEvent.setup();
    renderCron({ schedule: "0 6 * * *", command: "sh -c 'echo qa'" });

    await user.click(screen.getByRole("button", { name: "Edit command" }));
    const input = screen.getByRole("textbox", { name: "Command" });
    await user.clear(input);
    await user.type(input, "   ");
    await user.click(screen.getByRole("button", { name: "Save changes" }));

    expect(updateCronJob).toHaveBeenCalledWith("nightly", "0 6 * * *", "");
  });

  it('sends null — not "" — when rescheduling a commandless cron', async () => {
    const user = userEvent.setup();
    renderCron({ schedule: "0 6 * * *", command: null });

    await user.click(screen.getByRole("button", { name: "Edit schedule" }));
    const schedInput = screen.getByRole("textbox", { name: "Schedule" });
    await user.clear(schedInput);
    await user.type(schedInput, "0 8 * * 1");
    await user.click(screen.getByRole("button", { name: "Save changes" }));

    // null means keep. Sending "" here would turn every reschedule of a
    // commandless cron into an explicit command write, which selects a
    // different authorization path on the server.
    expect(updateCronJob).toHaveBeenCalledWith("nightly", "0 8 * * 1", null);
  });

  it("still replays a stored command when rescheduling a command-bearing cron", async () => {
    const user = userEvent.setup();
    renderCron({ schedule: "0 6 * * *", command: "  node daily.js  " });

    await user.click(screen.getByRole("button", { name: "Edit schedule" }));
    const schedInput = screen.getByRole("textbox", { name: "Schedule" });
    await user.clear(schedInput);
    await user.type(schedInput, "0 8 * * 1");
    await user.click(screen.getByRole("button", { name: "Save changes" }));

    // Byte for byte: a reschedule never rewrites the stored command (w8/m68).
    expect(updateCronJob).toHaveBeenCalledWith(
      "nightly",
      "0 8 * * 1",
      "  node daily.js  ",
    );
  });

  it("keeps the blank draft open when the clearing save fails", async () => {
    updateCronJob.mockResolvedValue(false);
    const user = userEvent.setup();
    renderCron({ schedule: "0 6 * * *", command: "sh -c 'echo qa'" });

    await user.click(screen.getByRole("button", { name: "Edit command" }));
    await user.clear(screen.getByRole("textbox", { name: "Command" }));
    await user.click(screen.getByRole("button", { name: "Save changes" }));

    // Still editing, still blank: the clear intent survives for a retry.
    expect(screen.getByRole("textbox", { name: "Command" })).toHaveValue("");
    expect(
      screen.getByRole("button", { name: "Save changes" }),
    ).toBeInTheDocument();
  });
});

describe("CronDeploySection on a native-runtime cron (w8/m69)", () => {
  const BASH = "[[ 1 = 1 ]] && printf ok";

  it("shows the start command when no override is set", () => {
    renderCron({
      schedule: "0 6 * * *",
      command: null,
      runtime: "node",
      startCommand: BASH,
    });
    expect(screen.getByRole("textbox", { name: "Command" })).toHaveValue(BASH);
    expect(screen.queryByText("Optional")).not.toBeInTheDocument();
    expect(
      screen.getByText(
        "The command the job runs on each schedule, as its runtime's start command.",
      ),
    ).toBeInTheDocument();
  });

  it("shows the override when one is set, since it runs instead", () => {
    renderCron({
      schedule: "0 6 * * *",
      command: "node override.js",
      runtime: "python",
      startCommand: "python job.py",
    });
    expect(screen.getByRole("textbox", { name: "Command" })).toHaveValue(
      "node override.js",
    );
  });

  it("saves a Bash command as the start command, byte for byte", async () => {
    const user = userEvent.setup();
    renderCron({
      schedule: "0 6 * * *",
      command: "node override.js",
      runtime: "node",
      startCommand: "npm start",
    });

    await user.click(screen.getByRole("button", { name: "Edit command" }));
    const input = screen.getByRole("textbox", { name: "Command" });
    await user.clear(input);
    await user.paste(`${BASH} `);
    await user.click(screen.getByRole("button", { name: "Save changes" }));

    expect(setStartCommand).toHaveBeenCalledWith("nightly", `${BASH} `);
    expect(updateCronJob).not.toHaveBeenCalled();
  });

  it("moves an unchanged override into the start command", async () => {
    const user = userEvent.setup();
    renderCron({
      schedule: "0 6 * * *",
      command: BASH,
      runtime: "ruby",
      startCommand: "bundle exec rake job",
    });

    await user.click(screen.getByRole("button", { name: "Edit command" }));
    await user.click(screen.getByRole("button", { name: "Save changes" }));

    expect(setStartCommand).toHaveBeenCalledWith("nightly", BASH);
    expect(updateCronJob).not.toHaveBeenCalled();
  });

  it("offers no save for an untouched start command", async () => {
    const user = userEvent.setup();
    renderCron({
      schedule: "0 6 * * *",
      command: null,
      runtime: "go",
      startCommand: "./app",
    });

    await user.click(screen.getByRole("button", { name: "Edit command" }));
    expect(screen.getByRole("button", { name: "Save changes" })).toBeDisabled();
  });

  it.each([
    ["an empty", ""],
    ["a whitespace-only", " \t "],
  ])("blocks %s draft with a required error", async (_label, draft) => {
    const user = userEvent.setup();
    renderCron({
      schedule: "0 6 * * *",
      command: "node override.js",
      runtime: "node",
      startCommand: "npm start",
    });

    await user.click(screen.getByRole("button", { name: "Edit command" }));
    const input = screen.getByRole("textbox", { name: "Command" });
    await user.clear(input);
    if (draft) await user.paste(draft);
    await user.keyboard("{Enter}");

    expect(input).toHaveAttribute("aria-invalid", "true");
    expect(screen.getByText("Command is required.")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Save changes" })).toBeDisabled();
    expect(setStartCommand).not.toHaveBeenCalled();
    expect(updateCronJob).not.toHaveBeenCalled();
  });

  it("reschedules with command null, never replaying the start command", async () => {
    const user = userEvent.setup();
    renderCron({
      schedule: "0 6 * * *",
      command: null,
      runtime: "node",
      startCommand: BASH,
    });

    await user.click(screen.getByRole("button", { name: "Edit schedule" }));
    const schedInput = screen.getByRole("textbox", { name: "Schedule" });
    await user.clear(schedInput);
    await user.type(schedInput, "0 8 * * 1");
    await user.click(screen.getByRole("button", { name: "Save changes" }));

    expect(updateCronJob).toHaveBeenCalledWith("nightly", "0 8 * * 1", null);
    expect(setStartCommand).not.toHaveBeenCalled();
  });

  it("reschedules replaying an override that is set", async () => {
    const user = userEvent.setup();
    renderCron({
      schedule: "0 6 * * *",
      command: "node override.js",
      runtime: "node",
      startCommand: "npm start",
    });

    await user.click(screen.getByRole("button", { name: "Edit schedule" }));
    const schedInput = screen.getByRole("textbox", { name: "Schedule" });
    await user.clear(schedInput);
    await user.type(schedInput, "0 8 * * 1");
    await user.click(screen.getByRole("button", { name: "Save changes" }));

    expect(updateCronJob).toHaveBeenCalledWith(
      "nightly",
      "0 8 * * 1",
      "node override.js",
    );
  });
});

describe("CronDeploySection on a container cron (w8/m69)", () => {
  it.each(["docker", "image", null])(
    "runtime %s shows and saves the command, not the start command",
    async (runtime) => {
      const user = userEvent.setup();
      renderCron({
        schedule: "0 6 * * *",
        command: null,
        runtime,
        startCommand: "bin/server",
      });

      const input = screen.getByRole("textbox", { name: "Command" });
      expect(input).toHaveValue("");
      expect(screen.getByText("Optional")).toBeInTheDocument();

      await user.click(screen.getByRole("button", { name: "Edit command" }));
      await user.paste("node job.js");
      await user.click(screen.getByRole("button", { name: "Save changes" }));

      expect(updateCronJob).toHaveBeenCalledWith(
        "nightly",
        "0 6 * * *",
        "node job.js",
      );
      expect(setStartCommand).not.toHaveBeenCalled();
    },
  );
});
