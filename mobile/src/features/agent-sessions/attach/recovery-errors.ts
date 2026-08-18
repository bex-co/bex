export function isTerminalAttachFailure(error: unknown): boolean {
  const pending: unknown[] = [error];
  const seen = new Set<unknown>();
  while (pending.length > 0) {
    const current = pending.shift();
    if (current == null || seen.has(current)) continue;
    seen.add(current);
    if (typeof current === "string") {
      if (/\b(?:401|403|409)\b/.test(current)) return true;
      continue;
    }
    if (typeof current !== "object") continue;
    const record = current as Record<string, unknown>;
    for (const key of ["status", "statusCode", "code"]) {
      const value = record[key];
      if (value === 401 || value === 403 || value === 409) return true;
      if (typeof value === "string" && /^(?:401|403|409)$/.test(value)) {
        return true;
      }
    }
    for (const key of [
      "message",
      "cause",
      "networkError",
      "errors",
      "graphQLErrors",
    ]) {
      const value = record[key];
      if (Array.isArray(value)) pending.push(...value);
      else if (value != null) pending.push(value);
    }
  }
  return false;
}
