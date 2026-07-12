import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { ServiceTypeBadge } from "@/features/services/components/service-type-badge";
import type { ServiceView } from "@/features/services/types";

function svc(type: string): ServiceView {
  return {
    id: "a",
    name: "a",
    type,
    suspended: false,
    phase: "Running",
    url: null,
    createdAt: null,
    replicas: 1,
    revision: null,
    plan: null,
    idleTTLSeconds: null,
    schedule: null,
    runs: [],
  };
}

describe("ServiceTypeBadge", () => {
  it("labels a cron job", () => {
    render(<ServiceTypeBadge service={svc("cron_job")} />);
    expect(screen.getByText("Cron Job")).toBeInTheDocument();
  });

  it("labels a background worker", () => {
    render(<ServiceTypeBadge service={svc("background_worker")} />);
    expect(screen.getByText("Background Worker")).toBeInTheDocument();
  });

  it("labels a web service", () => {
    render(<ServiceTypeBadge service={svc("web_service")} />);
    expect(screen.getByText("Web Service")).toBeInTheDocument();
  });

  it("labels a static site", () => {
    render(<ServiceTypeBadge service={svc("static_site")} />);
    expect(screen.getByText("Static Site")).toBeInTheDocument();
  });

  it("falls back to the neutral label for an unknown/legacy type", () => {
    render(<ServiceTypeBadge service={svc("some_future_type")} />);
    expect(screen.getByText("Service")).toBeInTheDocument();
  });
});
