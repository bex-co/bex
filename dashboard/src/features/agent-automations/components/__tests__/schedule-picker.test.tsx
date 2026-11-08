import { useState } from "react";
import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { SchedulePicker } from "@/features/agent-automations/components/schedule-picker";
import {
  defaultScheduleDraft,
  draftToScheduleInput,
  type ScheduleDraft,
} from "@/features/agent-automations/lib/schedule";

let latest: ScheduleDraft;

function Harness({
  initial = defaultScheduleDraft("Europe/Berlin"),
  problems = {},
  serverErrors = {},
}: {
  initial?: ScheduleDraft;
  problems?: React.ComponentProps<typeof SchedulePicker>["problems"];
  serverErrors?: React.ComponentProps<typeof SchedulePicker>["serverErrors"];
}) {
  const [value, setValue] = useState(initial);
  latest = value;
  return (
    <SchedulePicker
      value={value}
      onChange={setValue}
      problems={problems}
      serverErrors={serverErrors}
    />
  );
}

async function chooseFrequency(
  user: ReturnType<typeof userEvent.setup>,
  name: string,
) {
  await user.click(screen.getByRole("combobox", { name: "Repeat" }));
  await user.click(await screen.findByRole("option", { name }));
}

describe("SchedulePicker", () => {
  it("opens on a daily schedule in the given zone with a live summary", () => {
    render(<Harness />);
    expect(screen.getByLabelText("Time")).toHaveValue("09:00");
    expect(
      screen.getByRole("combobox", { name: "Time zone" }),
    ).toHaveTextContent("Europe/Berlin");
    expect(screen.getByTestId("schedule-summary")).toHaveTextContent(
      "Runs: Every day at 09:00 · Europe/Berlin",
    );
    expect(screen.queryByLabelText("Cron expression")).not.toBeInTheDocument();
  });

  it("each frequency reveals only its own inputs and produces the matching input object", async () => {
    const user = userEvent.setup();
    render(<Harness />);

    await chooseFrequency(user, "Every hour");
    expect(screen.getByLabelText("Minute past the hour")).toBeInTheDocument();
    expect(screen.queryByLabelText("Time")).not.toBeInTheDocument();
    await user.clear(screen.getByLabelText("Minute past the hour"));
    await user.type(screen.getByLabelText("Minute past the hour"), "15");
    expect(draftToScheduleInput(latest)).toEqual({
      frequency: "hour",
      minute: 15,
      timezone: "Europe/Berlin",
    });

    await chooseFrequency(user, "Every week");
    expect(screen.getByRole("group", { name: "Days" })).toBeInTheDocument();
    const mon = screen.getByRole("button", { name: "Mon" });
    expect(mon).toHaveAttribute("aria-pressed", "true");
    await user.click(mon);
    expect(mon).toHaveAttribute("aria-pressed", "false");
    expect(draftToScheduleInput(latest)).toMatchObject({
      frequency: "week",
      at: "09:00",
      weekdays: ["tue", "wed", "thu", "fri"],
    });

    await chooseFrequency(user, "Custom (cron)");
    await user.type(screen.getByLabelText("Cron expression"), "0 9 * * 1-5");
    expect(draftToScheduleInput(latest)).toEqual({
      frequency: "custom",
      cron: "0 9 * * 1-5",
      timezone: "Europe/Berlin",
    });
    expect(screen.getByTestId("schedule-summary")).toHaveTextContent(
      "Cron 0 9 * * 1-5 · Europe/Berlin",
    );

    await chooseFrequency(user, "Once");
    expect(screen.queryByLabelText(/Ends on/)).not.toBeInTheDocument();
    expect(screen.getByLabelText("Date and time")).toBeInTheDocument();
  });

  it("shows client problems and server messages next to their fields, described and announced", () => {
    render(
      <Harness
        initial={{ ...defaultScheduleDraft("Europe/Berlin"), at: "" }}
        problems={{ at: "agentAutomations.problem.at" }}
        serverErrors={{ timezone: "Timezone is not a known IANA zone" }}
      />,
    );
    const time = screen.getByLabelText("Time");
    expect(time).toHaveAttribute("aria-invalid", "true");
    expect(time).toHaveAccessibleDescription("Enter a time.");
    expect(screen.getAllByRole("alert").map((a) => a.textContent)).toEqual([
      "Enter a time.",
      "Timezone is not a known IANA zone",
    ]);
    expect(screen.getByTestId("schedule-summary")).toBeEmptyDOMElement();
  });

  it("calls onChange with the typed end date", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(
      <SchedulePicker
        value={defaultScheduleDraft("UTC")}
        onChange={onChange}
        problems={{}}
        serverErrors={{}}
      />,
    );
    await user.type(screen.getByLabelText(/Ends on/), "2030-01-31");
    expect(onChange).toHaveBeenLastCalledWith(
      expect.objectContaining({ endsOn: "2030-01-31" }),
    );
  });
});
