import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import type { ReactNode } from "react";
import { DeployDetailPage } from "../deploy-detail-page";

const deployState = {
  deploy: {
    id: "dep-1",
    status: "live",
    trigger: "api",
    image: "registry.example.com/web:1",
    rollbackOf: "",
    commitId: "",
    commitMessage: "",
    createdAt: "2026-07-14T00:00:00Z",
    updatedAt: "2026-07-14T00:01:00Z",
    startedAt: null,
    finishedAt: "2026-07-14T00:01:00Z",
    preDeployStatus: "",
  },
  loading: false,
  error: undefined as Error | undefined,
  notFound: false,
};

vi.mock("../../hooks/use-deploy", () => ({
  useDeploy: () => deployState,
}));
vi.mock("../deploy-header", () => ({
  DeployHeader: ({ actions }: { actions?: ReactNode }) => (
    <div data-testid="deploy-header">{actions}</div>
  ),
}));
vi.mock("../deploy-actions", () => ({
  DeployActions: () => <div data-testid="deploy-actions" />,
}));
vi.mock("../deploy-timeline", () => ({
  DeployTimeline: () => <div data-testid="deploy-timeline" />,
}));
vi.mock("../deploy-log-panel", () => ({
  DeployLogPanel: () => <div data-testid="build-log" />,
}));

beforeEach(() => {
  deployState.deploy = {
    ...deployState.deploy,
    id: "dep-1",
    status: "live",
  };
  deployState.loading = false;
  deployState.error = undefined;
  deployState.notFound = false;
});

describe("DeployDetailPage", () => {
  it("renders the shared actions, timeline, and deploy log viewer", () => {
    render(<DeployDetailPage serviceId="web" deployId="dep-1" />);

    expect(screen.getByTestId("deploy-header")).toBeInTheDocument();
    expect(screen.getByTestId("deploy-actions")).toBeInTheDocument();
    expect(screen.getByTestId("deploy-timeline")).toBeInTheDocument();
    expect(screen.getByTestId("build-log")).toBeInTheDocument();
  });

  it("shows a not-found state for an unknown deploy id", () => {
    deployState.deploy = null as never;
    deployState.notFound = true;

    render(<DeployDetailPage serviceId="web" deployId="dep-missing" />);

    expect(screen.getByText("Deploy not found")).toBeInTheDocument();
  });

  it("shows a request error instead of leaving a permanent skeleton", () => {
    deployState.deploy = null as never;
    deployState.error = new Error("deploy query unavailable");

    render(<DeployDetailPage serviceId="web" deployId="dep-1" />);

    expect(screen.getByText("deploy query unavailable")).toBeInTheDocument();
  });
});
