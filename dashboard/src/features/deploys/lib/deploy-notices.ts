import type { DeployView } from "@/features/deploys/hooks/use-deploy";

/**
 * One entry of a deploy page's Notices (w1/m659): Render's list under the
 * deploy header that says why the deploy looks the way it does.
 * docs/render-artifacts/deploy-detail-page.md § Notices records each entry's
 * words, condition and link as Render's bundle has them.
 */
export type DeployNotice =
  | { kind: "failure"; troubleshoot: boolean }
  | { kind: "buildCacheCleared" }
  | { kind: "freePlan" }
  | { kind: "maintenance" }
  | { kind: "rolledBackTo"; deployId: string }
  | { kind: "buildGone" }
  | { kind: "syncedFrom"; blueprintId: string }
  | { kind: "cancel"; supersede: boolean; policyLink: boolean }
  | { kind: "queued"; deployId: string; policyLink: boolean }
  | { kind: "healthCheck"; timedOut: boolean; address: string | null }
  | { kind: "stall" };

/** A workspace's Overlapping Deploy Policy, or null while unread. */
export type OverlapPolicy = "wait" | "override" | null;

export interface DeployNoticeFacts {
  /** The service's Health Check Path, "" or null for none. */
  healthCheckPath?: string | null;
  /** The port its health check reaches, when known. */
  port?: number | null;
  /** A queued deploy's in-progress deploy, from the service's deploys. */
  inProgressDeployId?: string | null;
  policy: OverlapPolicy;
}

/** Render's in-progress deploy statuses, the queued deploy's blocker. */
const IN_PROGRESS_DEPLOY_STATUSES = [
  "created",
  "build_in_progress",
  "pre_deploy_in_progress",
  "update_in_progress",
];

const FAILED = new Set(["build_failed", "pre_deploy_failed", "update_failed"]);

/** The cancel reasons a newer deploy's start left: Render's newDeploy. */
const SUPERSEDE_CODES = new Set([
  "superseded_by",
  "superseded_by_newer_release",
]);

/** The facts deciding which reads a deploy's Notices make beyond it. */
type NoticeReadFacts = {
  status?: string | null;
  cancelReasonTemplate?: { code?: string | null } | null;
};

/** A deploy a newer deploy's start canceled. */
function isSupersededDeploy(deploy: NoticeReadFacts): boolean {
  return SUPERSEDE_CODES.has(deploy.cancelReasonTemplate?.code ?? "");
}

/**
 * The reads a deploy's Notices make beyond the deploy itself: a queued
 * deploy's in-progress blocker, and for it or a superseded deploy the
 * workspace's Overlapping Deploy Policy. The page's hook and its route loader
 * (deploy-page-prefetch.ts) both decide by this, so the loader warms exactly
 * what the page reads.
 */
export function noticeReads(deploy: NoticeReadFacts): {
  blocker: boolean;
  policy: boolean;
} {
  const blocker = deploy.status === "queued";
  return { blocker, policy: blocker || isSupersededDeploy(deploy) };
}

/** The service's in-progress deploys, where a queued deploy's blocker is. */
export function inProgressDeploysVariables(serviceId: string) {
  return { serviceId, status: IN_PROGRESS_DEPLOY_STATUSES, limit: 5 };
}

/**
 * The Notices for deploy, in Render's order. Each entry stands only when its
 * fact does; an empty list is a page with no Notices section.
 */
export function deployNotices(
  deploy: DeployView,
  facts: DeployNoticeFacts,
): DeployNotice[] {
  const notices: DeployNotice[] = [];
  if (deploy.failureReason) {
    // Render leaves its troubleshooting docs off an internal failure.
    notices.push({
      kind: "failure",
      troubleshoot: deploy.failureReasonTemplate?.code !== "platform_error",
    });
  }
  if (deploy.clearCache) notices.push({ kind: "buildCacheCleared" });
  if (deploy.isTransitionToFreeTier) notices.push({ kind: "freePlan" });
  if (deploy.trigger === "deployed_by_render") {
    notices.push({ kind: "maintenance" });
  }
  if (deploy.rollbackOf) {
    notices.push({ kind: "rolledBackTo", deployId: deploy.rollbackOf });
  }
  if (deploy.status === "deactivated" && deploy.buildAvailable === false) {
    notices.push({ kind: "buildGone" });
  }
  if (deploy.trigger === "blueprint" && deploy.blueprintId) {
    notices.push({ kind: "syncedFrom", blueprintId: deploy.blueprintId });
  }
  if (deploy.cancelReason) {
    const supersede = isSupersededDeploy(deploy);
    notices.push({
      kind: "cancel",
      supersede,
      policyLink: supersede && facts.policy === "override",
    });
  }
  if (deploy.status === "queued" && facts.inProgressDeployId) {
    notices.push({
      kind: "queued",
      deployId: facts.inProgressDeployId,
      policyLink: facts.policy === "wait",
    });
  }
  if (facts.healthCheckPath) {
    const address = facts.port
      ? `0.0.0.0:${facts.port}${facts.healthCheckPath}`
      : null;
    if (deploy.status === "update_in_progress") {
      notices.push({ kind: "healthCheck", timedOut: false, address });
    } else if (FAILED.has(deploy.status) && deploy.timedOutSeconds) {
      notices.push({ kind: "healthCheck", timedOut: true, address });
    }
  }
  if (deploy.stallReason) notices.push({ kind: "stall" });
  return notices;
}

/** The ports bex reserves, as Render's warning lists them. */
export const RESERVED_PORTS = [18012, 18013, 19099] as const;

/**
 * The reserved port Render's warning names (w1/m659): a deploy in progress or
 * failed, on a free plan, whose instance serves HTTP on a reserved port; null
 * otherwise.
 */
export function reservedPortWarning(
  deploy: DeployView,
  plan: string | null | undefined,
): number | null {
  if (plan !== "free" || !deploy.reservedPort) return null;
  if (deploy.status !== "update_in_progress" && !FAILED.has(deploy.status)) {
    return null;
  }
  return deploy.reservedPort;
}
