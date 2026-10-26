import type { en } from "@/i18n";
import type {
  StaticHeaderView,
  StaticRouteView,
} from "@/features/services/types";

/**
 * The per-field rules bex-api enforces on static-site edge rules
 * (lego/backend/internal/apps/service.go validateStaticRoutes/Headers), checked
 * in the row so an invalid rule is marked where it is, before Save, instead of
 * a toast naming "Routes[1]" by its zero-based wire index (w4/145). The server
 * stays authoritative; these are the rules a row can check on its own.
 */
export type RuleFieldErrors<T> = Partial<Record<keyof T, keyof typeof en>>;

// The server trims surrounding whitespace before checking (w4/136), so the row
// does too.
function pathError(raw: string): keyof typeof en | undefined {
  const value = raw.trim();
  if (value === "") return "services.staticRuleRequired";
  if (!value.startsWith("/")) return "services.staticRulePathSlash";
  return undefined;
}

// Check the configured path before browser URL normalization can erase dot
// segments or backslashes. Decode percent bytes once, matching Go's URL.Path;
// query/fragment escapes are validated but remain data, not path selectors.
function unsafeDestination(value: string): boolean {
  if (/%(?![0-9a-f]{2})/i.test(value)) return true;
  const path = value
    .split(/[?#]/, 1)[0]
    .replace(/%([0-9a-f]{2})/gi, (_, byte: string) =>
      String.fromCharCode(Number.parseInt(byte, 16)),
    );
  const unsafe = (input: string) => {
    if (input.startsWith("//") || input.includes("\\")) return true;
    for (const character of input) {
      const code = character.charCodeAt(0);
      if (code < 0x20 || code === 0x7f) return true;
    }
    return false;
  };
  return unsafe(value) || unsafe(path);
}

// A redirect (w8/162) or a rewrite (w8/184) may name a full http(s) URL, as on
// Render: a literal DNS-style host, no credentials, no wildcard. The server
// also refuses a rewrite whose host can never be public.
const EXTERNAL_REDIRECT = /^https?:\/\/[a-z0-9.-]+(:\d+)?([/?#][^\\\s]*)?$/i;

function externalRedirectError(value: string): keyof typeof en | undefined {
  if (
    !EXTERNAL_REDIRECT.test(value) ||
    unsafeDestination(
      value.slice(value.indexOf("//") + 2).replace(/^[^/?#]*/, ""),
    )
  ) {
    return "services.staticRuleExternalHost";
  }
  return undefined;
}

export function routeErrors(
  route: StaticRouteView,
): RuleFieldErrors<StaticRouteView> {
  const errors: RuleFieldErrors<StaticRouteView> = {};
  const source = pathError(route.source);
  if (source) errors.source = source;
  const trimmedDestination = route.destination.trim();
  if (/^https?:\/\//i.test(trimmedDestination)) {
    const external = externalRedirectError(trimmedDestination);
    if (external) errors.destination = external;
    return errors;
  }
  const destination = pathError(route.destination);
  if (destination) {
    errors.destination = destination;
  } else if (unsafeDestination(route.destination.trim())) {
    // A network-path reference would leave the site (the open-redirect guard,
    // ADR029).
    errors.destination = "services.staticRuleLocalPath";
  }
  return errors;
}

// The header budgets and reserved names mirror bex-api's validateHeaders
// (lego/backend/internal/apps/service.go) and the Go reserved list
// (lego/types/v1alpha1/static_headers.go); the dashboard test pins the list so
// the two cannot drift apart (w4/402).
const MAX_STATIC_PATH_LEN = 2048;
const MAX_STATIC_HEADER_NAME_LEN = 256;
const MAX_STATIC_HEADER_VALUE_LEN = 4096;

export const RESERVED_STATIC_HEADERS = [
  "Content-Length",
  "Transfer-Encoding",
  "Connection",
  "Keep-Alive",
  "Upgrade",
  "Te",
  "Trailer",
  "Proxy-Connection",
] as const;

const HEADER_NAME_TOKEN = /^[!#$%&'*+\-.^_`|~0-9A-Za-z]+$/;

function validHeaderValue(value: string): boolean {
  for (const ch of value) {
    const code = ch.codePointAt(0)!;
    if (code === 0x09) continue; // HTAB is allowed in a field value.
    if (code < 0x20 || code === 0x7f || code > 0xff) return false;
  }
  return true;
}

function setCookieHasDomain(value: string): boolean {
  return value
    .split(";")
    .slice(1)
    .some((attr) => attr.trim().toLowerCase().startsWith("domain="));
}

export function headerErrors(
  header: StaticHeaderView,
): RuleFieldErrors<StaticHeaderView> {
  const errors: RuleFieldErrors<StaticHeaderView> = {};
  const path = pathError(header.path);
  if (path) errors.path = path;
  else if (header.path.trim().length > MAX_STATIC_PATH_LEN)
    errors.path = "services.staticRulePathTooLong";

  const name = header.name.trim();
  if (name === "") {
    errors.name = "services.staticRuleRequired";
  } else if (name.length > MAX_STATIC_HEADER_NAME_LEN) {
    errors.name = "services.staticRuleHeaderNameTooLong";
  } else if (!HEADER_NAME_TOKEN.test(header.name)) {
    errors.name = "services.staticRuleHeaderNameInvalid";
  } else if (
    RESERVED_STATIC_HEADERS.some(
      (reserved) => reserved.toLowerCase() === name.toLowerCase(),
    )
  ) {
    errors.name = "services.staticRuleHeaderNameReserved";
  }

  if (header.value.length > MAX_STATIC_HEADER_VALUE_LEN) {
    errors.value = "services.staticRuleHeaderValueTooLong";
  } else if (!validHeaderValue(header.value)) {
    errors.value = "services.staticRuleHeaderValueInvalid";
  } else if (
    name.toLowerCase() === "set-cookie" &&
    setCookieHasDomain(header.value)
  ) {
    errors.value = "services.staticRuleCookieDomain";
  }
  return errors;
}

/**
 * Rewrites the server's wire index ("routes[1].destination …", which reaches
 * the toast capitalized as "Routes[1]") into the row a person counts ("Row 2:
 * destination …"). A message without an index passes through unchanged.
 */
export function humanizeRuleError(
  message: string,
  rowLabel: (row: number) => string,
): string {
  return message.replace(
    /\b(?:routes|headers)\[(\d+)\]\.?/i,
    (_, index: string) => `${rowLabel(Number(index) + 1)} `,
  );
}
