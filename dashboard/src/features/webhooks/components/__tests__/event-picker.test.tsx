import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { EventPicker } from "@/features/webhooks/components/event-picker";
import { catalogEntries } from "@/features/webhooks/event-catalog";

const VOCAB = [
  "autoscaling_config_changed",
  "cron_job_run_ended",
  "cron_job_run_started",
  "deploy_ended",
  "deploy_started",
  "image_pull_failed",
  "commit_ignored",
  "instance_count_changed",
  "maintenance_mode_enabled",
  "maintenance_mode_uri_updated",
  "plan_changed",
  "postgres_backup_started",
  "postgres_created",
  "postgres_credentials_created",
  "postgres_credentials_deleted",
  "postgres_restarted",
  "server_restarted",
  "server_failed",
  "server_available",
  "autoscaling_started",
  "autoscaling_ended",
  "branch_changed",
  "service_resumed",
  "service_suspended",
];

function Harness({
  eventTypes = VOCAB,
  initial = [],
}: {
  eventTypes?: string[];
  initial?: string[];
}) {
  const [value, setValue] = useState<Set<string>>(() => new Set(initial));
  return (
    <EventPicker eventTypes={eventTypes} value={value} onChange={setValue} />
  );
}

describe("catalogEntries (w1/m49/t002)", () => {
  it("groups every served key and degrades unknown keys to 'other' with a null label", () => {
    const entries = catalogEntries([...VOCAB, "zz_mystery_event"]);
    const flat = entries.flatMap((e) => e.events);
    expect(flat.map((e) => e.type)).toHaveLength(VOCAB.length + 1);

    const mystery = flat.find((e) => e.type === "zz_mystery_event");
    expect(mystery?.labelKey).toBeNull();
    expect(entries[entries.length - 1].groupKey).toBe("other");
    expect(entries[entries.length - 1].events).toEqual([
      { type: "zz_mystery_event", labelKey: null },
    ]);
  });

  it("omits groups with no served events and keeps in-catalog order", () => {
    const entries = catalogEntries(["deploy_started", "service_resumed"]);
    expect(entries.map((e) => e.groupKey)).toEqual(["deploy", "suspension"]);
  });
});

describe("EventPicker", () => {
  it("renders human labels for known keys and raw text for unknown ones", () => {
    render(<Harness eventTypes={[...VOCAB, "zz_mystery_event"]} />);
    expect(screen.getByText("Deploy Started")).toBeInTheDocument();
    expect(screen.getByText("Plan Changed")).toBeInTheDocument();
    expect(screen.getByText("zz_mystery_event")).toBeInTheDocument();
    expect(screen.getByText("Other")).toBeInTheDocument();
  });

  it("group checkbox cascades to all children and the counter tracks", async () => {
    const user = userEvent.setup();
    render(<Harness />);
    expect(screen.getByTestId("event-count")).toHaveTextContent(
      "0 events selected",
    );

    await user.click(screen.getByRole("checkbox", { name: "Postgres" }));
    expect(screen.getByTestId("event-count")).toHaveTextContent(
      "5 events selected",
    );
    expect(
      screen.getByRole("checkbox", { name: "Postgres Created" }),
    ).toBeChecked();

    // Unchecking one child sends the group checkbox to mixed, not unchecked.
    await user.click(
      screen.getByRole("checkbox", { name: "Postgres Created" }),
    );
    expect(screen.getByTestId("event-count")).toHaveTextContent(
      "4 events selected",
    );
    expect(
      screen
        .getByRole("checkbox", { name: "Postgres" })
        .getAttribute("data-state"),
    ).toBe("indeterminate");

    // Clicking the mixed group selects the remainder (cascade to full).
    await user.click(screen.getByRole("checkbox", { name: "Postgres" }));
    expect(screen.getByTestId("event-count")).toHaveTextContent(
      "5 events selected",
    );
  });

  it("'All events' is tri-state: selects everything, shows mixed on partial, clears when full", async () => {
    const user = userEvent.setup();
    render(<Harness />);
    const all = screen.getByRole("checkbox", { name: "All events" });
    expect(all.getAttribute("data-state")).toBe("unchecked");

    await user.click(screen.getByRole("checkbox", { name: "Deploy Started" }));
    expect(all.getAttribute("data-state")).toBe("indeterminate");

    await user.click(all);
    expect(screen.getByTestId("event-count")).toHaveTextContent(
      `${VOCAB.length} events selected`,
    );
    expect(all.getAttribute("data-state")).toBe("checked");

    await user.click(all);
    expect(screen.getByTestId("event-count")).toHaveTextContent(
      "0 events selected",
    );
  });

  it("search narrows by label, raw key, or group name; clearing restores the tree", async () => {
    const user = userEvent.setup();
    render(<Harness />);
    const search = screen.getByRole("textbox", { name: "Search for events" });

    await user.type(search, "backup");
    expect(screen.getByText("Postgres Backup Started")).toBeInTheDocument();
    expect(screen.queryByText("Deploy Started")).not.toBeInTheDocument();

    await user.clear(search);
    await user.type(search, "cron_job_run"); // raw-key match
    expect(screen.getByText("Cron Job Run Started")).toBeInTheDocument();
    expect(screen.queryByText("Server Restarted")).not.toBeInTheDocument();

    await user.clear(search);
    await user.type(search, "Suspension"); // group-label match keeps the whole group
    expect(screen.getByText("Service Suspended")).toBeInTheDocument();
    expect(screen.getByText("Service Resumed")).toBeInTheDocument();

    await user.clear(search);
    expect(screen.getByText("Deploy Started")).toBeInTheDocument();
  });

  it("shows an empty state when the search matches nothing", async () => {
    const user = userEvent.setup();
    render(<Harness />);
    await user.type(
      screen.getByRole("textbox", { name: "Search for events" }),
      "nonexistent-xyz",
    );
    expect(screen.getByText("No events match your search")).toBeInTheDocument();
  });

  it("groups collapse and expand without losing selection", async () => {
    const user = userEvent.setup();
    render(<Harness initial={["deploy_started"]} />);
    await user.click(
      screen.getByRole("button", { name: "Toggle Deploy events" }),
    );
    expect(screen.queryByText("Deploy Started")).not.toBeInTheDocument();
    expect(screen.getByTestId("event-count")).toHaveTextContent(
      "1 events selected",
    );

    await user.click(
      screen.getByRole("button", { name: "Toggle Deploy events" }),
    );
    expect(
      screen.getByRole("checkbox", { name: "Deploy Started" }),
    ).toBeChecked();
  });

  it("disabled blocks every checkbox", async () => {
    const onChange = vi.fn();
    render(
      <EventPicker
        eventTypes={VOCAB}
        value={new Set()}
        onChange={onChange}
        disabled
      />,
    );
    const user = userEvent.setup();
    await user.click(screen.getByRole("checkbox", { name: "All events" }));
    await user.click(screen.getByRole("checkbox", { name: "Deploy Started" }));
    expect(onChange).not.toHaveBeenCalled();
  });
});
