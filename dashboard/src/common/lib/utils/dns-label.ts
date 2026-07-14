// A valid resource name: lowercase alphanumerics + hyphens, starting AND
// ending with an alphanumeric (digit-start allowed), 1-30 chars. Mirrors the
// backend's own rule exactly — store.ValidAppName's nameRE
// (lego/backend/internal/store/api.go), the DNS-1123 label every
// resource-creation form's name becomes a CR name under (services, databases,
// Key Value, …) — so the client can never accept a name the backend would
// then reject (or, before w4/m19, silently truncate expectations around: the
// old 63-char/letter-start rule here didn't match the backend's 30-char/
// digit-start-allowed one at all).
const DNS_LABEL_RE = /^[a-z0-9]([a-z0-9-]{0,28}[a-z0-9])?$/;

export function isValidDnsLabel(name: string): boolean {
  return DNS_LABEL_RE.test(name);
}
