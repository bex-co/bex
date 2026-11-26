import { describe, expect, it } from "vitest";
import {
  hostForRegistryError,
  registryForHost,
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
