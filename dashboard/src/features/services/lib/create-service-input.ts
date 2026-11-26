import { isValidGitUrl, parseGitHubPage } from "@/common/lib/utils/git-url";
import { isValidDockerImageRef } from "@/common/lib/utils/docker-image";
import { commandText } from "@/features/services/lib/command-text";
import { isValidCron } from "@/features/services/lib/cron";
import type { en } from "@/i18n";
import {
  buildFilterPatternError,
  healthCheckPathError,
  relativePathError,
  rootDirError,
} from "@/features/services/lib/build-settings-validation";
import { publishDirectoryError } from "@/features/services/lib/publish-directory";
import {
  DASHBOARD_DISK_NAME,
  diskAllowed,
  diskMountPathError,
  diskSizeError,
} from "@/features/services/lib/disk";
import {
  VALID_ENV_KEY,
  duplicateNames,
  envValueRuleError,
  isValidSecretFileName,
  secretFilesOverLimit,
} from "@/features/services/lib/environment-draft";
import type { ServiceType } from "@/features/services/lib/create-context";
import {
  isWebServiceType,
  servesHttp,
} from "@/features/services/lib/service-type";
import { DEFAULT_SERVICE_PORT, parsePort } from "@/features/services/lib/port";
import type { GitRuntime } from "@/features/services/lib/runtime";
import type { SourceTab } from "@/features/services/components/service-source-picker";
import type { BuildSourceOption } from "@/features/services/hooks/use-build-sources";
import type { RepoView } from "@/features/services/hooks/use-repos";
import type {
  CreateServiceInput,
  EnvVarEntry,
  SecretFileEntry,
} from "@/features/services/hooks/use-create-service";

/** Every field the create form collects — the input to both rules below. */
export interface NewServiceForm {
  name: string;
  serviceType: ServiceType;
  tab: SourceTab;
  selectedRepo: RepoView | null;
  gitUrl: string;
  image: string;
  /** Docker Command override for an Existing Image source (not cron). */
  imageCommand: string;
  /** Selected Build Source id (`bds-…`) when `tab === "build-source"`. */
  buildSourceId?: string;
  /** Resolved Build Source metadata for the selected `buildSourceId`. */
  selectedBuildSource?: BuildSourceOption | null;
  /** Start command for a native-runtime Build Source (non-cron). */
  buildSourceStartCommand?: string;
  /** Optional Docker command override for a Docker/image Build Source (non-cron). */
  buildSourceDockerCommand?: string;
  registryCredentialId: string;
  branch: string;
  rootDir: string;
  runtime: GitRuntime;
  buildCommand: string;
  startCommand: string;
  dockerfilePath: string;
  /** Docker Build Context Directory, relative to the Root Directory (Docker builds). */
  dockerContext: string;
  staticBuildCommand: string;
  publishPath: string;
  buildFilterPaths: string[];
  buildFilterIgnored: string[];
  plan: string;
  /**
   * Draft container port, as typed (w4/m121/t003); `null` until edited, which
   * shows the shape's default ({@link portDraft}). Only read for `showPort`
   * types.
   */
  port: string | null;
  autoDeploy: boolean;
  schedule: string;
  command: string;
  /** Health Check Path draft (web services). */
  healthCheckPath: string;
  /** Pre-Deploy Command draft (paid web, private and worker services). */
  preDeployCommand: string;
  /** The drafted disk, `null` until Add Disk is pressed (see `showDisk`). */
  disk: DiskDraft | null;
  envVars: EnvVarEntry[];
  secretFiles: SecretFileEntry[];
  projectId: string | null;
  environmentId: string | null;
}

/** A disk drafted on the create form: its mount path as typed, and its size. */
export interface DiskDraft {
  mountPath: string;
  sizeGB: number;
}

/**
 * The four build shapes the form can submit. Derived once and consumed by the
 * field JSX, the submit gate, and the payload alike: re-deriving any of these
 * conditions inline is how a static site ends up submitting a dockerfilePath.
 */
export interface BuildShape {
  isCronType: boolean;
  isStaticType: boolean;
  isImageSource: boolean;
  /** An Existing Image long-running service: its Docker Command override. */
  showImageCommand: boolean;
  /** Linking to an existing workspace Build Source (`tab === "build-source"`). */
  isBuildSource: boolean;
  /** Selected Build Source uses a native runtime (requires start/cron command). */
  isNativeBuildSource: boolean;
  /** Selected Build Source uses Docker or a prebuilt image (optional dockerCommand). */
  isContainerBuildSource: boolean;
  /** Docker/image Build Source long-running service: optional Docker Command override. */
  showBuildSourceDockerCommand: boolean;
  isGitSource: boolean;
  isBuildableGit: boolean;
  isDockerBuild: boolean;
  isNativeBuild: boolean;
  isStaticBuild: boolean;
  usesRegistryCredential: boolean;
  showPlan: boolean;
  /**
   * The type binds a port (`servesHttp`): web_service and private_service.
   * Gates BOTH the image tab's `$PORT` hint and the create form's port field
   * (w4/m121/t003) — one predicate, so a worker/cron/static site can never
   * render a port control or submit a `port`.
   */
  showPort: boolean;
  /**
   * A service whose image supplies its ports (a Docker build, prebuilt image,
   * or Docker/image Build Source): its port is optional, blank by default, and
   * only picks the primary when set. bex-api applies the same rule to a create
   * with no port.
   */
  imagePorts: boolean;
  /**
   * Render's Health Check Path: a web service of any source. Private services
   * get Render's TCP check, and the other types have none (bex-api refuses a
   * path for them, `errHealthCheckPathNotApplicable`).
   */
  showHealthCheckPath: boolean;
  /**
   * Render's Pre-Deploy Command: a web service, private service or background
   * worker of any source. A cron job and a static site run no pre-deploy step.
   */
  showPreDeployCommand: boolean;
  /**
   * The Pre-Deploy Command is shown but disabled: Render sells it on paid
   * plans only ("available for paid web services, private services, and
   * background workers", render.com/docs/deploys), and bex-api refuses one on
   * Free (`errPreDeployCommandFreePlan`). Its draft is kept, never sent.
   */
  preDeployPaidOnly: boolean;
  /**
   * Render's Disk: a paid web service, private service or background worker,
   * from any source. Render's create form shows no Disk on Free, for a cron
   * job or for a static site, and drops a drafted one when the service stops
   * qualifying (docs/render-artifacts/new-service-wizard.md § The Advanced
   * section); bex-api refuses one there too (`validateCreateDisk`).
   */
  showDisk: boolean;
}

/**
 * Whether a create with no port takes its ports from the image: bex-api stamps
 * an image-port mode for a web or private service built by Docker or deployed
 * from a prebuilt image (w4/m243, w6/m148, w6/095). A native runtime keeps the
 * configured port. The form must not send a port where none was chosen, or the
 * service is pinned to the default the image never binds (w4/m323).
 */
function imagePortShape(
  serviceType: NewServiceForm["serviceType"],
  isDockerBuild: boolean,
  isImageSource: boolean,
  isContainerBuildSource: boolean,
): boolean {
  return (
    (serviceType === "web_service" || serviceType === "private_service") &&
    (isDockerBuild || isImageSource || isContainerBuildSource)
  );
}

export function buildShape(form: NewServiceForm): BuildShape {
  const isCronType = form.serviceType === "cron_job";
  const isStaticType = form.serviceType === "static_site";
  // A static site has no image or Build Source (ADR029, ADR097) — even if the
  // form somehow holds tab === "image" or "build-source" for one, the shape
  // refuses to read it as such, so the submit gate and payload never emit it.
  const isImageSource = form.tab === "image" && !isStaticType;
  const isBuildSource = form.tab === "build-source" && !isStaticType;
  const selectedBuildSource = isBuildSource
    ? (form.selectedBuildSource ?? null)
    : null;
  const isNativeBuildSource =
    selectedBuildSource !== null && selectedBuildSource.nativeRuntime !== null;
  const isContainerBuildSource =
    selectedBuildSource !== null && selectedBuildSource.nativeRuntime === null;
  const isGitSource = form.tab === "github" || form.tab === "git";
  const isBuildableGit = isGitSource && !isStaticType;
  const isDockerBuild = isBuildableGit && form.runtime === "docker";
  const showPreDeployCommand = !isCronType && !isStaticType;
  return {
    isCronType,
    isStaticType,
    isImageSource,
    showImageCommand: isImageSource && !isCronType,
    isBuildSource,
    isNativeBuildSource,
    isContainerBuildSource,
    showBuildSourceDockerCommand: isContainerBuildSource && !isCronType,
    isGitSource,
    isBuildableGit,
    isDockerBuild,
    isNativeBuild: isBuildableGit && form.runtime !== "docker",
    isStaticBuild: isGitSource && isStaticType,
    usesRegistryCredential: isImageSource || isDockerBuild,
    showPlan: !isStaticType,
    showPort: servesHttp(form.serviceType),
    imagePorts: imagePortShape(
      form.serviceType,
      isDockerBuild,
      isImageSource,
      isContainerBuildSource,
    ),
    showHealthCheckPath: isWebServiceType(form.serviceType),
    showPreDeployCommand,
    preDeployPaidOnly: showPreDeployCommand && form.plan === "free",
    showDisk: diskAllowed(form.serviceType, form.plan),
  };
}

/**
 * The port field's value: what the user typed, else the shape's default — the
 * platform's 3000, pre-filled so it is visible before submit (w4/m121/t003),
 * or blank for an image-port service, whose ports come from its image.
 */
export function portDraft(form: NewServiceForm, shape: BuildShape): string {
  return form.port ?? (shape.imagePorts ? "" : String(DEFAULT_SERVICE_PORT));
}

/** Blank is valid only where the image supplies the ports; else 1-65535. */
export function portIsValid(form: NewServiceForm, shape: BuildShape): boolean {
  const draft = portDraft(form, shape);
  return (shape.imagePorts && draft.trim() === "") || parsePort(draft) !== null;
}

/** Blank means "leave it to the platform default", never an empty string. */
function set(value: string): string | undefined {
  return value.trim() || undefined;
}

/** {@link set} for a shell command: blank is omitted, else sent verbatim. */
function optionalCommand(value: string): string | undefined {
  return commandText(value) || undefined;
}

/** The rows the editors' blank "add" placeholder must not put on the wire. */
export function submittableEnvVars(rows: EnvVarEntry[]): EnvVarEntry[] {
  return rows.filter((row) => row.key !== "");
}

export function submittableSecretFiles(
  rows: SecretFileEntry[],
): SecretFileEntry[] {
  return rows.filter((file) => file.name !== "");
}

/**
 * What the create form marks in a field before the server refuses it, by the
 * rules the Settings editors mark too (w21/045, w21/047, w21/050): the build
 * shapes decide which fields the form owns, as they decide which ship. A hidden
 * field is never judged.
 */
export interface CreateFormErrors {
  rootDir?: keyof typeof en;
  dockerfilePath?: keyof typeof en;
  dockerContext?: keyof typeof en;
  publishPath?: keyof typeof en;
  healthCheckPath?: keyof typeof en;
  /** A drafted disk's mount path or size the server would refuse. */
  diskMountPath?: keyof typeof en;
  diskSize?: keyof typeof en;
  /** An invalid Build Filter pattern in either list holds Submit. */
  buildFilter: boolean;
  /** The row index of an environment variable whose value the server refuses. */
  envValues: Record<number, "nul" | "tooLarge" | "portInvalid">;
  /** The secret files break the Environment editor's quota (w4/658). */
  secretFilesLimit: boolean;
}

export function createFormErrors(form: NewServiceForm): CreateFormErrors {
  const shape = buildShape(form);
  const errors: CreateFormErrors = {
    buildFilter: false,
    envValues: {},
    secretFilesLimit: secretFilesOverLimit(
      submittableSecretFiles(form.secretFiles),
    ),
  };
  if (shape.isGitSource)
    errors.rootDir = rootDirError(form.rootDir) ?? undefined;
  if (shape.isDockerBuild) {
    errors.dockerfilePath = relativePathError(form.dockerfilePath) ?? undefined;
    errors.dockerContext = rootDirError(form.dockerContext) ?? undefined;
  }
  if (shape.showHealthCheckPath) {
    errors.healthCheckPath =
      healthCheckPathError(form.healthCheckPath) ?? undefined;
  }
  if (shape.showDisk && form.disk) {
    errors.diskMountPath =
      diskMountPathError(form.disk.mountPath.trim()) ?? undefined;
    errors.diskSize = diskSizeError(form.disk.sizeGB) ?? undefined;
  }
  if (shape.isStaticType) {
    errors.publishPath =
      publishDirectoryError(form.publishPath, {
        rootDir: shape.isGitSource ? form.rootDir : "",
        repoBacked: shape.isGitSource,
      }) ?? undefined;
  }
  // Render's Build Filters gate a repository push: every repository-backed
  // type has them, an image or a Build Source has no push to filter.
  if (shape.isGitSource) {
    errors.buildFilter = [
      ...form.buildFilterPaths,
      ...form.buildFilterIgnored,
    ].some((pattern) => buildFilterPatternError(pattern) !== null);
  }
  form.envVars.forEach((row, index) => {
    if (row.key === "") return; // the blank "add" placeholder never ships
    const rule = envValueRuleError(row.key, row.value);
    if (rule) errors.envValues[index] = rule;
  });
  return errors;
}

/** Whether the selected tab names a source the service can be built or run
 *  from: a picked repository, a valid repository URL, a valid image, or a
 *  selected Build Source. The create page reveals its settings once it does,
 *  as Render's does. */
export function sourceChosen(form: NewServiceForm): boolean {
  const shape = buildShape(form);
  return (
    (form.tab === "github" && form.selectedRepo != null) ||
    (form.tab === "git" &&
      isValidGitUrl(form.gitUrl) &&
      !parseGitHubPage(form.gitUrl)?.file) ||
    (shape.isImageSource && isValidDockerImageRef(form.image)) ||
    (shape.isBuildSource && form.selectedBuildSource != null)
  );
}

/** A native build (git or native Build Source) is missing a required command. */
export function nativeCommandsMissing(
  form: NewServiceForm,
  shape: BuildShape = buildShape(form),
): boolean {
  if (shape.isNativeBuild) {
    return (
      form.buildCommand.trim() === "" ||
      (shape.isCronType
        ? form.command.trim() === ""
        : form.startCommand.trim() === "")
    );
  }
  if (shape.isNativeBuildSource) {
    return shape.isCronType
      ? form.command.trim() === ""
      : (form.buildSourceStartCommand ?? "").trim() === "";
  }
  return false;
}

/** A secret file the server would refuse: a bad name or a repeated one. */
export function secretFilesInvalid(form: NewServiceForm): boolean {
  return (
    !submittableSecretFiles(form.secretFiles).every((file) =>
      isValidSecretFileName(file.name),
    ) || duplicateNames(form.secretFiles.map((file) => file.name)).size > 0
  );
}

/** True when the form is complete enough to submit, per the same build shapes. */
export function isSubmittable(form: NewServiceForm): boolean {
  const fieldErrors = createFormErrors(form);
  const shape = buildShape(form);
  return (
    sourceChosen(form) &&
    !nativeCommandsMissing(form, shape) &&
    // A value the server would refuse (a bad PORT, an entry over the process
    // limit, a NUL) never leaves the browser either (w21/050).
    Object.keys(fieldErrors.envValues).length === 0 &&
    !fieldErrors.rootDir &&
    !fieldErrors.dockerfilePath &&
    !fieldErrors.dockerContext &&
    !fieldErrors.healthCheckPath &&
    !fieldErrors.diskMountPath &&
    !fieldErrors.diskSize &&
    !fieldErrors.publishPath &&
    !fieldErrors.buildFilter &&
    !fieldErrors.secretFilesLimit &&
    submittableEnvVars(form.envVars).every((row) =>
      VALID_ENV_KEY.test(row.key),
    ) &&
    duplicateNames(form.envVars.map((row) => row.key)).size === 0 &&
    !secretFilesInvalid(form) &&
    // A port the backend would refuse never leaves the browser: the form blocks
    // Submit (w4/m121/t003). Types that bind no port never read the field at
    // all.
    (!shape.showPort || portIsValid(form, shape)) &&
    (shape.isStaticType || form.plan !== "") &&
    // bex-api refuses a static site without a publish directory, and the
    // builder trims a blank one away, so blank never reaches Submit (w4/332).
    // The placeholder is an example, not a default.
    (!shape.isStaticType || form.publishPath.trim() !== "") &&
    (!shape.isCronType ||
      (form.schedule.trim() !== "" && isValidCron(form.schedule)))
  );
}

/**
 * The one place the build shapes decide which fields ship. Every field the
 * current shape does not own is omitted rather than sent empty.
 */
export function buildCreateServiceInput(
  form: NewServiceForm,
): CreateServiceInput {
  const shape = buildShape(form);
  const paths = form.buildFilterPaths.map((p) => p.trim()).filter(Boolean);
  const ignoredPaths = form.buildFilterIgnored
    .map((p) => p.trim())
    .filter(Boolean);

  const source =
    shape.isBuildSource && form.selectedBuildSource
      ? {
          buildSourceId: form.selectedBuildSource.id,
          region:
            form.selectedBuildSource.kind === "git"
              ? form.selectedBuildSource.region || undefined
              : undefined,
        }
      : shape.isImageSource
        ? { image: form.image.trim() }
        : form.tab === "github" && form.selectedRepo
          ? {
              repo: form.selectedRepo.cloneUrl,
              branch:
                form.branch || form.selectedRepo.defaultBranch || undefined,
            }
          : { repo: form.gitUrl.trim(), branch: form.branch || undefined };

  return {
    ...source,
    // A free-form label (w5/m153); bex-api derives the resource name from it.
    name: form.name.trim(),
    type: form.serviceType,
    environmentId: form.environmentId || undefined,
    registryCredentialId: shape.usesRegistryCredential
      ? form.registryCredentialId
      : undefined,
    // The Git draft's root survives a tab switch so returning restores it, but
    // an image or Build Source has no repository root to scope (w4/226).
    rootDir: shape.isGitSource ? form.rootDir || undefined : undefined,
    runtime: shape.isImageSource
      ? "image"
      : shape.isBuildableGit
        ? form.runtime
        : undefined,
    buildCommand: shape.isStaticBuild
      ? optionalCommand(form.staticBuildCommand)
      : shape.isNativeBuild
        ? commandText(form.buildCommand)
        : undefined,
    startCommand: shape.isBuildableGit
      ? shape.isCronType
        ? commandText(form.command)
        : optionalCommand(form.startCommand)
      : shape.showImageCommand
        ? optionalCommand(form.imageCommand)
        : shape.isNativeBuildSource && !shape.isCronType
          ? commandText(form.buildSourceStartCommand ?? "")
          : undefined,
    dockerCommand: shape.showBuildSourceDockerCommand
      ? optionalCommand(form.buildSourceDockerCommand ?? "")
      : shape.isContainerBuildSource && shape.isCronType
        ? optionalCommand(form.command)
        : undefined,
    dockerfilePath: shape.isDockerBuild ? set(form.dockerfilePath) : undefined,
    dockerContext: shape.isDockerBuild ? set(form.dockerContext) : undefined,
    buildFilter:
      shape.isGitSource && (paths.length || ignoredPaths.length)
        ? { paths, ignoredPaths }
        : undefined,
    plan: shape.showPlan ? form.plan || undefined : undefined,
    autoDeploy:
      shape.isGitSource || shape.isBuildSource ? form.autoDeploy : undefined,
    schedule: shape.isCronType ? set(form.schedule) : undefined,
    // A Docker/image Build Source cron sends its optional command override as
    // dockerCommand (Render's command rule refuses startCommand/command on
    // container Build Sources).
    command:
      shape.isCronType && !shape.isContainerBuildSource
        ? optionalCommand(form.command)
        : undefined,
    publishPath: shape.isStaticType ? set(form.publishPath) : undefined,
    // The wizard used to send no port at all, so an image binding anything but
    // 3000 (nginx:alpine binds :80) came up unroutable and answered 503
    // (.pm/w4/m121 Evidence 1). Only the port-binding types send one.
    port: shape.showPort
      ? (parsePort(portDraft(form, shape)) ?? undefined)
      : undefined,
    healthCheckPath: shape.showHealthCheckPath
      ? set(form.healthCheckPath)
      : undefined,
    preDeployCommand:
      shape.showPreDeployCommand && !shape.preDeployPaidOnly
        ? optionalCommand(form.preDeployCommand)
        : undefined,
    disk:
      shape.showDisk && form.disk
        ? {
            name: DASHBOARD_DISK_NAME,
            mountPath: form.disk.mountPath.trim(),
            sizeGB: form.disk.sizeGB,
          }
        : undefined,
    envVars: submittableEnvVars(form.envVars),
    secretFiles: submittableSecretFiles(form.secretFiles),
  };
}
