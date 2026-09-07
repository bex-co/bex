import { describe, expect, it } from "vitest";
import {
  buildCreateServiceInput,
  isSubmittable,
  type NewServiceForm,
} from "../create-service-input";

function form(overrides: Partial<NewServiceForm> = {}): NewServiceForm {
  return {
    name: "svc",
    serviceType: "web_service",
    tab: "git",
    selectedRepo: null,
    gitUrl: "https://github.com/acme/app.git",
    image: "",
    registryCredentialId: "rc-1",
    branch: "",
    rootDir: "",
    runtime: "node",
    buildCommand: "yarn build",
    startCommand: "yarn start",
    dockerfilePath: "./Dockerfile",
    staticBuildCommand: "",
    publishPath: "",
    buildFilterPaths: [],
    buildFilterIgnored: [],
    plan: "starter",
    autoDeploy: true,
    schedule: "",
    command: "",
    envVars: [],
    secretFiles: [],
    projectId: null,
    environmentId: null,
    ...overrides,
  };
}

describe("buildCreateServiceInput", () => {
  it("omits every field the current build shape does not own", () => {
    // A static site builds from git but has no runtime, dockerfile, or plan —
    // the regression the four shapes exist to prevent.
    const input = buildCreateServiceInput(
      form({ serviceType: "static_site", staticBuildCommand: "yarn build" }),
    );
    expect(input).toMatchObject({
      repo: "https://github.com/acme/app.git",
      buildCommand: "yarn build",
    });
    expect(input.dockerfilePath).toBeUndefined();
    expect(input.runtime).toBeUndefined();
    expect(input.startCommand).toBeUndefined();
    expect(input.plan).toBeUndefined();
  });

  it("sends a dockerfile path and registry credential only for a docker build", () => {
    const docker = buildCreateServiceInput(form({ runtime: "docker" }));
    expect(docker.dockerfilePath).toBe("./Dockerfile");
    expect(docker.registryCredentialId).toBe("rc-1");

    const native = buildCreateServiceInput(form());
    expect(native.dockerfilePath).toBeUndefined();
    expect(native.registryCredentialId).toBeUndefined();
  });

  it("carries the image source with no git fields and no autoDeploy", () => {
    const input = buildCreateServiceInput(
      form({ tab: "image", image: " nginx:1 " }),
    );
    expect(input.image).toBe("nginx:1");
    expect(input.repo).toBeUndefined();
    expect(input.runtime).toBe("image");
    expect(input.autoDeploy).toBeUndefined();
  });

  it("never emits an image source for a static site", () => {
    // Even if the form somehow holds tab === "image" for a static site (the
    // wizard hides that tab), the payload must not carry the impossible
    // static+image combination (ADR029; w8/m32).
    const input = buildCreateServiceInput(
      form({
        serviceType: "static_site",
        tab: "image",
        image: "nginx:1",
        publishPath: "dist",
      }),
    );
    expect(input.image).toBeUndefined();
    expect(input.runtime).toBeUndefined();
    expect(input.registryCredentialId).toBeUndefined();
  });

  it("prefers an explicit branch over the selected repo's default", () => {
    const repo = {
      cloneUrl: "https://github.com/acme/app.git",
      defaultBranch: "main",
    } as NewServiceForm["selectedRepo"];
    expect(
      buildCreateServiceInput(form({ tab: "github", selectedRepo: repo }))
        .branch,
    ).toBe("main");
    expect(
      buildCreateServiceInput(
        form({ tab: "github", selectedRepo: repo, branch: "next" }),
      ).branch,
    ).toBe("next");
  });

  it("drops the editors' blank placeholder rows", () => {
    const input = buildCreateServiceInput(
      form({
        envVars: [
          { key: "A", value: "1" },
          { key: "", value: "" },
        ],
        secretFiles: [{ name: "", content: "" }],
      }),
    );
    expect(input.envVars).toEqual([{ key: "A", value: "1" }]);
    expect(input.secretFiles).toEqual([]);
  });
});

describe("isSubmittable", () => {
  it("requires a valid source", () => {
    expect(isSubmittable(form())).toBe(true);
    expect(isSubmittable(form({ gitUrl: "not a url" }))).toBe(false);
    expect(isSubmittable(form({ tab: "image", image: "" }))).toBe(false);
  });

  it("never lets a static site submit through an image source", () => {
    // The image tab is unreachable for a static site in the wizard; this pins
    // the submit gate as the independent backstop (w8/m32).
    expect(
      isSubmittable(
        form({
          serviceType: "static_site",
          tab: "image",
          image: "docker.io/library/nginx:latest",
          publishPath: "dist",
          buildCommand: "",
        }),
      ),
    ).toBe(false);
    // The same image source stays submittable for an image-valid type.
    expect(
      isSubmittable(form({ tab: "image", image: "nginx:1", buildCommand: "" })),
    ).toBe(true);
  });

  it("requires build and start commands for a native build only", () => {
    expect(isSubmittable(form({ buildCommand: "" }))).toBe(false);
    expect(isSubmittable(form({ runtime: "docker", buildCommand: "" }))).toBe(
      true,
    );
  });

  it("requires a valid cron schedule for a cron job", () => {
    const cron = { serviceType: "cron_job", command: "./run" } as const;
    expect(isSubmittable(form({ ...cron }))).toBe(false);
    expect(isSubmittable(form({ ...cron, schedule: "nonsense" }))).toBe(false);
    expect(isSubmittable(form({ ...cron, schedule: "0 * * * *" }))).toBe(true);
  });

  it("requires a plan for everything but a static site", () => {
    expect(isSubmittable(form({ plan: "" }))).toBe(false);
    expect(
      isSubmittable(
        form({ plan: "", serviceType: "static_site", buildCommand: "" }),
      ),
    ).toBe(true);
  });

  it("rejects malformed env keys and secret-file names", () => {
    expect(
      isSubmittable(form({ envVars: [{ key: "1BAD", value: "x" }] })),
    ).toBe(false);
    expect(
      isSubmittable(form({ secretFiles: [{ name: "../esc", content: "x" }] })),
    ).toBe(false);
  });
});
