import { proFeaturesPlan } from "@/features/workspaces/types";

/**
 * Whether autoscaling is held off for a service's plan, and why: it is
 * paid-only (w4/365), and a Hobby workspace cannot turn it on (w7/m429;
 * Render: Pro and higher).
 */
export function autoscalingLock(
  plan: string | null | undefined,
  workspacePlan: string | null | undefined,
): { eligible: boolean; planGated: boolean; locked: boolean } {
  const eligible = plan !== "free";
  const planGated = eligible && !proFeaturesPlan(workspacePlan);
  return { eligible, planGated, locked: !eligible || planGated };
}
