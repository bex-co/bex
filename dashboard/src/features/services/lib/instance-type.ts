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
