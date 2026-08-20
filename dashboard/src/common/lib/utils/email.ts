// A deliberately permissive address check for form gating: it rejects what is
// obviously not an address (no `@`, no domain dot, whitespace) without trying to
// out-parse RFC 5322 — the server stays authoritative, so a false NEGATIVE here
// would block a legitimate address for no reason, while a false positive merely
// costs the round trip that already existed. Mirrors the shape Go's
// mail.ParseAddress accepts for a bare address (bex-api's invite verb).
const PLAIN_ADDRESS = /^[^\s@]+@[^\s@.]+(\.[^\s@.]+)+$/;

/** Whether value is plausibly an email address (form gating, not validation). */
export function isValidEmail(value: string): boolean {
  return PLAIN_ADDRESS.test(value.trim());
}
