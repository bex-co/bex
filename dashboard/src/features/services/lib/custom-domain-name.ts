/**
 * The Add Custom Domain dialog's pre-check, so an obviously bad name costs no
 * round trip. It mirrors what bex-api cheaply can (lego/backend/internal/apps/
 * domains.go `normalizeHostname`, `canonicalHostname`, `claimableHostname`): it
 * accepts every name a person really owns (an alphabetic or punycode top-level
 * domain, a trailing dot) and keeps refusing what cannot be a real domain,
 * holding the top-level label to the server's rule, anything but all digits.
 * The server stays authoritative for the label length, the public suffix, the
 * registrable-domain rule, reserved hosts and quota, and answers those as
 * readable sentences in the dialog (w21/048). The shared corpus
 * lego/backend/testdata/validation/custom-domain.json pins the verdicts on both
 * sides; `TestCustomDomainDialogVectors` pins the stored form.
 */

/** The name as the server stores it: trimmed, with one trailing dot dropped. */
export function normalizedDomainName(raw: string): string {
  return raw.trim().replace(/\.$/, "");
}

// Dot-separated DNS labels, two or more. The top-level label may be
// alphabetic or its internationalized `xn--` punycode form (.рф is xn--p1ai),
// and the server refuses only a numeric one (RFC 1123 §2.1), so a label with a
// digit in it passes here too. Raw Unicode stays refused: the server wants the
// punycode spelling.
const DOMAIN_NAME =
  /^([a-z0-9]([a-z0-9-]*[a-z0-9])?\.)+(?<tld>[a-z0-9]([a-z0-9-]*[a-z0-9])?)$/i;

export type DomainNameError = "invalid";

/**
 * Why the dialog refuses `raw` before sending it, or null to send it. A
 * wildcard is one leading `*.` label over a valid name, as on Render
 * (w4/m326): `*.example.com`.
 */
export function domainNameError(raw: string): DomainNameError | null {
  const name = normalizedDomainName(raw).replace(/^\*\./, "");
  const tld = DOMAIN_NAME.exec(name)?.groups?.tld;
  return tld !== undefined && !/^\d+$/.test(tld) ? null : "invalid";
}
