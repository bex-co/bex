import { parseInviteToken } from "./invite-token";

export const VERIFIED_INVITE_ORIGIN = "https://dashboard.bex.co";
export const VERIFIED_INVITE_PATH = "/invite";

/**
 * Accept only the one OS-verified HTTPS handoff and require its capability to
 * byte-match Expo Router's independently parsed route parameter.
 */
export function verifiedInviteToken(
  linkingURL: unknown,
  routeParameter: unknown,
): string | null {
  const routeToken = parseInviteToken(routeParameter);
  if (!routeToken || typeof linkingURL !== "string") return null;

  let url: URL;
  try {
    url = new URL(linkingURL);
  } catch {
    return null;
  }
  const entries = [...url.searchParams.entries()];
  if (
    url.protocol !== "https:" ||
    url.origin !== VERIFIED_INVITE_ORIGIN ||
    url.pathname !== VERIFIED_INVITE_PATH ||
    url.username !== "" ||
    url.password !== "" ||
    url.hash !== "" ||
    linkingURL.includes("#") ||
    entries.length !== 1 ||
    entries[0]?.[0] !== "invite"
  ) {
    return null;
  }
  const linkToken = parseInviteToken(entries[0][1]);
  return linkToken === routeToken ? routeToken : null;
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
