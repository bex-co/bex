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
    expect(screen.getByText("manual deploy")).toBeInTheDocument();
  });

  it("labels a rollback deploy with the restored deploy's id, not the generic trigger label", () => {
    render(
      <DeployHeader
        deploy={deploy({ trigger: "rollback", rollbackOf: "dep-live-001" })}
      />,
    );
    expect(screen.getByText("rollback to dep-live-001")).toBeInTheDocument();
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
});
