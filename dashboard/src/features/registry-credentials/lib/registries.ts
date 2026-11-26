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
