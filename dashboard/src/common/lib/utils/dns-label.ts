// A valid resource name: lowercase alphanumerics + hyphens, starting AND
// ending with an alphanumeric (digit-start allowed), 1-30 chars. Mirrors the
// backend's one rule exactly — appv1alpha1.ValidResourceName
// (lego/types/v1alpha1/resourcename.go), the DNS-1123 label every service,
// database, Key Value and workspace name becomes a CR name under — so the
// client can never accept a name the backend would then reject. Both sides are
// tested against lego/types/v1alpha1/testdata/resource-names.json (w5/m118).
const DNS_LABEL_RE = /^[a-z0-9]([a-z0-9-]{0,28}[a-z0-9])?$/;

export function isValidDnsLabel(name: string): boolean {
  return DNS_LABEL_RE.test(name);
}
