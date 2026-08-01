import { useState } from "react";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { ServiceEventFilter } from "@/features/events/components/service-event-filter";
import { SERVICE_EVENT_TYPES } from "@/features/events/service-event-catalog";

function Harness() {
  const [selected, setSelected] = useState(() => new Set(SERVICE_EVENT_TYPES));
  return <ServiceEventFilter value={selected} onChange={setSelected} />;
}

describe("ServiceEventFilter", () => {
  it("exposes the truthful Render-shaped groups and filters them by search", async () => {
    const user = userEvent.setup();
    render(<Harness />);
    await user.click(screen.getByRole("button", { name: "Filter events" }));

    expect(screen.getByRole("checkbox", { name: "Deploy" })).toBeChecked();
    expect(
      screen.getByRole("checkbox", { name: "Service Status" }),
    ).toBeChecked();
    expect(screen.getByRole("checkbox", { name: "Scaling" })).toBeChecked();
    expect(
      screen.getByRole("checkbox", { name: "Maintenance Mode" }),
    ).toBeChecked();
    expect(
      screen.queryByRole("checkbox", { name: "Pipeline Minutes Exhausted" }),
    ).not.toBeInTheDocument();

    await user.type(
      screen.getByRole("textbox", { name: "Search events" }),
      "commit ignored",
    );
    expect(
      screen.getByRole("checkbox", { name: "Commit ignored" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("checkbox", { name: "Image pull failed" }),
    ).not.toBeInTheDocument();
  });

  it("supports group selection and reports the narrowed selected count", async () => {
    const user = userEvent.setup();
    render(<Harness />);
    await user.click(screen.getByRole("button", { name: "Filter events" }));
    await user.click(screen.getByRole("checkbox", { name: "All events" }));
    await user.click(screen.getByRole("checkbox", { name: "Deploy" }));

    // The Deploy group carries the full deploy lifecycle (w7/m66 added
    // build/pre-deploy/job-run-ended): 12 types.
    expect(
      screen.getByRole("button", { name: /Filter events \(12\)/ }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("checkbox", { name: "Deploy started" }),
    ).toBeChecked();
    expect(screen.getByRole("checkbox", { name: "Resumed" })).not.toBeChecked();
  });
});
