import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { DeployHeader } from "@/features/deploys/components/deploy-header";
import type { DeployView } from "@/features/deploys/hooks/use-deploy";

function deploy(over: Partial<DeployView> = {}): DeployView {
  return {
    id: "dep-1",
    status: "live",
    trigger: "api",
    image: "registry.example.com/web:1",
    rollbackOf: "",
    commitId: "",
    commitMessage: "",
    commitCreatedAt: null,
    createdAt: "2026-07-14T00:00:00Z",
    updatedAt: "2026-07-14T00:01:00Z",
    startedAt: "2026-07-14T00:00:01Z",
    finishedAt: "2026-07-14T00:01:00Z",
    preDeployStatus: "",
    failureReason: "",
    cancelReason: "",
    stallReason: "",
    ...over,
  };
}

describe("DeployHeader", () => {
  it.each([
    ["live", "Live"],
    ["update_in_progress", "In Progress"],
    ["update_failed", "Failed"],
    ["canceled", "Canceled"],
    ["build_failed", "Build Failed"],
    ["pre_deploy_failed", "Pre-Deploy Failed"],
    ["deactivated", "Deactivated"],
  ])("renders the %s status as %j", (status, label) => {
    render(<DeployHeader deploy={deploy({ status })} />);
    expect(screen.getByText(label)).toBeInTheDocument();
  });

  it.each([
    ["running", "Pre-deploy command running"],
    ["succeeded", "Pre-deploy command succeeded"],
    ["failed", "Pre-deploy command failed"],
  ])("renders the %s pre-deploy outcome", (preDeployStatus, label) => {
    render(<DeployHeader deploy={deploy({ preDeployStatus })} />);
    expect(screen.getByText(label)).toBeInTheDocument();
  });

  it("shows no pre-deploy line when the deploy has no pre-deploy step", () => {
    render(<DeployHeader deploy={deploy({ preDeployStatus: "" })} />);
    expect(screen.queryByText(/Pre-deploy command/)).not.toBeInTheDocument();
  });

  it("labels a manual (api-triggered) deploy", () => {
    render(
      <DeployHeader deploy={deploy({ trigger: "api", rollbackOf: "" })} />,
    );
    expect(screen.getByText("Manual Deploy")).toBeInTheDocument();
  });

  it("labels a rollback deploy with the restored deploy's id intact (no CSS capitalize)", () => {
    render(
      <DeployHeader
        deploy={deploy({ trigger: "rollback", rollbackOf: "dep-live-001" })}
      />,
    );
    const label = screen.getByText("Rollback to dep-live-001");
    expect(label).toBeInTheDocument();
    expect(label).not.toHaveClass("capitalize");
  });

  it("renders the resolved commit as short SHA + the message's first line (w9/001)", () => {
    render(
      <DeployHeader
        deploy={deploy({
          commitId: "abc1234def5678",
          commitMessage: "fix: header\n\nlonger body that must not render",
        })}
      />,
    );
    expect(screen.getByText("abc1234")).toBeInTheDocument();
    expect(screen.getByText(/fix: header/)).toBeInTheDocument();
    expect(screen.queryByText(/longer body/)).not.toBeInTheDocument();
  });

  // Hash-only commits (message unavailable) must still render cleanly — the
  // public-repo resolve path can yield SHA before a subject is known.
  it("renders a hash-only commit without a dangling message spacer", () => {
    render(
      <DeployHeader
        deploy={deploy({
          commitId: "abc1234def5678",
          commitMessage: "",
        })}
      />,
    );
    const line = screen.getByText("abc1234").closest("p");
    expect(line?.textContent?.trim()).toBe("abc1234");
  });

  // w6/m45 t004: the date used to be separated from the commit message by a
  // margin alone, so the paragraph was one unbroken text run — a screen reader
  // (and anyone copying the line) got "…Docker build contextAugust 22, 2026".
  it("separates the commit message from the commit date with real text", () => {
    render(
      <DeployHeader
        deploy={deploy({
          commitId: "abc1234def5678",
          commitMessage: "fix: Docker build context",
          commitCreatedAt: "2026-07-14T00:00:00Z",
        })}
      />,
    );
    const line = screen.getByText("abc1234").closest("p");
    expect(line?.textContent).toMatch(/Docker build context\s+·\s+\S/);
  });

  it("shows no commit line when the commit was never resolved", () => {
    render(<DeployHeader deploy={deploy({ commitId: "" })} />);
    expect(screen.queryByText("abc1234")).not.toBeInTheDocument();
  });

  it("renders the deploy's image", () => {
    render(
      <DeployHeader
        deploy={deploy({ image: "registry.example.com/web:abc123" })}
      />,
    );
    expect(
      screen.getByText("registry.example.com/web:abc123"),
    ).toBeInTheDocument();
  });

  it("shows duration through the shared formatter", () => {
    render(<DeployHeader deploy={deploy()} />);

    expect(screen.getByText("Duration:")).toBeInTheDocument();
    expect(screen.getByText("59s")).toBeInTheDocument();
  });

  it("omits timestamps and duration that haven't happened instead of inventing values", () => {
    render(
      <DeployHeader
        deploy={deploy({
          status: "update_in_progress",
          createdAt: null,
          updatedAt: null,
          startedAt: null,
          finishedAt: null,
        })}
      />,
    );

    expect(screen.queryByText("Created:")).not.toBeInTheDocument();
    expect(screen.queryByText("Duration:")).not.toBeInTheDocument();
    expect(screen.queryByText("—")).not.toBeInTheDocument();
  });

  // w4/m112: a rollout gated on a failing health check used to show a bare
  // "In Progress" for the full 900s budget — the page named no probe, and a
  // user could not tell it from a slow image pull. Live on 2026-09-17 with
  // Health Check Path /qa-bogus-health.
  it("names what an in-progress rollout is waiting on", () => {
    const stall =
      "the container is running but its readiness health check has not " +
      "succeeded, so the rollout is waiting: GET /qa-bogus-health on port 3000.";
    render(
      <DeployHeader
        deploy={deploy({ status: "update_in_progress", stallReason: stall })}
      />,
    );

    expect(screen.getByText(stall)).toBeInTheDocument();
    expect(screen.getByText("In Progress")).toBeInTheDocument();
  });

  // Before the diagnosis arrives (~2 min) the page reads exactly as it did.
  it("shows nothing extra while a rollout is progressing normally", () => {
    const { container } = render(
      <DeployHeader deploy={deploy({ status: "update_in_progress" })} />,
    );
    expect(container.textContent).not.toContain("health check");
  });

  // The server clears stall_reason as the row goes terminal, so the terminal
  // states keep their own copy. Assert the component does not resurrect it.
  it("keeps the failure copy for a terminal deploy", () => {
    render(
      <DeployHeader
        deploy={deploy({
          status: "update_failed",
          failureReason: "the deploy did not become healthy within the health-gate window",
          stallReason: "",
        })}
      />,
    );
    expect(
      screen.getByText(
        "the deploy did not become healthy within the health-gate window",
      ),
    ).toBeInTheDocument();
  });
});
