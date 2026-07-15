import type { DeployView } from "@/features/deploys/hooks/use-deploy";
import type { DeployTimelineEvent } from "@/features/deploys/hooks/use-deploy-timeline";
import { isTerminalDeployStatus } from "./deploy-status";

export type DeployTimelineStepKind =
  | "created"
  | "started"
  | "in_progress"
  | "live"
  | "failed"
  | "canceled"
  | "deactivated";

export interface DeployTimelineStep {
  id: string;
  kind: DeployTimelineStepKind;
  timestamp: string | null;
  /** Present for current/terminal steps so the UI can name the exact status. */
  status?: string;
}

/**
 * Builds an honest sequence from facts bex actually stores. It never invents
 * build/pre-deploy phases: createdAt and startedAt come from the deploy row;
 * the matching deploy_ended event supplies the terminal timestamp when present,
 * with finishedAt as the row-backed fallback. A non-terminal current status is
 * shown without a timestamp because bex does not record when that state began.
 */
export function buildDeployTimeline(
  deploy: DeployView,
  events: DeployTimelineEvent[],
): DeployTimelineStep[] {
  const startedEvent = events.find((event) => event.type === "deploy_started");
  const endedEvent = events.find((event) => event.type === "deploy_ended");
  const createdAt = startedEvent?.timestamp ?? deploy.createdAt;

  const steps: DeployTimelineStep[] = [
    { id: "created", kind: "created", timestamp: createdAt },
  ];

  if (deploy.startedAt && deploy.startedAt !== createdAt) {
    steps.push({ id: "started", kind: "started", timestamp: deploy.startedAt });
  }

  if (!isTerminalDeployStatus(deploy.status)) {
    steps.push({
      id: "current",
      kind: "in_progress",
      timestamp: null,
      status: deploy.status,
    });
    return steps;
  }

  const kind = terminalKind(deploy.status);
  steps.push({
    id: "terminal",
    kind,
    timestamp: endedEvent?.timestamp ?? deploy.finishedAt,
    status: deploy.status,
  });
  return steps;
}

function terminalKind(status: string): DeployTimelineStepKind {
  switch (status) {
    case "live":
      return "live";
    case "canceled":
      return "canceled";
    case "deactivated":
      return "deactivated";
    default:
      return "failed";
  }
}
