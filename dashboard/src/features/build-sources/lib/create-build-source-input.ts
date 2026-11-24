import { graphQLErrorExtensions, refusalReason } from "@/common/lib/graphql-error";
import { isValidDockerImageRef } from "@/common/lib/utils/docker-image";
import { isValidGitUrl, parseGitHubPage } from "@/common/lib/utils/git-url";
import { isValidLabel } from "@/common/lib/utils/resource-label";
import {
  buildFilterPatternError,
  relativePathError,
  rootDirError,
} from "@/features/services/lib/build-settings-validation";
import {
  submittableEnvVars,
  submittableSecretFiles,
} from "@/features/services/lib/create-service-input";
import {
  VALID_ENV_KEY,
  duplicateNames,
  envValueRuleError,
  isValidSecretFileName,
  secretFilesOverLimit,
} from "@/features/services/lib/environment-draft";
import type {
  EnvVarEntry,
  SecretFileEntry,
} from "@/features/services/hooks/use-create-service";
import type { RepoView } from "@/features/services/hooks/use-repos";
import type { GitRuntime } from "@/features/services/lib/runtime";
import type { SourceTab } from "@/features/services/components/service-source-picker";
import type { BuildSourcePartiallyLinked } from "@/features/build-sources/types";
import type { CreateBuildSourceMutationVariables } from "@/graphql/definitions";

export const BUILD_SOURCE_NAME_TAKEN_CODE = "BUILD_SOURCE_NAME_TAKEN";
export const BUILD_SOURCE_PARTIALLY_LINKED_CODE =
  "BUILD_SOURCE_PARTIALLY_LINKED";

/** Render's five Build Source / Service regions (resourcemeta.RenderRegions). */
export const BUILD_SOURCE_REGIONS = [
  "oregon",
  "ohio",
  "virginia",
  "frankfurt",
  "singapore",
] as const;

export type BuildSourceRegion = (typeof BUILD_SOURCE_REGIONS)[number];

/**
 * Service types Render allows linking to a Build Source (`buildSourceLinkTypes`
 * in `lego/backend/internal/apps/build_sources.go`). `static_site` is excluded.
 */
const LINKABLE_SERVICE_TYPES = new Set([
  "web_service",
  "private_service",
  "background_worker",
  "cron_job",
]);

export function isLinkableServiceType(type: string | null | undefined): boolean {
  return type != null && LINKABLE_SERVICE_TYPES.has(type);
}

/** Source tab for `/build-sources/new` (GitHub repo, Public Git URL, or Docker image). */
export type BuildSourceCreateTab = Exclude<SourceTab, "build-source">;

export interface CreateBuildSourceFormState {
  tab: BuildSourceCreateTab;
  selectedRepo: RepoView | null;
  gitUrl: string;
  imageUrl: string;
  imageRegistryCredentialId: string;
  name: string;
  projectId: string | null;
  runtime: GitRuntime;
  branch: string;
  region: string;
  buildCommand: string;
  rootDir: string;
  dockerfilePath: string;
  dockerContext: string;
  gitRegistryCredentialId: string;
  buildFilterPaths: string[];
  buildFilterIgnored: string[];
  envVars: EnvVarEntry[];
  secretFiles: SecretFileEntry[];
  envGroupIds: string[];
  serviceIds: string[];
}

export function isGitCreateTab(tab: BuildSourceCreateTab): boolean {
  return tab === "github" || tab === "git";
}

export function isBuildSourceSourceChosen(
  form: CreateBuildSourceFormState,
): boolean {
  if (form.tab === "github") return form.selectedRepo !== null;
  if (form.tab === "git") {
    const trimmed = form.gitUrl.trim();
    if (!trimmed) return false;
    const page = parseGitHubPage(trimmed);
    return isValidGitUrl(trimmed) && page?.file !== true;
  }
  return (
    form.imageUrl.trim() !== "" && isValidDockerImageRef(form.imageUrl.trim())
  );
}

export interface CreateBuildSourceFieldErrors {
  rootDir: string | null;
  dockerfilePath: string | null;
  dockerContext: string | null;
  buildFilter: boolean;
  envVarsInvalid: boolean;
  secretFilesInvalid: boolean;
  secretFilesLimit: boolean;
}

export function getCreateBuildSourceFieldErrors(
  form: CreateBuildSourceFormState,
): CreateBuildSourceFieldErrors {
  if (!isGitCreateTab(form.tab)) {
    return {
      rootDir: null,
      dockerfilePath: null,
      dockerContext: null,
      buildFilter: false,
      envVarsInvalid: false,
      secretFilesInvalid: false,
      secretFilesLimit: false,
    };
  }
  const isDocker = form.runtime === "docker";
  const rootDir = form.rootDir.trim() ? rootDirError(form.rootDir) : null;
  const dockerfilePath =
    isDocker && form.dockerfilePath.trim()
      ? relativePathError(form.dockerfilePath)
      : null;
  const dockerContext =
    isDocker && form.dockerContext.trim()
      ? relativePathError(form.dockerContext)
      : null;
  const buildFilter = [...form.buildFilterPaths, ...form.buildFilterIgnored].some(
    (p) => p.trim() !== "" && buildFilterPatternError(p) !== null,
  );
  const envDupes = duplicateNames(form.envVars.map((r) => r.key));
  const envVarsInvalid = form.envVars.some((r) => {
    if (r.key === "" && r.value === "") return false;
    if (r.key === "" || !VALID_ENV_KEY.test(r.key)) return true;
    if (envDupes.has(r.key.trim())) return true;
    return envValueRuleError(r.key, r.value) !== null;
  });
  const fileDupes = duplicateNames(form.secretFiles.map((r) => r.name));
  const secretFilesInvalid = form.secretFiles.some((r) => {
    if (r.name === "" && r.content === "") return false;
    if (r.name === "" || !isValidSecretFileName(r.name)) return true;
    return fileDupes.has(r.name.trim());
  });
  const secretFilesLimit = secretFilesOverLimit(
    submittableSecretFiles(form.secretFiles),
  );

  return {
    rootDir,
    dockerfilePath,
    dockerContext,
    buildFilter,
    envVarsInvalid,
    secretFilesInvalid,
    secretFilesLimit,
  };
}

export function isCreateBuildSourceSubmittable(
  form: CreateBuildSourceFormState,
): boolean {
  if (!isBuildSourceSourceChosen(form)) return false;
  if (!isValidLabel(form.name)) return false;
  const errs = getCreateBuildSourceFieldErrors(form);
  return (
    !errs.rootDir &&
    !errs.dockerfilePath &&
    !errs.dockerContext &&
    !errs.buildFilter &&
    !errs.envVarsInvalid &&
    !errs.secretFilesInvalid &&
    !errs.secretFilesLimit
  );
}

function nonEmpty(value: string): string | undefined {
  const trimmed = value.trim();
  return trimmed || undefined;
}

/**
 * Builds the exact GraphQL `CreateBuildSource` mutation variables matching
 * Render's `POST /v1/build-sources` contract:
 * - Git sources omit `image`; Docker-only fields (`dockerfilePath`,
 *   `dockerContext`, `registryCredentialId`) are omitted for native runtimes,
 *   and `buildCommand` is omitted for `docker`.
 * - Image sources omit `git`, `envVars`, `secretFiles`, and `envGroupIds`.
 */
export function buildCreateBuildSourceParams(
  form: CreateBuildSourceFormState,
): Omit<CreateBuildSourceMutationVariables, "ownerId"> {
  const base: Omit<CreateBuildSourceMutationVariables, "ownerId"> = {
    name: form.name.trim(),
    ...(form.projectId ? { projectId: form.projectId } : {}),
    ...(form.serviceIds.length > 0 ? { serviceIds: form.serviceIds } : {}),
  };

  if (!isGitCreateTab(form.tab)) {
    return {
      ...base,
      image: {
        imageUrl: form.imageUrl.trim(),
        ...(nonEmpty(form.imageRegistryCredentialId)
          ? { registryCredentialId: form.imageRegistryCredentialId.trim() }
          : {}),
      },
    };
  }

  const repoUrl =
    form.tab === "github"
      ? (form.selectedRepo?.htmlUrl ?? "")
      : form.gitUrl.trim();
  const isDocker = form.runtime === "docker";
  const paths = form.buildFilterPaths.map((p) => p.trim()).filter(Boolean);
  const ignoredPaths = form.buildFilterIgnored
    .map((p) => p.trim())
    .filter(Boolean);
  const envVars = submittableEnvVars(form.envVars);
  const secretFiles = submittableSecretFiles(form.secretFiles);

  return {
    ...base,
    git: {
      repoUrl,
      runtime: form.runtime,
      ...(nonEmpty(form.branch) ? { branch: form.branch.trim() } : {}),
      ...(nonEmpty(form.region) ? { region: form.region.trim() } : {}),
      ...(nonEmpty(form.rootDir) ? { rootDir: form.rootDir.trim() } : {}),
      ...(!isDocker && nonEmpty(form.buildCommand)
        ? { buildCommand: form.buildCommand.trim() }
        : {}),
      ...(isDocker && nonEmpty(form.dockerfilePath)
        ? { dockerfilePath: form.dockerfilePath.trim() }
        : {}),
      ...(isDocker && nonEmpty(form.dockerContext)
        ? { dockerContext: form.dockerContext.trim() }
        : {}),
      ...(isDocker && nonEmpty(form.gitRegistryCredentialId)
        ? { registryCredentialId: form.gitRegistryCredentialId.trim() }
        : {}),
      ...(paths.length > 0 || ignoredPaths.length > 0
        ? { buildFilter: { paths, ignoredPaths } }
        : {}),
    },
    ...(envVars.length > 0 ? { envVars } : {}),
    ...(secretFiles.length > 0 ? { secretFiles } : {}),
    ...(form.envGroupIds.length > 0 ? { envGroupIds: form.envGroupIds } : {}),
  };
}

/**
 * Extracts structured `BUILD_SOURCE_PARTIALLY_LINKED` details from a GraphQL
 * error when creating a Build Source succeeded and linked some services before
 * a subsequent service link failed.
 */
export function extractPartiallyLinkedError(
  err: unknown,
): BuildSourcePartiallyLinked | null {
  const ext = graphQLErrorExtensions(err, BUILD_SOURCE_PARTIALLY_LINKED_CODE);
  if (!ext) return null;
  const buildSourceId =
    typeof ext["buildSourceId"] === "string" ? ext["buildSourceId"] : "";
  const linkedServiceIds = Array.isArray(ext["linkedServiceIds"])
    ? ext["linkedServiceIds"].filter((v): v is string => typeof v === "string")
    : [];
  const failedServiceId =
    typeof ext["failedServiceId"] === "string" ? ext["failedServiceId"] : "";
  return {
    buildSourceId,
    linkedServiceIds,
    failedServiceId,
    message: refusalReason(err),
  };
}

/** Suggests a clean initial Build Source name from a repo name or image reference. */
export function suggestBuildSourceName(raw: string): string {
  const trimmed = raw.trim().replace(/\/+$/, "").replace(/\.git$/i, "");
  if (!trimmed) return "";
  // Strip digest/tag from image references before taking the final path segment.
  const withoutDigest = trimmed.split("@")[0] ?? trimmed;
  const lastSlash = withoutDigest.lastIndexOf("/");
  const segment = lastSlash >= 0 ? withoutDigest.slice(lastSlash + 1) : withoutDigest;
  const withoutTag = segment.split(":")[0] ?? segment;
  return withoutTag.slice(0, 100);
}
