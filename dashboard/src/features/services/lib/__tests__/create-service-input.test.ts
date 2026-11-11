import { describe, expect, it } from "vitest";
import {
  buildCreateServiceInput,
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
    staticBuildCommand: "",
    publishPath: "",
    buildFilterPaths: [],
    buildFilterIgnored: [],
    plan: "starter",
    port: "3000",
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
