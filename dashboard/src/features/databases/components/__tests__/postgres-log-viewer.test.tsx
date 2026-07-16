import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import type { UsePostgresLogsResult } from "../../hooks/use-postgres-logs";
import { PostgresLogViewer } from "../postgres-log-viewer";

const state: UsePostgresLogsResult = {
  lines: [],
  loading: false,
  error: undefined,
  unavailable: false,
  unauthorized: false,
};

vi.mock("../../hooks/use-postgres-logs", () => ({
  usePostgresLogs: () => state,
}));
vi.mock("@/features/logs/hooks/use-log-label-values", () => ({
  useLogLabelValues: () => ["dpg-example-1"],
}));

beforeEach(() => {
  state.lines = [];
  state.loading = false;
  state.error = undefined;
  state.unavailable = false;
  state.unauthorized = false;
});

describe("PostgresLogViewer", () => {
  it("renders timestamped, instance-attributed database lines", () => {
    state.lines = [
      {
        key: "line-1",
        timestamp: "2026-07-15T12:00:00Z",
        time: "12:00:00",
        instance: "dpg-example-1",
        message: "checkpoint complete",
        type: "postgres",
        level: "",
        method: "",
        statusCode: "",
      },
    ];
    render(<PostgresLogViewer resource="dpg-example" />);

    expect(screen.getByText("checkpoint complete")).toBeInTheDocument();
    expect(screen.getByText("[dpg-example-1]")).toBeInTheDocument();
    expect(screen.getByText("12:00:00")).toBeInTheDocument();
  });

  it("distinguishes empty, unavailable, unauthorized, and generic errors", () => {
    const { rerender } = render(<PostgresLogViewer resource="dpg-example" />);
    expect(screen.getByText("No database logs yet")).toBeInTheDocument();

    state.unavailable = true;
    rerender(<PostgresLogViewer resource="dpg-example" />);
    expect(
      screen.getByText("Database logs aren't configured"),
    ).toBeInTheDocument();

    state.unavailable = false;
    state.unauthorized = true;
    rerender(<PostgresLogViewer resource="dpg-example" />);
    expect(screen.getByText("You can't view these logs")).toBeInTheDocument();

    state.unauthorized = false;
    state.error = new Error("Loki is unavailable");
    rerender(<PostgresLogViewer resource="dpg-example" />);
    expect(screen.getByText("Couldn't load database logs")).toBeInTheDocument();
    expect(screen.getByText("Loki is unavailable")).toBeInTheDocument();
  });
});
