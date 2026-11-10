import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { LogEmptyState } from "../log-empty-state";

// w21/025: a first-load failure replaces the pane, so it is announced; an
// empty result is not an error and stays silent.
describe("LogEmptyState roles", () => {
  it("announces a failed load and a timed-out search", () => {
    const { rerender } = render(
      <LogEmptyState
        reason="failed"
        title="Couldn't load logs"
        error={new Error("boom")}
      />,
    );
    expect(screen.getByRole("alert")).toHaveTextContent("Couldn't load logs");
    rerender(<LogEmptyState reason="timeout" />);
    expect(screen.getByRole("alert")).toBeInTheDocument();
  });

  it("keeps an empty result silent", () => {
    render(
      <LogEmptyState
        reason="empty"
        filtered={false}
        emptyTitle="No logs"
        emptyBody="Nothing yet"
      />,
    );
    expect(screen.queryByRole("alert")).toBeNull();
  });
});
