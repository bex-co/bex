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
 * build/pre-deploy phases: createdAt, startedAt, updatedAt, and finishedAt come
 * from the deploy row. updatedAt timestamps the current observed state; a
 * deactivated deploy retains its original live finishedAt and adds the later
 * deactivation transition.
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
      timestamp: deploy.updatedAt,
      status: deploy.status,
    });
    return steps;
  }

  if (deploy.status === "deactivated" && deploy.finishedAt) {
    steps.push({
      id: "live",
      kind: "live",
      timestamp: deploy.finishedAt,
      status: "live",
    });
  }

  const kind = terminalKind(deploy.status);
  steps.push({
    id: "terminal",
    kind,
    timestamp:
      deploy.status === "deactivated"
        ? deploy.updatedAt
        : (endedEvent?.timestamp ?? deploy.finishedAt ?? deploy.updatedAt),
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
