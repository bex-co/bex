import { parseInviteToken } from "./invite-token";

export const VERIFIED_INVITE_ORIGIN = "https://dashboard.bex.co";
export const VERIFIED_INVITE_PATH = "/invite";

function hasInviteParameter(url: URL): boolean {
  if (url.searchParams.has("invite")) return true;
  if (!url.hash) return false;
  return new URLSearchParams(url.hash.slice(1)).has("invite");
}

/** Whether a verified-link candidate should be scrubbed even before parsing. */
export function hasInviteLinkingParameter(linkingURL: unknown): boolean {
  if (typeof linkingURL !== "string") return false;
  try {
    return hasInviteParameter(new URL(linkingURL));
  } catch {
    return false;
  }
}

/**
 * Accept only the one OS-verified HTTPS handoff. Query links must byte-match
 * Expo Router's independently parsed route parameter; fragment links are read
 * directly from the OS URL because the router intentionally does not expose
 * fragments as route params.
 */
export function verifiedInviteToken(
  linkingURL: unknown,
  routeParameter: unknown,
): string | null {
  const routeToken =
    routeParameter === undefined ? null : parseInviteToken(routeParameter);
  if (routeParameter !== undefined && !routeToken) return null;
  if (typeof linkingURL !== "string") return null;

  let url: URL;
  try {
    url = new URL(linkingURL);
  } catch {
    return null;
  }
  const entries = [...url.searchParams.entries()];
  const fragmentEntries = url.hash
    ? [...new URLSearchParams(url.hash.slice(1)).entries()]
    : [];
  const hasHash = linkingURL.includes("#");
  let linkToken: string | null = null;
  if (!hasHash && entries.length === 1 && entries[0]?.[0] === "invite") {
    linkToken = parseInviteToken(entries[0][1]);
  } else if (
    hasHash &&
    fragmentEntries.length === 1 &&
    fragmentEntries[0]?.[0] === "invite"
  ) {
    linkToken = parseInviteToken(fragmentEntries[0][1]);
  }
  if (
    url.protocol !== "https:" ||
    url.origin !== VERIFIED_INVITE_ORIGIN ||
    url.pathname !== VERIFIED_INVITE_PATH ||
    url.username !== "" ||
    url.password !== "" ||
    !linkToken
  ) {
    return null;
  }
  return routeToken && linkToken !== routeToken ? null : linkToken;
}

/**
 * Remove the bearer from navigation synchronously, before waiting on secure
 * storage. The capture promise keeps the snapshotted value alive without ever
 * publishing it through component state or rendered copy.
 */
export function bootstrapInviteLink(
  value: unknown,
  scrubRoute: () => void,
  capture: (value: unknown) => Promise<boolean>,
): Promise<boolean> {
  scrubRoute();
  return capture(value);
}
