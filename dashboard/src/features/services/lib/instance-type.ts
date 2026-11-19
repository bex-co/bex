import type { en } from "@/i18n";
// Display helpers for lego/types/tiers' CPU/Memory quantity strings (the same
// k8s resource.Quantity spellings the operator parses), matching Render's own
// plan card labels captured live: "0.5 CPU", "512 MB", "2 GB".

/** "500m" -> "0.5 CPU"; "1" -> "1 CPU"; "8" -> "8 CPU". */
export function formatInstanceCPU(cpu: string): string {
  const cores = cpu.endsWith("m") ? parseInt(cpu, 10) / 1000 : parseFloat(cpu);
  const n = Number.isInteger(cores) ? cores.toString() : cores.toFixed(1);
  return `${n} CPU`;
}

interface NamedPlan {
  /** The legacy Render spelling ("starter"). */
  id: string;
  /** Render's plan ID ("0.5c-512mb"); omitted where the family has none. */
  planId?: string;
  name: string;
}

/**
 * The plan ID worth showing beside a plan's legacy name (w6/m483), or
 * undefined where it is the legacy id itself (Free, an older bex-api's
 * catalog) or the family has none (Key Value, Postgres).
 */
export function distinctPlanId(plan: NamedPlan): string | undefined {
  return plan.planId && plan.planId !== plan.id ? plan.planId : undefined;
}

/**
 * A compute plan in one line of text, for a sentence or a dialog title: the
 * plan ID with the legacy name in brackets ("0.5c-512mb (Starter)"), else the
 * name alone — the text form of ComputePlanName.
 */
export function computePlanLabel(plan: NamedPlan): string {
  const planId = distinctPlanId(plan);
  return planId ? `${planId} (${plan.name})` : plan.name;
}

/** "512Mi" -> "512 MB"; "2Gi" -> "2 GB" (Render's card unit spelling, not Mi/Gi). */
export function formatInstanceMemory(memory: string): string {
  const match = /^(\d+(?:\.\d+)?)(Mi|Gi)$/.exec(memory);
  if (!match) return memory;
  const [, amount, unit] = match;
  return `${amount} ${unit === "Mi" ? "MB" : "GB"}`;
}

/**
 * Service types bex never offers the compute `free` tier, because Render sells
 * none of them on it: Background Workers (w6/025), Private Services (w1/111)
 * and Cron Jobs (w8/048).
 *
 * Web services keep Free, and a static site runs no instance at all, so this
 * is an allowlist rather than "everything but web".
 */
const PAID_ONLY_SERVICE_TYPES = new Set([
  "background_worker",
  "private_service",
  "cron_job",
]);

/**
 * The catalog tiers a service of this type may be offered. For a paid-only type
 * Free never appears in the create form's plan grid or the instance-type picker,
 * and a Free selection made under another type does not survive a switch into
 * one. bex-api refuses a free plan for these types server-side as well
 * (`paidOnlyServiceType` / `errFreePlanForType`), so this filter is
 * presentation, not the enforcement.
 */
export function offeredInstanceTypes<T extends { id: string }>(
  serviceType: string | null,
  instanceTypes: T[],
): T[] {
  if (serviceType === null || offersFreeTier(serviceType)) {
    return instanceTypes;
  }
  return instanceTypes.filter((it) => it.id !== "free");
}

/** Whether a service of this (known) type may be offered the Free tier. */
export function offersFreeTier(serviceType: string): boolean {
  return !PAID_ONLY_SERVICE_TYPES.has(serviceType);
}

// Each paid-only type's Plan-page notice, while still on Free or untiered.
const LEGACY_FREE_NOTICE: Record<
  string,
  { free: keyof typeof en; untiered: keyof typeof en }
> = {
  background_worker: {
    free: "services.planPickerLegacyFreeWorker",
    untiered: "services.planPickerUntieredWorker",
  },
  private_service: {
    free: "services.planPickerLegacyFreePrivateService",
    untiered: "services.planPickerUntieredPrivateService",
  },
  cron_job: {
    free: "services.planPickerLegacyFreeCronJob",
    untiered: "services.planPickerUntieredCronJob",
  },
};

/**
 * The notice a paid-only service still on Free, or untiered, gets on its Plan
 * page: Free is no longer offered to its type, so no card matches its current
 * plan and nothing is pre-selected (ADR030 §7, w4/351). Null for any other
 * service, and while the plan is unknown (undefined).
 */
export function legacyFreeNotice(
  serviceType: string | null,
  currentPlan: string | null | undefined,
): keyof typeof en | null {
  const notice = serviceType ? LEGACY_FREE_NOTICE[serviceType] : undefined;
  if (!notice) return null;
  if (currentPlan === "free") return notice.free;
  if (currentPlan === null) return notice.untiered;
  return null;
}

/** The compute ladder's tier count (lego/types/tiers/tiers.yaml, free through
 *  pro-ultra), pinned by a test against that file. */
export const COMPUTE_TIER_COUNT = 7;

/** How many plan cards the create form offers this service type before its
 *  catalog arrives — the ladder, less Free for a paid-only type — so the
 *  pending grid has the ready grid's rows (w4/329). */
export function servicePlanSlotCount(serviceType: string): number {
  return offersFreeTier(serviceType)
    ? COMPUTE_TIER_COUNT
    : COMPUTE_TIER_COUNT - 1;
}
