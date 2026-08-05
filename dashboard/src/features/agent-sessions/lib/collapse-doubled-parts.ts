// A React dev double-mount makes `useChat`'s id-keyed store replay the same
// transcript twice INTO ONE message, doubling its parts (w3/m44). Prod
// single-mounts, so this is dev-only, but collapse an exact doubled-parts
// sequence defensively so the conversation never renders twice. The two replays
// get FRESH per-part ids, so the halves are compared id-agnostically by
// type + visible text + tool name.

type Part = { type: string } & Record<string, unknown>;

function str(value: unknown): string {
  return typeof value === "string" ? value : "";
}

export function collapseDoubledParts<T extends Part>(parts: T[]): T[] {
  const n = parts.length;
  if (n < 4 || n % 2 !== 0) return parts;
  const half = n / 2;
  const key = (p: Part) => `${p.type}|${str(p.text)}|${str(p.toolName)}`;
  for (let i = 0; i < half; i++) {
    if (key(parts[i]) !== key(parts[half + i])) return parts;
  }
  return parts.slice(0, half);
}
