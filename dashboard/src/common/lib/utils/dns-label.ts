// A valid DNS label (Kubernetes object name): starts with a letter, lowercase
// alphanumerics + hyphens, no trailing hyphen, <=63 chars. Shared by every
// resource-creation form whose name becomes a CR name (databases, Key Value,
// …) so the rule has one source of truth instead of a copy per feature.
const DNS_LABEL_RE = /^[a-z]([-a-z0-9]*[a-z0-9])?$/;

export function isValidDnsLabel(name: string): boolean {
  return DNS_LABEL_RE.test(name) && name.length <= 63;
}
