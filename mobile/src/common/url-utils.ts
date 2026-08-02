const ALLOWED_DEEP_LINK_ROUTES = new Set(["status", "activity", "sessions"]);

export function safeDeepLink(path: string): string | null {
  const normalized = path.replace(/^\/+/, "");
  const root = normalized.split("/", 1)[0];
  if (!root || !ALLOWED_DEEP_LINK_ROUTES.has(root)) return null;
  return `/${normalized}`;
}

export function isAllowedHttpsUrl(
  value: string,
  hosts: readonly string[],
): boolean {
  try {
    const url = new URL(value);
    return url.protocol === "https:" && hosts.includes(url.hostname);
  } catch {
    return false;
  }
}
