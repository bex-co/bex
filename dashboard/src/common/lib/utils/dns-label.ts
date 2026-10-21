// A valid resource name: lowercase alphanumerics + hyphens, starting AND
// ending with an alphanumeric (digit-start allowed), 1-30 chars. Mirrors the
// backend's one rule exactly — appv1alpha1.ValidResourceName
// (lego/types/v1alpha1/resourcename.go), the DNS-1123 label a service's
// resource name and a workspace name become a CR name under — so the client
// can never accept a name the backend would then reject. (Postgres and Key
// Value names, and a service's typed name, are free-form labels since w5/m153:
// resource-label.ts.) Both sides are
// tested against lego/types/v1alpha1/testdata/resource-names.json (w5/m118).
const DNS_LABEL_RE = /^[a-z0-9]([a-z0-9-]*[a-z0-9])?$/;

/** A DNS-1123 label of at most `maxLength` characters. */
export function isDnsLabel(name: string, maxLength: number): boolean {
  return name.length <= maxLength && DNS_LABEL_RE.test(name);
}

export function isValidDnsLabel(name: string): boolean {
  return isDnsLabel(name, 30);
}
