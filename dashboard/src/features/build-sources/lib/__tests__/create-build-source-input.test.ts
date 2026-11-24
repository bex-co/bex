import { describe, expect, it } from "vitest";
import { CombinedGraphQLErrors } from "@apollo/client/errors";
import {
  buildCreateBuildSourceParams,
  extractPartiallyLinkedError,
  getCreateBuildSourceFieldErrors,
  isCreateBuildSourceSubmittable,
  isLinkableServiceType,
  suggestBuildSourceName,
  type CreateBuildSourceFormState,
} from "../create-build-source-input";

function baseForm(
  overrides: Partial<CreateBuildSourceFormState> = {},
): CreateBuildSourceFormState {
  return {
    tab: "git",
    selectedRepo: null,
    gitUrl: "https://github.com/acme/monorepo",
    imageUrl: "",
    imageRegistryCredentialId: "",
    name: "shared-build",
    projectId: null,
    runtime: "go",
    branch: "main",
    region: "frankfurt",
    buildCommand: "go build -o app .",
    rootDir: "services/api",
    dockerfilePath: "Dockerfile.prod",
    dockerContext: "docker",
    gitRegistryCredentialId: "rgc-1",
    buildFilterPaths: ["services/api/**"],
    buildFilterIgnored: ["**/*.md"],
    envVars: [{ key: "CGO_ENABLED", value: "0" }],
    secretFiles: [{ name: "npmrc", content: "//registry.npmjs.org/:_authToken=x" }],
    envGroupIds: ["evg-1"],
    serviceIds: ["srv-1"],
    ...overrides,
  };
}

describe("buildCreateBuildSourceParams", () => {
  it("omits Docker-only fields for native git runtimes", () => {
    const params = buildCreateBuildSourceParams(baseForm({ runtime: "go" }));
    expect(params).toEqual({
      name: "shared-build",
      serviceIds: ["srv-1"],
      git: {
        repoUrl: "https://github.com/acme/monorepo",
        runtime: "go",
        branch: "main",
        region: "frankfurt",
        rootDir: "services/api",
        buildCommand: "go build -o app .",
        buildFilter: {
          paths: ["services/api/**"],
          ignoredPaths: ["**/*.md"],
        },
      },
      envVars: [{ key: "CGO_ENABLED", value: "0" }],
      secretFiles: [
        { name: "npmrc", content: "//registry.npmjs.org/:_authToken=x" },
      ],
      envGroupIds: ["evg-1"],
    });
    expect(params.image).toBeUndefined();
    expect(params.git?.dockerContext).toBeUndefined();
    expect(params.git?.dockerfilePath).toBeUndefined();
    expect(params.git?.registryCredentialId).toBeUndefined();
  });

  it("includes dockerfilePath, dockerContext, and registryCredentialId and omits buildCommand for docker runtime", () => {
    const params = buildCreateBuildSourceParams(
      baseForm({ runtime: "docker", projectId: "prj-9" }),
    );
    expect(params.projectId).toBe("prj-9");
    expect(params.git).toEqual({
      repoUrl: "https://github.com/acme/monorepo",
      runtime: "docker",
      branch: "main",
      region: "frankfurt",
      rootDir: "services/api",
      dockerfilePath: "Dockerfile.prod",
      dockerContext: "docker",
      registryCredentialId: "rgc-1",
      buildFilter: {
        paths: ["services/api/**"],
        ignoredPaths: ["**/*.md"],
      },
    });
    expect(params.git?.buildCommand).toBeUndefined();
  });

  it("omits git and all build-time environment fields for Docker image sources", () => {
    const params = buildCreateBuildSourceParams(
      baseForm({
        tab: "image",
        imageUrl: "ghcr.io/acme/api:v2",
        imageRegistryCredentialId: "rgc-img",
      }),
    );
    expect(params).toEqual({
      name: "shared-build",
      serviceIds: ["srv-1"],
      image: {
        imageUrl: "ghcr.io/acme/api:v2",
        registryCredentialId: "rgc-img",
      },
    });
    expect(params.git).toBeUndefined();
    expect(params.envVars).toBeUndefined();
    expect(params.secretFiles).toBeUndefined();
    expect(params.envGroupIds).toBeUndefined();
  });
});

describe("validation helpers", () => {
  it("flags invalid dockerContext only when runtime is docker", () => {
    const nativeErrs = getCreateBuildSourceFieldErrors(
      baseForm({ runtime: "node", dockerContext: "../escape" }),
    );
    expect(nativeErrs.dockerContext).toBeNull();

    const dockerErrs = getCreateBuildSourceFieldErrors(
      baseForm({ runtime: "docker", dockerContext: "../escape" }),
    );
    expect(dockerErrs.dockerContext).not.toBeNull();
    expect(
      isCreateBuildSourceSubmittable(
        baseForm({ runtime: "docker", dockerContext: "../escape" }),
      ),
    ).toBe(false);
  });

  it("rejects static_site from linkable service types", () => {
    expect(isLinkableServiceType("web_service")).toBe(true);
    expect(isLinkableServiceType("private_service")).toBe(true);
    expect(isLinkableServiceType("background_worker")).toBe(true);
    expect(isLinkableServiceType("cron_job")).toBe(true);
    expect(isLinkableServiceType("static_site")).toBe(false);
    expect(isLinkableServiceType(null)).toBe(false);
  });

  it("suggests clean names from repo URLs and image refs", () => {
    expect(suggestBuildSourceName("https://github.com/acme/payments.git")).toBe(
      "payments",
    );
    expect(suggestBuildSourceName("ghcr.io/acme/worker:1.2.3")).toBe("worker");
    expect(suggestBuildSourceName("nginx@sha256:abcdef")).toBe("nginx");
  });

  it("extracts BUILD_SOURCE_PARTIALLY_LINKED structured extensions", () => {
    const err = new CombinedGraphQLErrors({
      errors: [
        {
          message: "conflict: partial link",
          extensions: {
            code: "BUILD_SOURCE_PARTIALLY_LINKED",
            buildSourceId: "bds-123",
            linkedServiceIds: ["srv-a", "srv-b"],
            failedServiceId: "srv-c",
          },
        },
      ],
    });
    expect(extractPartiallyLinkedError(err)).toEqual({
      buildSourceId: "bds-123",
      linkedServiceIds: ["srv-a", "srv-b"],
      failedServiceId: "srv-c",
      message: "Partial link",
    });
    expect(extractPartiallyLinkedError(new Error("boom"))).toBeNull();
  });
});
