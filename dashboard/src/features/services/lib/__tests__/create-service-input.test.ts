import { describe, expect, it } from "vitest";
import {
  buildCreateServiceInput,
  buildShape,
  createFormErrors,
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
    imageCommand: "",
    registryCredentialId: "rc-1",
    branch: "",
    rootDir: "",
    runtime: "node",
    buildCommand: "yarn build",
    startCommand: "yarn start",
    dockerfilePath: "./Dockerfile",
    dockerContext: "",
    staticBuildCommand: "",
    publishPath: "",
    buildFilterPaths: [],
    buildFilterIgnored: [],
    plan: "starter",
    port: "3000",
    autoDeploy: true,
    schedule: "",
    command: "",
    healthCheckPath: "",
    preDeployCommand: "",
    disk: null,
    envGroups: [],
    envVars: [],
    secretFiles: [],
    projectId: null,
    environmentId: null,
    ...overrides,
  };
}

describe("buildCreateServiceInput", () => {
  it.each([
    "web_service",
    "static_site",
    "cron_job",
    "background_worker",
    "private_service",
  ] as const)(
    "submits Environment assignment but not the Project filter for %s",
    (serviceType) => {
      const ungrouped = buildCreateServiceInput(
        form({ serviceType, projectId: "prj-selected" }),
      );
      expect(ungrouped.environmentId).toBeUndefined();
      expect(ungrouped).not.toHaveProperty("projectId");
      const assigned = buildCreateServiceInput(
        form({
          serviceType,
          projectId: "prj-selected",
          environmentId: "evm-selected",
        }),
      );
      expect(assigned.environmentId).toBe("evm-selected");
      expect(assigned).not.toHaveProperty("projectId");
    },
  );

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

  // w4/226: a Root Directory typed on a Git tab stays in the draft after
  // switching to Existing Image, where no control shows it; the image create
  // must not send it (the API refuses rootDirectory on a prebuilt image).
  it.each([
    "web_service",
    "private_service",
    "background_worker",
    "cron_job",
  ] as const)(
    "omits a hidden Git Root Directory from a %s image create",
    (serviceType) => {
      const input = buildCreateServiceInput(
        form({
          serviceType,
          tab: "image",
          image: "docker.io/library/busybox:1.37",
          imageCommand: "httpd -f",
          rootDir: "examples/hello-go",
          schedule: serviceType === "cron_job" ? "*/5 * * * *" : "",
        }),
      );
      const wire = JSON.parse(JSON.stringify(input));
      expect(wire).not.toHaveProperty("rootDir");
      expect(wire).toMatchObject({
        image: "docker.io/library/busybox:1.37",
        runtime: "image",
      });
      if (serviceType !== "cron_job") {
        expect(wire.startCommand).toBe("httpd -f");
      }
    },
  );

  it.each([
    ["git native", { tab: "git" as const }],
    ["github docker", { tab: "github" as const, runtime: "docker" as const }],
    [
      "git static",
      { tab: "git" as const, serviceType: "static_site" as const },
    ],
  ])("keeps Root Directory for a %s build", (_name, overrides) => {
    expect(
      buildCreateServiceInput(form({ ...overrides, rootDir: "apps/api" }))
        .rootDir,
    ).toBe("apps/api");
  });

  it("carries the image source with no git fields and no autoDeploy", () => {
    const input = buildCreateServiceInput(
      form({ tab: "image", image: " nginx:1 " }),
    );
    expect(input.image).toBe("nginx:1");
    expect(input.repo).toBeUndefined();
    expect(input.runtime).toBe("image");
    expect(input.autoDeploy).toBeUndefined();
    expect(input.startCommand).toBeUndefined();
  });

  it("sends an image's Docker Command as startCommand, never the git start command", () => {
    const withCommand = buildCreateServiceInput(
      form({
        tab: "image",
        image: "hashicorp/http-echo:0.2.3",
        serviceType: "web_service",
        imageCommand: "  /http-echo -text=hello  ",
      }),
    );
    // A nonblank command ships as typed, padding included (w8/m68).
    expect(withCommand.startCommand).toBe("  /http-echo -text=hello  ");

    for (const serviceType of [
      "private_service",
      "background_worker",
    ] as const) {
      expect(
        buildCreateServiceInput(
          form({
            tab: "image",
            image: "alpine:3.20",
            serviceType,
            imageCommand: "tail -f /dev/null",
          }),
        ).startCommand,
      ).toBe("tail -f /dev/null");
    }

    // Blank sends no key; the git tab's runtime default never leaks into it.
    expect(
      buildCreateServiceInput(
        form({ tab: "image", image: "nginx:1", imageCommand: "   " }),
      ).startCommand,
    ).toBeUndefined();
    // A cron image sets its command through `command`, not startCommand.
    const cron = buildCreateServiceInput(
      form({
        tab: "image",
        image: "alpine:3.20",
        serviceType: "cron_job",
        imageCommand: "ignored",
        command: "echo tick",
        schedule: "*/5 * * * *",
      }),
    );
    expect(cron.startCommand).toBeUndefined();
    expect(cron.command).toBe("echo tick");
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

describe("isSubmittable duplicates (w21/015)", () => {
  it("refuses a repeated env key or secret-file name, trimmed", () => {
    const dup = [
      { key: "ALPHA", value: "one" },
      { key: "ALPHA", value: "two" },
    ];
    expect(isSubmittable(form({ envVars: dup }))).toBe(false);
    expect(
      isSubmittable(
        form({
          secretFiles: [
            { name: "dup.txt", content: "a" },
            { name: "dup.txt", content: "b" },
          ],
        }),
      ),
    ).toBe(false);
  });

  it("keeps case-different keys and blank placeholders submittable", () => {
    expect(
      isSubmittable(
        form({
          envVars: [
            { key: "ALPHA", value: "1" },
            { key: "Alpha", value: "2" },
            { key: "", value: "" },
            { key: "", value: "" },
          ],
        }),
      ),
    ).toBe(true);
  });
});

describe("isSubmittable", () => {
  // w4/406: the Existing Image path holds submit for a reference the server
  // would refuse, instead of a toast-only round trip.
  it.each([
    "not a valid ref!!",
    "Nginx:latest",
    "localhost:5000/x:1",
    "169.254.169.254/x:1",
  ])("holds an image create for %s", (image) => {
    expect(isSubmittable(form({ tab: "image", image }))).toBe(false);
  });

  it.each(["ghcr.io/traefik/whoami:v1.10", `nginx@sha256:${"a".repeat(64)}`])(
    "allows an image create for %s",
    (image) => {
      expect(isSubmittable(form({ tab: "image", image }))).toBe(true);
    },
  );

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

  // w5/m149: Render lets a service set PORT, so the wizard submits it.
  it("accepts a PORT env var like any other key", () => {
    expect(
      isSubmittable(form({ envVars: [{ key: "PORT", value: "8080" }] })),
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
        form({
          plan: "",
          serviceType: "static_site",
          buildCommand: "",
          publishPath: "dist",
        }),
      ),
    ).toBe(true);
  });

  // bex-api refuses a static site without a publish directory; the builder
  // trims a blank one away, so an empty draft must not reach Submit (w4/332).
  it("requires a nonblank publish directory for a static site only", () => {
    const site = (publishPath: string) =>
      form({ serviceType: "static_site", plan: "", publishPath });
    expect(isSubmittable(site(""))).toBe(false);
    expect(isSubmittable(site("   "))).toBe(false);
    for (const path of ["dist", ".", "examples/static-site", "  build  "]) {
      expect(isSubmittable(site(path))).toBe(true);
    }
    // Hidden for every other type, so a blank one never blocks them.
    for (const serviceType of [
      "web_service",
      "private_service",
      "background_worker",
    ] as const) {
      expect(isSubmittable(form({ serviceType, publishPath: "" }))).toBe(true);
    }
    expect(
      isSubmittable(
        form({
          serviceType: "cron_job",
          schedule: "0 * * * *",
          command: "run",
          publishPath: "",
        }),
      ),
    ).toBe(true);
  });

  it("rejects malformed env keys and secret-file names", () => {
    expect(
      isSubmittable(form({ envVars: [{ key: "BAD/KEY", value: "x" }] })),
    ).toBe(false);
    expect(
      isSubmittable(form({ secretFiles: [{ name: "../esc", content: "x" }] })),
    ).toBe(false);
  });
  // w4/m121/t003. The payload is the last gate on the portless types: they must
  // never carry a port, which bex-api refuses for them anyway.
  it("sends the port only for the types that bind one", () => {
    expect(
      buildCreateServiceInput(
        form({ serviceType: "web_service", port: "8080" }),
      ).port,
    ).toBe(8080);
    expect(
      buildCreateServiceInput(
        form({ serviceType: "private_service", port: "8080" }),
      ).port,
    ).toBe(8080);
    for (const serviceType of [
      "background_worker",
      "cron_job",
      "static_site",
    ] as const) {
      expect(
        buildCreateServiceInput(form({ serviceType, port: "8080" })).port,
      ).toBeUndefined();
    }
  });

  // w6/m148 (w6/083): a Git-built Docker private service publishes the ports
  // its image declares. An unedited or cleared port sends nothing, so bex-api
  // reads the image instead of pinning the primary to 3000. Every other
  // port-binding shape keeps the pre-filled 3000 (w4/m121/t003).
  it("leaves an image-port private service's port to its image unless one is typed", () => {
    const dockerPrivate = {
      serviceType: "private_service",
      runtime: "docker",
    } as const;
    for (const port of [null, "", "  "]) {
      const draft = form({ ...dockerPrivate, port });
      expect(isSubmittable(draft)).toBe(true);
      expect(buildCreateServiceInput(draft).port).toBeUndefined();
    }
    expect(
      buildCreateServiceInput(form({ ...dockerPrivate, port: "8123" })).port,
    ).toBe(8123);
    expect(isSubmittable(form({ ...dockerPrivate, port: "abc" }))).toBe(false);
  });

  // w4/m243 t010: a Docker-built or prebuilt-image web service takes its port
  // from the image too, as bex-api's create stamps it; a typed port is still
  // sent (the configured variant). w4/m323: so does a prebuilt-image private
  // service, as bex-api has since w6/095; sending the pre-filled 10000 pinned
  // it to a port whoami never binds, and its first deploy never went live.
  it.each([
    [
      "a Git Docker web service",
      { serviceType: "web_service", runtime: "docker" },
    ],
    [
      "an existing-image web service",
      {
        serviceType: "web_service",
        tab: "image",
        image: "docker.io/traefik/whoami:latest",
      },
    ],
    [
      "an existing-image private service",
      {
        serviceType: "private_service",
        tab: "image",
        image: "docker.io/traefik/whoami:v1.10.1",
      },
    ],
  ] as const)(
    "leaves %s's port to its image unless one is typed",
    (_name, overrides) => {
      for (const port of [null, "", "  "]) {
        const draft = form({ ...overrides, port });
        expect(isSubmittable(draft)).toBe(true);
        expect(buildCreateServiceInput(draft)).not.toHaveProperty(
          "port",
          expect.anything(),
        );
        expect(buildCreateServiceInput(draft).port).toBeUndefined();
      }
      expect(
        buildCreateServiceInput(form({ ...overrides, port: "8080" })).port,
      ).toBe(8080);
    },
  );

  it.each([
    ["a native web service", { serviceType: "web_service" }],
    ["a native private service", { serviceType: "private_service" }],
  ] as const)("still sends the pre-filled 10000 for %s", (_name, overrides) => {
    const draft = form({ ...overrides, port: null });
    expect(isSubmittable(draft)).toBe(true);
    expect(buildCreateServiceInput(draft).port).toBe(10000);
    expect(isSubmittable(form({ ...overrides, port: "" }))).toBe(false);
  });

  it("refuses a port outside 1-65535, and accepts one below 1024 (w6/m149)", () => {
    expect(isSubmittable(form({ port: "80" }))).toBe(true);
    expect(isSubmittable(form({ port: "0" }))).toBe(false);
    expect(isSubmittable(form({ port: "70000" }))).toBe(false);
    expect(isSubmittable(form({ port: "" }))).toBe(false);
    expect(isSubmittable(form({ port: "8080" }))).toBe(true);
    // A portless type never reads the field, so a stale draft can't block it.
    expect(
      isSubmittable(
        form({ serviceType: "background_worker", port: "nonsense" }),
      ),
    ).toBe(true);
  });
});

describe("command drafts on create (w8/m68)", () => {
  const cmd = "printf '%s' tail\\ ";
  const BLANK = " \t ";
  const SCHEDULE = "*/5 * * * *";

  it("keeps a native build's Build and Start Commands", () => {
    const input = buildCreateServiceInput(
      form({ buildCommand: cmd, startCommand: cmd }),
    );
    expect(input.buildCommand).toBe(cmd);
    expect(input.startCommand).toBe(cmd);
  });

  it("keeps a Dockerfile build's Docker Command (startCommand)", () => {
    const input = buildCreateServiceInput(
      form({ runtime: "docker", startCommand: cmd }),
    );
    expect(input.startCommand).toBe(cmd);
  });

  it("keeps an Existing Image's Docker Command (startCommand)", () => {
    const input = buildCreateServiceInput(
      form({ tab: "image", image: "alpine:3.20", imageCommand: cmd }),
    );
    expect(input.startCommand).toBe(cmd);
  });

  it("keeps a static site's Build Command", () => {
    const input = buildCreateServiceInput(
      form({ serviceType: "static_site", staticBuildCommand: cmd }),
    );
    expect(input.buildCommand).toBe(cmd);
  });

  it("keeps a native cron job's Command in both startCommand and command", () => {
    const input = buildCreateServiceInput(
      form({ serviceType: "cron_job", schedule: SCHEDULE, command: cmd }),
    );
    expect(input.startCommand).toBe(cmd);
    expect(input.command).toBe(cmd);
  });

  it("keeps an image cron job's Command", () => {
    const input = buildCreateServiceInput(
      form({
        serviceType: "cron_job",
        tab: "image",
        image: "alpine:3.20",
        schedule: SCHEDULE,
        command: cmd,
      }),
    );
    expect(input.command).toBe(cmd);
    expect(input.startCommand).toBeUndefined();
  });

  it("omits a whitespace-only optional command exactly as an empty one", () => {
    for (const draft of ["", BLANK]) {
      expect(
        buildCreateServiceInput(form({ startCommand: draft })).startCommand,
      ).toBeUndefined();
      expect(
        buildCreateServiceInput(
          form({ runtime: "docker", startCommand: draft }),
        ).startCommand,
      ).toBeUndefined();
      expect(
        buildCreateServiceInput(
          form({ tab: "image", image: "alpine:3.20", imageCommand: draft }),
        ).startCommand,
      ).toBeUndefined();
      expect(
        buildCreateServiceInput(
          form({ serviceType: "static_site", staticBuildCommand: draft }),
        ).buildCommand,
      ).toBeUndefined();
      const imageCron = buildCreateServiceInput(
        form({
          serviceType: "cron_job",
          tab: "image",
          image: "alpine:3.20",
          schedule: SCHEDULE,
          command: draft,
        }),
      );
      expect(imageCron.command).toBeUndefined();
    }
  });

  it("sends a whitespace-only required native command as the same empty string", () => {
    const build = buildCreateServiceInput(form({ buildCommand: BLANK }));
    expect(build.buildCommand).toBe("");
    const cron = buildCreateServiceInput(
      form({ serviceType: "cron_job", schedule: SCHEDULE, command: BLANK }),
    );
    expect(cron.startCommand).toBe("");
    expect(cron.command).toBeUndefined();
  });

  it("still treats a whitespace-only required native command as missing", () => {
    expect(isSubmittable(form({ buildCommand: BLANK }))).toBe(false);
    expect(isSubmittable(form({ startCommand: BLANK }))).toBe(false);
    expect(
      isSubmittable(
        form({ serviceType: "cron_job", schedule: SCHEDULE, command: BLANK }),
      ),
    ).toBe(false);
  });

  it("keeps trimming the Dockerfile path, publish directory, and schedule", () => {
    const docker = buildCreateServiceInput(
      form({ runtime: "docker", dockerfilePath: "  ./Dockerfile  " }),
    );
    expect(docker.dockerfilePath).toBe("./Dockerfile");
    const site = buildCreateServiceInput(
      form({ serviceType: "static_site", publishPath: "  dist  " }),
    );
    expect(site.publishPath).toBe("dist");
    const cron = buildCreateServiceInput(
      form({
        serviceType: "cron_job",
        schedule: `  ${SCHEDULE}  `,
        command: cmd,
      }),
    );
    expect(cron.schedule).toBe(SCHEDULE);
  });
});

// w21/050: the create form marks, before Deploy Service, what the server
// refuses and the Settings editors already mark (w21/045, w21/047), by the
// fields each build shape owns.
describe("createFormErrors", () => {
  it("marks a Root Directory that leaves the repository, for a Git source only", () => {
    expect(createFormErrors(form({ rootDir: "../x" })).rootDir).toBe(
      "services.buildPathDotDot",
    );
    expect(isSubmittable(form({ rootDir: "../x" }))).toBe(false);
    // The spellings the server normalizes still submit.
    for (const rootDir of ["", "./", "server/", "a//b"]) {
      expect(createFormErrors(form({ rootDir })).rootDir).toBeUndefined();
      expect(isSubmittable(form({ rootDir }))).toBe(true);
    }
    // An image source has no repository root: the draft survives a tab switch
    // but is never judged, as it never ships (w4/226).
    expect(
      createFormErrors(form({ tab: "image", image: "nginx", rootDir: "../x" }))
        .rootDir,
    ).toBeUndefined();
  });

  it("marks a Dockerfile Path only on a Docker build", () => {
    const docker = form({ runtime: "docker", dockerfilePath: "../Dockerfile" });
    expect(createFormErrors(docker).dockerfilePath).toBe(
      "services.buildPathDotDot",
    );
    expect(isSubmittable(docker)).toBe(false);
    expect(
      createFormErrors(
        form({ runtime: "node", dockerfilePath: "../Dockerfile" }),
      ).dockerfilePath,
    ).toBeUndefined();
    expect(
      isSubmittable(
        form({ runtime: "docker", dockerfilePath: "docker/Dockerfile" }),
      ),
    ).toBe(true);
  });

  it("marks a static site's publish directory that leaves the checkout", () => {
    const base = form({
      serviceType: "static_site",
      rootDir: "examples/hello-node",
    });
    expect(
      createFormErrors({ ...base, publishPath: "../../../x" }).publishPath,
    ).toBe("services.publishPathEscapes");
    expect(isSubmittable({ ...base, publishPath: "../../../x" })).toBe(false);
    // A path that stays inside the repository still saves (a separate decision).
    for (const publishPath of ["dist", ".", "/etc", "../static-site"]) {
      expect(
        createFormErrors({ ...base, publishPath }).publishPath,
      ).toBeUndefined();
      expect(isSubmittable({ ...base, publishPath })).toBe(true);
    }
    // An image-backed static site extracts from its image.
    expect(
      createFormErrors(
        form({
          serviceType: "static_site",
          tab: "image",
          image: "nginx",
          publishPath: "../x",
        }),
      ).publishPath,
    ).toBeUndefined();
  });

  it("marks the environment values the server refuses and holds Submit", () => {
    const row = (key: string, value: string) =>
      form({ envVars: [{ key, value }] });
    expect(createFormErrors(row("PORT", "abc")).envValues).toEqual({
      0: "portInvalid",
    });
    expect(createFormErrors(row("PORT", "18012")).envValues).toEqual({
      0: "portInvalid",
    });
    expect(
      createFormErrors(row("QA_BIG", "x".repeat(140000))).envValues,
    ).toEqual({
      0: "tooLarge",
    });
    expect(createFormErrors(row("K", "a\u0000b")).envValues).toEqual({
      0: "nul",
    });
    for (const invalid of [
      row("PORT", "abc"),
      row("QA_BIG", "x".repeat(140000)),
    ]) {
      expect(isSubmittable(invalid)).toBe(false);
    }
    expect(isSubmittable(row("PORT", "8080"))).toBe(true);
    expect(isSubmittable(row("QA_BIG", "x".repeat(100000)))).toBe(true);
    // The blank "add" placeholder is never judged.
    expect(createFormErrors(row("", "x".repeat(140000))).envValues).toEqual({});
  });

  // w4/658: the Environment editor's secret-files quota (1 MiB of contents in
  // UTF-8 bytes, names not counted; 500 files) holds Submit here too.
  it("marks secret files past the 1 MiB total and holds Submit", () => {
    const files = (...contents: string[]) =>
      form({
        secretFiles: contents.map((content, i) => ({
          name: `f${i}.txt`,
          content,
        })),
      });
    const half = "x".repeat(512 * 1024);
    expect(createFormErrors(files(half, half)).secretFilesLimit).toBe(false);
    expect(isSubmittable(files(half, half))).toBe(true);
    expect(createFormErrors(files(half, half + "x")).secretFilesLimit).toBe(
      true,
    );
    expect(isSubmittable(files(half, half + "x"))).toBe(false);
    // Bytes, not characters: "é" is two.
    expect(
      createFormErrors(files("é".repeat(512 * 1024 + 1))).secretFilesLimit,
    ).toBe(true);
    expect(
      createFormErrors(files(...Array.from({ length: 501 }, () => "x")))
        .secretFilesLimit,
    ).toBe(true);
    // The blank "add" placeholder is never counted.
    expect(
      createFormErrors(
        form({
          secretFiles: [
            { name: "a.txt", content: half + half },
            { name: "", content: "x" },
          ],
        }),
      ).secretFilesLimit,
    ).toBe(false);
  });

  it("holds Submit for an invalid Build Filter pattern on a static build", () => {
    const base = form({ serviceType: "static_site", publishPath: "dist" });
    expect(
      createFormErrors({ ...base, buildFilterPaths: ["["] }).buildFilter,
    ).toBe(true);
    expect(isSubmittable({ ...base, buildFilterPaths: ["["] })).toBe(false);
    expect(
      isSubmittable({
        ...base,
        buildFilterPaths: ["src/**"],
        buildFilterIgnored: ["docs/**"],
      }),
    ).toBe(true);
  });
});

describe("Build Source service creation (w2/m612 t003)", () => {
  const nativeGitSource = {
    id: "bds-native",
    name: "Native Mono",
    source: "https://github.com/acme/mono",
    kind: "git" as const,
    branch: "main",
    region: "oregon",
    nativeRuntime: "node" as const,
  };
  const dockerGitSource = {
    id: "bds-docker",
    name: "Docker Mono",
    source: "https://github.com/acme/docker",
    kind: "git" as const,
    branch: "main",
    region: "frankfurt",
    nativeRuntime: null,
  };
  const imageSource = {
    id: "bds-image",
    name: "Prebuilt Image",
    source: "ghcr.io/acme/api:1",
    kind: "image" as const,
    branch: null,
    region: null,
    nativeRuntime: null,
  };

  it("emits buildSourceId, region, and startCommand for a native git Build Source while omitting source-owned fields", () => {
    const f = form({
      tab: "build-source",
      buildSourceId: nativeGitSource.id,
      selectedBuildSource: nativeGitSource,
      buildSourceStartCommand: "node api.js",
      rootDir: "apps/api",
      dockerfilePath: "Dockerfile",
      registryCredentialId: "rc-1",
    });
    expect(isSubmittable(f)).toBe(true);
    const input = buildCreateServiceInput(f);
    expect(input).toMatchObject({
      buildSourceId: "bds-native",
      region: "oregon",
      startCommand: "node api.js",
      autoDeploy: true,
    });
    expect(input.repo).toBeUndefined();
    expect(input.branch).toBeUndefined();
    expect(input.image).toBeUndefined();
    expect(input.runtime).toBeUndefined();
    expect(input.buildCommand).toBeUndefined();
    expect(input.dockerCommand).toBeUndefined();
    expect(input.rootDir).toBeUndefined();
    expect(input.dockerfilePath).toBeUndefined();
    expect(input.registryCredentialId).toBeUndefined();

    // Missing startCommand blocks submit on native Build Source.
    expect(isSubmittable({ ...f, buildSourceStartCommand: "  " })).toBe(false);
  });

  it("emits dockerCommand (never startCommand) for Docker/image Build Sources and allows blank port/command", () => {
    const dockerForm = form({
      tab: "build-source",
      buildSourceId: dockerGitSource.id,
      selectedBuildSource: dockerGitSource,
      buildSourceDockerCommand: "./serve --port 8080",
      port: null,
    });
    expect(isSubmittable(dockerForm)).toBe(true);
    const dockerIn = buildCreateServiceInput(dockerForm);
    expect(dockerIn).toMatchObject({
      buildSourceId: "bds-docker",
      region: "frankfurt",
      dockerCommand: "./serve --port 8080",
    });
    expect(dockerIn.startCommand).toBeUndefined();
    expect(dockerIn.port).toBeUndefined();

    const imgForm = form({
      tab: "build-source",
      buildSourceId: imageSource.id,
      selectedBuildSource: imageSource,
      buildSourceDockerCommand: "",
      port: null,
    });
    expect(isSubmittable(imgForm)).toBe(true);
    const imgIn = buildCreateServiceInput(imgForm);
    expect(imgIn.buildSourceId).toBe("bds-image");
    expect(imgIn.region).toBeUndefined();
    expect(imgIn.dockerCommand).toBeUndefined();
    expect(imgIn.startCommand).toBeUndefined();
  });

  it("maps cron commands cleanly: command for native Build Source, dockerCommand for Docker/image Build Source", () => {
    const nativeCron = form({
      serviceType: "cron_job",
      tab: "build-source",
      schedule: "*/5 * * * *",
      buildSourceId: nativeGitSource.id,
      selectedBuildSource: nativeGitSource,
      command: "node job.js",
    });
    expect(isSubmittable(nativeCron)).toBe(true);
    expect(isSubmittable({ ...nativeCron, command: "" })).toBe(false);
    expect(buildCreateServiceInput(nativeCron)).toMatchObject({
      buildSourceId: "bds-native",
      command: "node job.js",
      dockerCommand: undefined,
      startCommand: undefined,
    });

    const imgCron = form({
      serviceType: "cron_job",
      tab: "build-source",
      schedule: "0 * * * *",
      buildSourceId: imageSource.id,
      selectedBuildSource: imageSource,
      command: "date",
    });
    expect(isSubmittable(imgCron)).toBe(true);
    expect(isSubmittable({ ...imgCron, command: "" })).toBe(true);
    expect(buildCreateServiceInput(imgCron)).toMatchObject({
      buildSourceId: "bds-image",
      dockerCommand: "date",
      command: undefined,
      startCommand: undefined,
    });
  });

  it("never emits buildSourceId for a static_site", () => {
    const f = form({
      serviceType: "static_site",
      tab: "build-source",
      buildSourceId: dockerGitSource.id,
      selectedBuildSource: dockerGitSource,
      publishPath: "dist",
    });
    expect(isSubmittable(f)).toBe(false);
    expect(buildCreateServiceInput(f).buildSourceId).toBeUndefined();
  });
});

// w4/m651: the Advanced card's Health Check Path, Pre-Deploy Command, Docker
// Build Context Directory and Build Filters, gated as Render's create form
// gates them (docs/render-artifacts/new-service-wizard.md § The Advanced
// section) and as bex-api refuses them.
describe("Advanced fields at create (w4/m651)", () => {
  const imageForm = (overrides: Partial<NewServiceForm> = {}) =>
    form({ tab: "image", image: "nginx:alpine", ...overrides });
  const buildSourceForm = (overrides: Partial<NewServiceForm> = {}) =>
    form({
      tab: "build-source",
      buildSourceId: "bds-image",
      selectedBuildSource: {
        id: "bds-image",
        name: "Prebuilt Image",
        source: "ghcr.io/acme/api:1",
        kind: "image",
        branch: null,
        region: null,
        nativeRuntime: null,
      },
      ...overrides,
    });

  it("sends a web service's Health Check Path, from any source, and never another type's", () => {
    for (const f of [
      form({ healthCheckPath: "/healthz" }),
      imageForm({ healthCheckPath: "/healthz" }),
      buildSourceForm({ healthCheckPath: "/healthz" }),
    ]) {
      expect(buildCreateServiceInput(f).healthCheckPath).toBe("/healthz");
    }
    for (const serviceType of [
      "private_service",
      "background_worker",
      "cron_job",
      "static_site",
    ] as const) {
      const input = buildCreateServiceInput(
        form({
          serviceType,
          healthCheckPath: "/healthz",
          schedule: "*/5 * * * *",
          publishPath: "dist",
        }),
      );
      expect(input.healthCheckPath).toBeUndefined();
    }
  });

  it("sends the Pre-Deploy Command of a paid web, private or worker service, from any source", () => {
    for (const serviceType of [
      "web_service",
      "private_service",
      "background_worker",
    ] as const) {
      for (const f of [
        form({ serviceType, preDeployCommand: "npm run migrate" }),
        imageForm({ serviceType, preDeployCommand: "npm run migrate" }),
        buildSourceForm({ serviceType, preDeployCommand: "npm run migrate" }),
      ]) {
        expect(buildCreateServiceInput(f).preDeployCommand).toBe(
          "npm run migrate",
        );
      }
    }
  });

  it("never sends a Pre-Deploy Command for a cron job, a static site or a Free plan", () => {
    const cron = form({
      serviceType: "cron_job",
      schedule: "*/5 * * * *",
      command: "node job.js",
      preDeployCommand: "npm run migrate",
    });
    const site = form({
      serviceType: "static_site",
      publishPath: "dist",
      preDeployCommand: "npm run migrate",
    });
    const free = form({ plan: "free", preDeployCommand: "npm run migrate" });
    for (const f of [cron, site, free]) {
      expect(buildCreateServiceInput(f).preDeployCommand).toBeUndefined();
    }
    // Free disables the field but still lets the service deploy.
    expect(buildShape(free).preDeployPaidOnly).toBe(true);
    expect(isSubmittable(free)).toBe(true);
    // The kept draft ships again once a paid plan is chosen.
    expect(
      buildCreateServiceInput({ ...free, plan: "starter" }).preDeployCommand,
    ).toBe("npm run migrate");
  });

  it("omits blank values, and keeps a command's bytes as the build and start commands do", () => {
    const blank = buildCreateServiceInput(
      form({ healthCheckPath: "   ", preDeployCommand: " \t " }),
    );
    expect(blank.healthCheckPath).toBeUndefined();
    expect(blank.preDeployCommand).toBeUndefined();

    const set = buildCreateServiceInput(
      form({
        healthCheckPath: " /healthz ",
        preDeployCommand: "printf '%s' tail\\ ",
      }),
    );
    expect(set.healthCheckPath).toBe("/healthz");
    expect(set.preDeployCommand).toBe("printf '%s' tail\\ ");
  });

  it("holds Submit on a Health Check Path that does not start with /, with Settings' reason", () => {
    const bad = form({ healthCheckPath: "healthz" });
    expect(createFormErrors(bad).healthCheckPath).toBe(
      "services.healthCheckPathSlash",
    );
    expect(isSubmittable(bad)).toBe(false);
    expect(isSubmittable(form({ healthCheckPath: "/healthz" }))).toBe(true);
    // A draft left under a type with no HTTP check is never judged.
    const worker = form({
      serviceType: "background_worker",
      healthCheckPath: "healthz",
    });
    expect(createFormErrors(worker).healthCheckPath).toBeUndefined();
    expect(isSubmittable(worker)).toBe(true);
  });

  it("sends a Docker build's Docker Build Context Directory and refuses one that leaves the repository", () => {
    const docker = form({ runtime: "docker", dockerContext: " ./server " });
    expect(buildCreateServiceInput(docker).dockerContext).toBe("./server");
    expect(
      buildCreateServiceInput({ ...docker, dockerContext: "" }).dockerContext,
    ).toBeUndefined();
    expect(
      buildCreateServiceInput({ ...docker, runtime: "node" }).dockerContext,
    ).toBeUndefined();
    expect(
      buildCreateServiceInput(imageForm({ dockerContext: "server" }))
        .dockerContext,
    ).toBeUndefined();

    const escaping = { ...docker, dockerContext: "../other" };
    expect(createFormErrors(escaping).dockerContext).toBe(
      "services.buildPathDotDot",
    );
    expect(isSubmittable(escaping)).toBe(false);
    // Not judged once the build is native again.
    expect(isSubmittable({ ...escaping, runtime: "node" })).toBe(true);
  });

  it("sends Build Filters for every repository-backed type, and none for an image or a Build Source", () => {
    const filters = {
      buildFilterPaths: ["src/**"],
      buildFilterIgnored: ["docs/**"],
    };
    for (const f of [
      form(filters),
      form({ ...filters, serviceType: "private_service" }),
      form({ ...filters, serviceType: "background_worker" }),
      form({
        ...filters,
        serviceType: "cron_job",
        schedule: "*/5 * * * *",
        command: "node job.js",
      }),
      form({ ...filters, serviceType: "static_site", publishPath: "dist" }),
    ]) {
      expect(buildCreateServiceInput(f).buildFilter).toEqual({
        paths: ["src/**"],
        ignoredPaths: ["docs/**"],
      });
    }
    expect(buildCreateServiceInput(imageForm(filters)).buildFilter).toBe(
      undefined,
    );
    expect(buildCreateServiceInput(buildSourceForm(filters)).buildFilter).toBe(
      undefined,
    );
    // A bad pattern holds a web service's Submit too, and only where shown.
    expect(isSubmittable(form({ buildFilterPaths: ["["] }))).toBe(false);
    expect(isSubmittable(imageForm({ buildFilterPaths: ["["] }))).toBe(true);
  });
});

// w4/m652: Render's create form attaches a disk to a paid web service,
// private service or background worker, so its first deploy mounts it
// (docs/render-artifacts/new-service-wizard.md § The Advanced section).
describe("Disk at create (w4/m652)", () => {
  const disk = { mountPath: " /var/data ", sizeGB: 5 };
  const imageForm = (overrides: Partial<NewServiceForm> = {}) =>
    form({ tab: "image", image: "nginx:alpine", ...overrides });

  it("sends a drafted disk once, named as Render's dashboard names it, for a paid web, private or worker service", () => {
    for (const serviceType of [
      "web_service",
      "private_service",
      "background_worker",
    ] as const) {
      for (const f of [
        form({ serviceType, disk }),
        imageForm({ serviceType, disk }),
      ]) {
        expect(buildShape(f).showDisk).toBe(true);
        expect(buildCreateServiceInput(f).disk).toEqual({
          name: "disk",
          mountPath: "/var/data",
          sizeGB: 5,
        });
        expect(isSubmittable(f)).toBe(true);
      }
    }
  });

  it("sends no disk when none is drafted", () => {
    expect(buildCreateServiceInput(form()).disk).toBeUndefined();
  });

  it("offers and sends no disk for a cron job, a static site, Free, or before a plan is known", () => {
    const cron = form({
      serviceType: "cron_job",
      schedule: "*/5 * * * *",
      command: "node job.js",
      disk,
    });
    const site = form({
      serviceType: "static_site",
      publishPath: "dist",
      disk,
    });
    const free = form({ plan: "free", disk });
    const unknownPlan = form({ plan: "", disk });
    for (const f of [cron, site, free, unknownPlan]) {
      expect(buildShape(f).showDisk).toBe(false);
      expect(buildCreateServiceInput(f).disk).toBeUndefined();
      // A disk the form no longer offers is never judged either.
      expect(
        createFormErrors({ ...f, disk: { mountPath: "", sizeGB: 0 } }),
      ).not.toHaveProperty("diskMountPath");
    }
    expect(isSubmittable(free)).toBe(true);
  });

  it.each([
    ["", "services.diskMountPathRequired"],
    ["var/data", "services.diskMountPathAbsolute"],
    ["/", "services.diskMountPathNotRoot"],
    ["/var/data/", "services.diskMountPathNotRoot"],
    ["/etc/secrets", "services.diskMountPathReserved"],
    ["/bex-health", "services.diskMountPathReserved"],
  ] as const)(
    "holds Submit on the mount path %j, as bex-api would refuse it",
    (mountPath, reason) => {
      const f = form({ disk: { mountPath, sizeGB: 10 } });
      expect(createFormErrors(f).diskMountPath).toBe(reason);
      expect(isSubmittable(f)).toBe(false);
    },
  );

  it("accepts a subdirectory of a reserved path", () => {
    const f = form({ disk: { mountPath: "/etc/app-data", sizeGB: 10 } });
    expect(createFormErrors(f).diskMountPath).toBeUndefined();
    expect(isSubmittable(f)).toBe(true);
  });

  it.each([0, 1.5, 10001, Number.NaN])(
    "holds Submit on the size %s GB, outside bex-api's whole 1-10000",
    (sizeGB) => {
      const f = form({ disk: { mountPath: "/var/data", sizeGB } });
      expect(createFormErrors(f).diskSize).toBe("services.diskSizeRange");
      expect(isSubmittable(f)).toBe(false);
    },
  );

  it.each([1, 10000])("accepts a %s GB disk", (sizeGB) => {
    const f = form({ disk: { mountPath: "/var/data", sizeGB } });
    expect(createFormErrors(f).diskSize).toBeUndefined();
    expect(buildCreateServiceInput(f).disk?.sizeGB).toBe(sizeGB);
  });
});

// w4/731: Render's create form links environment groups for every type and
// source, so the create sends them, in the order chosen, whatever the shape.
describe("buildCreateServiceInput envGroups", () => {
  const envGroups = ["evg-b", "evg-a"];

  it.each([
    ["a web service from Git", form({ envGroups })],
    [
      "a static site",
      form({ serviceType: "static_site", publishPath: "dist", envGroups }),
    ],
    [
      "a cron job",
      form({
        serviceType: "cron_job",
        schedule: "0 * * * *",
        command: "run",
        envGroups,
      }),
    ],
    ["an existing image", form({ tab: "image", image: "nginx:1", envGroups })],
  ])("sends %s's linked groups in their order", (_, f) => {
    expect(buildCreateServiceInput(f).envGroups).toEqual(envGroups);
  });

  it("sends none when no group is linked", () => {
    expect(buildCreateServiceInput(form()).envGroups).toBeUndefined();
  });
});
