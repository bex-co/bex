import { isValidGitUrl } from "@/common/lib/utils/git-url";
import { isValidCron } from "@/features/services/lib/cron";
import {
  VALID_ENV_KEY,
  isValidSecretFileName,
} from "@/features/services/lib/environment-draft";
import type { ServiceType } from "@/features/services/lib/create-context";
import type { GitRuntime } from "@/features/services/lib/runtime";
import type { SourceTab } from "@/features/services/components/service-source-picker";
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
  registryCredentialId: string;
  branch: string;
  rootDir: string;
  runtime: GitRuntime;
  buildCommand: string;
  startCommand: string;
  dockerfilePath: string;
  staticBuildCommand: string;
  publishPath: string;
  buildFilterPaths: string[];
  buildFilterIgnored: string[];
  plan: string;
  autoDeploy: boolean;
  schedule: string;
  command: string;
  envVars: EnvVarEntry[];
  secretFiles: SecretFileEntry[];
  projectId: string | null;
  environmentId: string | null;
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
  isGitSource: boolean;
  isBuildableGit: boolean;
  isDockerBuild: boolean;
  isNativeBuild: boolean;
  isStaticBuild: boolean;
  usesRegistryCredential: boolean;
  showPlan: boolean;
  showNoUrlNote: boolean;
}

export function buildShape(form: NewServiceForm): BuildShape {
  const isCronType = form.serviceType === "cron_job";
  const isStaticType = form.serviceType === "static_site";
  const isImageSource = form.tab === "image";
  const isGitSource = form.tab === "github" || form.tab === "git";
  const isBuildableGit = isGitSource && !isStaticType;
  const isDockerBuild = isBuildableGit && form.runtime === "docker";
  return {
    isCronType,
    isStaticType,
    isImageSource,
    isGitSource,
    isBuildableGit,
    isDockerBuild,
    isNativeBuild: isBuildableGit && form.runtime !== "docker",
    isStaticBuild: isGitSource && isStaticType,
    usesRegistryCredential: isImageSource || isDockerBuild,
    showPlan: !isStaticType,
    showNoUrlNote:
      form.serviceType === "private_service" ||
      form.serviceType === "background_worker",
  };
}

/** Blank means "leave it to the platform default", never an empty string. */
function set(value: string): string | undefined {
  return value.trim() || undefined;
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

/** True when the form is complete enough to submit, per the same build shapes. */
export function isSubmittable(form: NewServiceForm): boolean {
  const shape = buildShape(form);
  const sourceValid =
    (form.tab === "github" && form.selectedRepo != null) ||
    (form.tab === "git" && isValidGitUrl(form.gitUrl)) ||
    (shape.isImageSource && form.image.trim().length > 0);
  const nativeCommandsValid =
    !shape.isNativeBuild ||
    (form.buildCommand.trim() !== "" &&
      (shape.isCronType
        ? form.command.trim() !== ""
        : form.startCommand.trim() !== ""));
  return (
    sourceValid &&
    nativeCommandsValid &&
    submittableEnvVars(form.envVars).every((row) =>
      VALID_ENV_KEY.test(row.key),
    ) &&
    submittableSecretFiles(form.secretFiles).every((file) =>
      isValidSecretFileName(file.name),
    ) &&
    (shape.isStaticType || form.plan !== "") &&
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
    form.tab === "github" && form.selectedRepo
      ? {
          repo: form.selectedRepo.cloneUrl,
          branch: form.branch || form.selectedRepo.defaultBranch || undefined,
        }
      : form.tab === "git"
        ? { repo: form.gitUrl.trim(), branch: form.branch || undefined }
        : { image: form.image.trim() };

  return {
    ...source,
    name: form.name,
    type: form.serviceType,
    environmentId: form.environmentId || undefined,
    registryCredentialId: shape.usesRegistryCredential
      ? form.registryCredentialId
      : undefined,
    rootDir: form.rootDir || undefined,
    runtime: shape.isImageSource
      ? "image"
      : shape.isBuildableGit
        ? form.runtime
        : undefined,
    buildCommand: shape.isStaticBuild
      ? set(form.staticBuildCommand)
      : shape.isNativeBuild
        ? form.buildCommand.trim()
        : undefined,
    startCommand: shape.isBuildableGit
      ? shape.isCronType
        ? form.command.trim()
        : set(form.startCommand)
      : undefined,
    dockerfilePath: shape.isDockerBuild ? set(form.dockerfilePath) : undefined,
    buildFilter:
      shape.isStaticBuild && (paths.length || ignoredPaths.length)
        ? { paths, ignoredPaths }
        : undefined,
    plan: shape.showPlan ? form.plan || undefined : undefined,
    autoDeploy: shape.isGitSource ? form.autoDeploy : undefined,
    schedule: shape.isCronType ? set(form.schedule) : undefined,
    command: shape.isCronType ? set(form.command) : undefined,
    publishPath: shape.isStaticType ? set(form.publishPath) : undefined,
    envVars: submittableEnvVars(form.envVars),
    secretFiles: submittableSecretFiles(form.secretFiles),
  };
}
