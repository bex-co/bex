import { describe, expect, it } from "vitest";
import {
  credentialFitsImage,
  hostForRegistryError,
  registryForHost,
  registryForImage,
} from "@/features/registry-credentials/lib/registries";

// bex-api's renderRegistry (lego/backend/internal/registrycreds/registry.go):
// createHost refuses a registry/host pair this disagrees with (w4/m670).
describe("registryForHost", () => {
  it.each([
    ["docker.io", "DOCKER"],
    ["index.docker.io", "DOCKER"],
    ["https://GHCR.IO/", "GITHUB"],
    ["registry.gitlab.com", "GITLAB"],
    ["gcr.io", "GOOGLE_ARTIFACT"],
    ["eu.gcr.io", "GOOGLE_ARTIFACT"],
    ["us-docker.pkg.dev", "GOOGLE_ARTIFACT"],
    ["europe-west1-docker.pkg.dev", "GOOGLE_ARTIFACT"],
    ["123456789012.dkr.ecr.us-east-1.amazonaws.com", "AWS_ECR"],
    ["registry.example.com:5000", null],
    ["gitlab.example.com", null],
    ["12345.dkr.ecr.us-east-1.amazonaws.com", null],
    ["pkg.dev", null],
    ["us-python.pkg.dev", null],
  ])("%j belongs to %j", (host, registry) => {
    expect(registryForHost(host)).toBe(registry);
  });
});

// Render's image parser (its `parsedRegistryType`), which narrows the
// Credential dropdown of Render's service image forms (w4/735). Every
// expectation is what Render's own parser, run from its dashboard bundle's
// RegistryIcon chunk on 2026-11-27, answered for the same input.
describe("registryForImage", () => {
  it.each([
    ["ghcr.io/acme/web:1", "GITHUB"],
    ["GHCR.io/acme/web", "GITHUB"],
    ["nginx", "DOCKER"],
    ["nginx:1.27", "DOCKER"],
    ["acme/web:1", "DOCKER"],
    ["acme/team/web", null],
    ["library/nginx", "DOCKER"],
    ["docker.io/library/nginx", "DOCKER"],
    ["docker.io/nginx", "DOCKER"],
    ["index.docker.io/acme/web", "DOCKER"],
    ["registry-1.docker.io/acme/web", "DOCKER"],
    ["registry.gitlab.com/acme/web", "GITLAB"],
    ["gitlab.example.com/acme/web", "GITLAB"],
    ["us-docker.pkg.dev/proj/repo/web", "GOOGLE_ARTIFACT"],
    ["gcr.io/proj/web", null],
    ["k8s.gcr.io/pause", null],
    ["registry.k8s.io/pause", null],
    ["registry.k8s.io", null],
    ["123456789012.dkr.ecr.us-east-1.amazonaws.com/web", "AWS_ECR"],
    ["123456789012.dkr.ecr.us-east-1.amazonaws.com", "DOCKER"],
    ["registry.example.com/web:1", null],
    ["registry.example.com:5000/web", null],
    ["localhost/web", "DOCKER"],
    ["localhost:5000/web", "DOCKER"],
    ["ghcr.io", "GITHUB"],
    ["ghcr.io/", "GITHUB"],
    ["ghcr.io/acme", "GITHUB"],
    ["docker.io", "DOCKER"],
    ["registry.gitlab.com", "GITLAB"],
    ["acme/web/", "DOCKER"],
    ["acme/web:", "DOCKER"],
    ["registry.example.com/", "DOCKER"],
    ["quay.io/acme/web", null],
    ["ghcr.io/acme/web/sub:1", "GITHUB"],
    ["mcr.microsoft.com/dotnet/runtime:8.0", null],
    ["public.ecr.aws/nginx/nginx", null],
  ])("%j is %j on Render", (image, registry) => {
    expect(registryForImage(image)).toBe(registry);
  });

  it("names no registry for a blank field", () => {
    expect(registryForImage("")).toBeNull();
    expect(registryForImage("   ")).toBeNull();
  });
});

describe("credentialFitsImage", () => {
  const github = { registry: "GITHUB" as const, host: "ghcr.io" };
  const hub = { registry: "DOCKER" as const, host: "docker.io" };
  const gitlab = { registry: "GITLAB" as const, host: "registry.gitlab.com" };
  const own = { registry: null, host: "https://GitLab.example.com/" };

  it("keeps the credentials for the image's registry", () => {
    expect(credentialFitsImage(github, "ghcr.io/acme/web:1")).toBe(true);
    expect(credentialFitsImage(hub, "ghcr.io/acme/web:1")).toBe(false);
    expect(credentialFitsImage(own, "ghcr.io/acme/web:1")).toBe(false);
    expect(credentialFitsImage(hub, "nginx:1.27")).toBe(true);
    expect(credentialFitsImage(github, "nginx:1.27")).toBe(false);
  });

  it("keeps a self-hosted credential for its own host only", () => {
    // Render's parser calls any host with "gitlab" in it GitLab.
    expect(credentialFitsImage(own, "gitlab.example.com/acme/web")).toBe(true);
    expect(credentialFitsImage(gitlab, "gitlab.example.com/acme/web")).toBe(
      true,
    );
    expect(credentialFitsImage(own, "registry.gitlab.com/acme/web")).toBe(
      false,
    );
  });

  it("keeps every credential for a host Render has no registry for", () => {
    for (const credential of [github, hub, gitlab, own]) {
      expect(
        credentialFitsImage(credential, "registry.example.com/web:1"),
      ).toBe(true);
      expect(credentialFitsImage(credential, "")).toBe(true);
    }
  });
});

describe("hostForRegistryError", () => {
  it("leaves a blank host to the required check", () => {
    expect(hostForRegistryError("", "GOOGLE_ARTIFACT")).toBeNull();
  });

  it("reports the host rules before the registry", () => {
    expect(
      hostForRegistryError("us-docker.pkg.dev/p/r", "GOOGLE_ARTIFACT"),
    ).toBe("registryCredentials.hostInvalid");
  });

  it("refuses a host of another registry and accepts its own", () => {
    expect(hostForRegistryError("ghcr.io", "GOOGLE_ARTIFACT")).toBe(
      "registryCredentials.hostRegistryMismatch",
    );
    expect(hostForRegistryError("us-docker.pkg.dev", "AWS_ECR")).toBe(
      "registryCredentials.hostRegistryMismatch",
    );
    expect(
      hostForRegistryError(
        "123456789012.dkr.ecr.eu-west-1.amazonaws.com",
        "AWS_ECR",
      ),
    ).toBeNull();
  });
});
