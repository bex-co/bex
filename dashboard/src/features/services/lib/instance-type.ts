// Display helpers for lego/types/tiers' CPU/Memory quantity strings (the same
// k8s resource.Quantity spellings the operator parses), matching Render's own
// instance-type card labels captured live: "0.5 CPU", "512 MB", "2 GB".

/** "500m" -> "0.5 CPU"; "1" -> "1 CPU"; "8" -> "8 CPU". */
export function formatInstanceCPU(cpu: string): string {
  const cores = cpu.endsWith("m") ? parseInt(cpu, 10) / 1000 : parseFloat(cpu);
  const n = Number.isInteger(cores) ? cores.toString() : cores.toFixed(1);
  return `${n} CPU`;
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
 * neither on it: Background Workers (w6/025) and Private Services (w1/111).
 *
 * Web services and cron jobs keep Free — Render sells both that way — and a
 * static site runs no instance at all, so this is a two-type allowlist rather
 * than "everything but web".
 */
const PAID_ONLY_SERVICE_TYPES = new Set([
  "background_worker",
  "private_service",
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
  if (serviceType === null || !PAID_ONLY_SERVICE_TYPES.has(serviceType)) {
    return instanceTypes;
  }
  return instanceTypes.filter((it) => it.id !== "free");
}
