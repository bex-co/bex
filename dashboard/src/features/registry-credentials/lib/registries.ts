import type { en } from "@/i18n";
import {
  canonicalRegistryHost,
  registryHostError,
} from "@/features/registry-credentials/lib/credential-fields";

/**
 * Render's registry enum, in the order its Add Credential dialog lists it
 * (docs/render-artifacts/registry-credential-dialog.md, w4/m670).
 */
export const RENDER_REGISTRIES = [
  "GITHUB",
  "GITLAB",
  "DOCKER",
  "GOOGLE_ARTIFACT",
  "AWS_ECR",
] as const;

export type RenderRegistry = (typeof RENDER_REGISTRIES)[number];

/** Render's labels for its registries: product names, the same in every language. */
export const REGISTRY_LABELS: Record<RenderRegistry, string> = {
  GITHUB: "GitHub",
  GITLAB: "GitLab",
  DOCKER: "Docker",
  GOOGLE_ARTIFACT: "Google Artifact",
  AWS_ECR: "AWS ECR",
};

/**
 * An example host for the registries whose host names a project or account
 * and region, so the dialog asks for it (bex's `host` extension; ADR018).
 * The other three have one host, which bex-api fills in from `registry`.
 */
export const REGISTRY_HOST_EXAMPLES: Partial<Record<RenderRegistry, string>> = {
  GOOGLE_ARTIFACT: "us-docker.pkg.dev",
  AWS_ECR: "123456789012.dkr.ecr.us-east-1.amazonaws.com",
};

export function isRenderRegistry(value: unknown): value is RenderRegistry {
  return RENDER_REGISTRIES.includes(value as RenderRegistry);
}

const FIXED_HOSTS: Record<string, RenderRegistry> = {
  "docker.io": "DOCKER",
  "ghcr.io": "GITHUB",
  "registry.gitlab.com": "GITLAB",
};
const ECR_HOST = /^[0-9]{12}\.dkr\.ecr\.[a-z0-9-]+\.amazonaws\.com$/;

/**
 * The registry a host belongs to, or null for a self-hosted registry Render
 * has no value for: bex-api's `renderRegistry`
 * (lego/backend/internal/registrycreds/registry.go), which refuses a
 * `registry`/`host` pair this disagrees with.
 */
export function registryForHost(raw: string): RenderRegistry | null {
  const host = canonicalRegistryHost(raw);
  if (host in FIXED_HOSTS) return FIXED_HOSTS[host];
  if (
    host === "gcr.io" ||
    host.endsWith(".gcr.io") ||
    host.endsWith("-docker.pkg.dev")
  ) {
    return "GOOGLE_ARTIFACT";
  }
  return ECR_HOST.test(host) ? "AWS_ECR" : null;
}

// Render's image parser, from the RegistryIcon chunk of its dashboard bundle
// (docs/render-artifacts/registry-credential-dialog.md, read 2026-11-27).
const RENDER_FIXED_HOSTS = new Set([
  "docker.io",
  "ghcr.io",
  "registry.gitlab.com",
]);
const RENDER_KNOWN_HOSTS = new Set([
  ...RENDER_FIXED_HOSTS,
  "registry.k8s.io",
  "k8s.gcr.io",
]);

/** The registry Render's parser gives an image's first path segment. */
function renderRegistryForSegment(segment: string): RenderRegistry | null {
  const host = segment.toLowerCase();
  if (/pkg\.dev/.test(host)) return "GOOGLE_ARTIFACT";
  if (/dkr\.ecr/.test(host)) return "AWS_ECR";
  if (host === "docker.io" || host.endsWith(".docker.io")) return "DOCKER";
  if (host === "ghcr.io") return "GITHUB";
  return /gitlab/.test(host) ? "GITLAB" : null;
}

/**
 * The registry Render's image parser gives an image (its
 * `parsedRegistryType`), which narrows the Credential dropdown of Render's
 * service image forms (w4/735), or null to list every credential: a blank
 * field, or a host Render has no registry for. A name with no host, such as
 * `nginx` or `acme/web`, is Docker Hub.
 */
export function registryForImage(raw: string): RenderRegistry | null {
  const image = raw.trim();
  if (image === "") return null;
  const [first, , third] = image.split("/");
  const registry = renderRegistryForSegment(first);
  if (!image.includes("/")) {
    return RENDER_KNOWN_HOSTS.has(image) ? registry : "DOCKER";
  }
  const known = RENDER_KNOWN_HOSTS.has(first);
  if (!known && /[:/]$/.test(image)) return "DOCKER";
  if (
    !RENDER_FIXED_HOSTS.has(first) &&
    first !== "library" &&
    registry !== "AWS_ECR"
  ) {
    return !known && /^[^/.]*\/[^/]*$/.test(image) ? "DOCKER" : registry;
  }
  if (known && third === undefined) return registry;
  return registry ?? "DOCKER";
}

/**
 * The host bex-api checks a chosen credential against (`registryHost`,
 * lego/backend/internal/registrycreds/pullsecret.go): the first path segment
 * when it names a host (a dot, a port, or localhost), Docker Hub otherwise.
 */
function imageRegistryHost(image: string): string {
  const [first, ...path] = image.trim().split("/");
  const named =
    path.length > 0 && (first === "localhost" || /[.:]/.test(first));
  return named ? canonicalRegistryHost(first) : "docker.io";
}

/**
 * Whether Render's service image forms list a credential for an image
 * (w4/735): every credential while the image names no registry Render
 * knows, otherwise the credentials for that registry. A self-hosted
 * credential, which Render's enum cannot name, matches only its own host.
 */
export function credentialFitsImage(
  credential: { registry: RenderRegistry | null; host: string },
  image: string,
): boolean {
  const registry = registryForImage(image);
  if (registry === null) return true;
  return credential.registry === null
    ? canonicalRegistryHost(credential.host) === imageRegistryHost(image)
    : credential.registry === registry;
}

/**
 * The Registry host of a Google Artifact or ECR credential: the host rules,
 * then the registry it must belong to, so the pair bex-api would refuse
 * ("registry … does not match host") is marked in the field before Add.
 */
export function hostForRegistryError(
  raw: string,
  registry: RenderRegistry,
): keyof typeof en | null {
  const reason = registryHostError(raw);
  if (reason || raw.trim() === "") return reason;
  return registryForHost(raw) === registry
    ? null
    : "registryCredentials.hostRegistryMismatch";
}
