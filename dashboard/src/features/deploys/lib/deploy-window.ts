import type { DeployView } from "../hooks/use-deploy";
import { stepLogEnd } from "../hooks/use-canceled-step-end";

type DeployWindow = Pick<
  DeployView,
  | "status"
  | "finishedAt"
  | "initialDeployHookStatus"
  | "initialDeployHookFinishedAt"
>;

/**
 * How far past `finishedAt` bex-api stamps the last line of a live close:
 * Render's primary URL block, six lines a millisecond apart after a web
 * service's `==> Your service is live 🎉` (w32/m567, `primaryURLLines` in
 * lego/backend/internal/logs/progress.go), plus the millisecond `Date.parse`
 * drops from `finishedAt`'s microseconds.
 */
const LIVE_LINES_MS = 7;

/**
 * Where a deploy's own log window ends. A deploy that went live closes it just
 * past the primary URL block bex-api writes after its live line (w32/m567).
 * The Blueprint's one-time `initialDeployHook` runs after the deploy is live
 * (w33/m435), so its output and bex-api's lines about it come later still:
 * while it runs the window stays open (`undefined`, which keeps the panel
 * polling), and once it ended the window closes there. bex-api stamps the
 * hook's end in whole seconds, as it stamps a step's end fact, so the bound
 * takes in that whole second (`stepLogEnd`).
 */
export function deployWindowEnd(
  deploy: DeployWindow | null,
): string | undefined {
  if (!deploy) return undefined;
  if (deploy.initialDeployHookStatus === "running") return undefined;
  const { finishedAt, initialDeployHookFinishedAt: hookEnd } = deploy;
  if (!finishedAt) return undefined;
  const closed = liveLinesEnd(deploy.status, finishedAt);
  return hookEnd ? stepLogEnd(closed, hookEnd) : closed;
}

function liveLinesEnd(status: string, finishedAt: string): string {
  if (status !== "live" && status !== "deactivated") return finishedAt;
  const at = Date.parse(finishedAt);
  return Number.isFinite(at)
    ? new Date(at + LIVE_LINES_MS).toISOString()
    : finishedAt;
}
