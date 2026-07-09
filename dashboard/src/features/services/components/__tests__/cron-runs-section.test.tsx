import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { CronRunsSection } from "@/features/services/components/cron-runs-section";
import type { CronRunView, ServiceView } from "@/features/services/types";

function cron(runs: CronRunView[]): ServiceView {
  return {
    id: "nightly",
    name: "nightly",
    type: "cron_job",
    suspended: false,
    phase: "Running",
    url: null,
    createdAt: null,
    replicas: null,
    revision: null,
    plan: null,
    idleTTLSeconds: null,
    schedule: "*/5 * * * *",
    runs,
  };
}

describe("CronRunsSection", () => {
  it("shows an empty state when the cron has no runs", () => {
    render(<CronRunsSection service={cron([])} />);
    expect(screen.getByText("No runs yet.")).toBeInTheDocument();
  });

  it("lists recent runs with their outcome", () => {
    render(
      <CronRunsSection
        service={cron([
          {
            name: "nightly-run-1",
            startedAt: "2026-07-09T10:00:00Z",
            finishedAt: "2026-07-09T10:00:05Z",
            status: "Succeeded",
          },
          {
            name: "nightly-run-2",
            startedAt: "2026-07-09T10:05:00Z",
            finishedAt: null,
            status: "Failed",
          },
        ])}
      />,
    );
    expect(screen.getByText("Succeeded")).toBeInTheDocument();
    expect(screen.getByText("Failed")).toBeInTheDocument();
  });
});
