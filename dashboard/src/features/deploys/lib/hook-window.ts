import type { DeployView } from "../hooks/use-deploy";
import { stepLogEnd } from "../hooks/use-canceled-step-end";

type HookWindow = Pick<
  DeployView,
  "finishedAt" | "initialDeployHookStatus" | "initialDeployHookFinishedAt"
>;

/**
 * Where a deploy's own log window ends once its Blueprint's one-time
 * `initialDeployHook` is counted (w33/m435). The hook runs after the deploy is
 * live, so its output and bex-api's lines about it are stamped past
 * `finishedAt`: while it runs the window stays open (`undefined`, which keeps
 * the panel polling), and once it ended the window closes there. bex-api
 * stamps the hook's end in whole seconds, as it stamps a step's end fact, so
 * the bound takes in that whole second (`stepLogEnd`).
 */
export function hookWindowEnd(deploy: HookWindow | null): string | undefined {
  if (!deploy) return undefined;
  if (deploy.initialDeployHookStatus === "running") return undefined;
  const { finishedAt, initialDeployHookFinishedAt: hookEnd } = deploy;
  if (!finishedAt) return undefined;
  return hookEnd ? stepLogEnd(finishedAt, hookEnd) : finishedAt;
}
