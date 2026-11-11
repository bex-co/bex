/**
 * The Add Custom Domain dialog's pre-check, so an obviously bad name costs no
 * round trip. It mirrors what bex-api cheaply can (lego/backend/internal/apps/
 * domains.go `normalizeHostname`, `canonicalHostname`, `claimableHostname`): it
 * accepts every name a person really owns (an alphabetic or punycode top-level
 * domain, a trailing dot) and keeps refusing what cannot be a real domain. The
 * server stays authoritative for the label length, the public suffix, the
 * registrable-domain rule, reserved hosts and quota, and answers those as
 * readable sentences in the dialog (w21/048). `TestCustomDomainDialogVectors`
 * pins the same table against the server.
 */

/** The name as the server stores it: trimmed, with one trailing dot dropped. */
export function normalizedDomainName(raw: string): string {
  return raw.trim().replace(/\.$/, "");
}

// Dot-separated DNS labels ending in an alphabetic top-level domain, or an
// internationalized one in its `xn--` punycode form (.рф is xn--p1ai). A numeric
// TLD stays refused, as the server refuses it. Raw Unicode stays refused too:
// the server wants the punycode spelling.
const DOMAIN_NAME =
  /^([a-z0-9]([a-z0-9-]*[a-z0-9])?\.)+([a-z]{2,}|xn--[a-z0-9-]*[a-z0-9])$/i;

export type DomainNameError = "invalid" | "wildcard";

/**
 * Why the dialog refuses `raw` before sending it, or null to send it. A
 * wildcard name is its own reason: bex-api refuses it deliberately (ADR005), so
 * the person is told so instead of "enter a valid domain".
 */
export function domainNameError(raw: string): DomainNameError | null {
  const name = normalizedDomainName(raw);
  if (name.startsWith("*.")) return "wildcard";
  return DOMAIN_NAME.test(name) ? null : "invalid";
}
