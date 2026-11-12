import type { en } from "@/i18n";

/**
 * The rules bex-api applies to a registry credential's fields
 * (lego/backend/internal/registrycreds: host.go `canonicalRegistryHost`,
 * service.go `validateCredentialFields`), checked in the Add and Edit dialogs so
 * a refused value is marked in its field in the person's language instead of an
 * English toast after the click (w21/053). The server stays authoritative; the
 * vectors in `__tests__/credential-fields.test.ts` are the ones its Go test
 * pins.
 */
type Reason = keyof typeof en;

const utf8 = new TextEncoder();
const MAX_FIELD_BYTES = 253;
const MAX_SECRET_BYTES = 64 * 1024;

// A bare registry host with an optional port of 1–65535 (w4/577): DNS
// labels, or localhost.
const REGISTRY_HOST =
  /^(localhost|[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)*)(:([1-9][0-9]{0,3}|[1-5][0-9]{4}|6[0-4][0-9]{3}|65[0-4][0-9]{2}|655[0-2][0-9]|6553[0-5]))?$/;

// The other spellings of Docker Hub the server folds to docker.io.
const DOCKER_HUB_ALIASES = new Set([
  "index.docker.io",
  "registry-1.docker.io",
  "registry.hub.docker.com",
]);

/**
 * What the server stores for a pasted host: a leading `https://` or `http://`
 * dropped (in any case), trailing slashes dropped, lowercased, Docker Hub's
 * aliases folded. A path, a query or anything that is still not host[:port]
 * stays as is for the check below to refuse.
 */
export function canonicalRegistryHost(raw: string): string {
  let host = raw.trim();
  const lower = host.toLowerCase();
  for (const scheme of ["https://", "http://"]) {
    if (lower.startsWith(scheme)) {
      host = host.slice(scheme.length);
      break;
    }
  }
  host = host.replace(/\/+$/, "").toLowerCase();
  return DOCKER_HUB_ALIASES.has(host) ? "docker.io" : host;
}

/** The Registry host, or null when it is blank (the dialog asks for it) or fine. */
export function registryHostError(raw: string): Reason | null {
  if (raw.trim() === "") return null;
  const host = canonicalRegistryHost(raw);
  if (host === "" || !REGISTRY_HOST.test(host)) {
    return "registryCredentials.hostInvalid";
  }
  return utf8.encode(host).length > MAX_FIELD_BYTES
    ? "registryCredentials.hostTooLong"
    : null;
}

/** The username: no control characters (Unicode Cc), at most 253 bytes. */
export function usernameError(raw: string): Reason | null {
  const username = raw.trim();
  if (/\p{Cc}/u.test(username)) return "registryCredentials.usernameControl";
  return utf8.encode(username).length > MAX_FIELD_BYTES
    ? "registryCredentials.usernameTooLong"
    : null;
}

/** The password or token: at most 64 KiB. */
export function authTokenError(raw: string): Reason | null {
  return utf8.encode(raw.trim()).length > MAX_SECRET_BYTES
    ? "registryCredentials.authTokenTooLong"
    : null;
}
