import type {
  BuildSourceBuildFieldsFragment,
  BuildSourceFieldsFragment,
} from "@/graphql/definitions";
import { nonNull } from "@/common/lib/non-null";

export type BuildSourceKind = "git" | "image";

export interface BuildSourceFilterView {
  paths: string[];
  ignoredPaths: string[];
}

export interface BuildSourceGitView {
  repoUrl: string;
  branch: string;
  runtime: string | null;
  region: string | null;
  buildCommand: string | null;
  rootDir: string | null;
  dockerfilePath: string | null;
  dockerContext: string | null;
  registryCredentialId: string | null;
  buildFilter: BuildSourceFilterView | null;
}

export interface BuildSourceImageView {
  ownerId: string | null;
  imageUrl: string;
  registryCredentialId: string | null;
}

export interface BuildSourceServiceLinkView {
  id: string;
  name: string;
}

/** Normalized workspace Build Source (ADR097, Render BuildSource object). */
export interface BuildSourceView {
  id: string;
  name: string;
  ownerId: string;
  projectId: string | null;
  kind: BuildSourceKind;
  git: BuildSourceGitView | null;
  image: BuildSourceImageView | null;
  serviceLinks: BuildSourceServiceLinkView[];
  createdAt: string | null;
  updatedAt: string | null;
  cursor: string;
}

export interface BuildDeployRefView {
  serviceId: string;
  deployId: string;
}

export interface SkippedBuildServiceView {
  serviceId: string;
  reason: string;
}

/** One build of a Build Source (Render Build object). */
export interface BuildSourceBuildView {
  id: string;
  buildSourceId: string;
  /** Render buildRun.status: created | in_progress | succeeded | failed | canceled. */
  status: string;
  /** True when this build produced a deployable artifact (selectable in Manual Deploy). */
  succeeded: boolean;
  commitId: string | null;
  commitUrl: string | null;
  runtime: string | null;
  buildStartedAt: string | null;
  buildFinishedAt: string | null;
  imageRef: string | null;
  imageSha: string | null;
  registryCredentialId: string | null;
  deploys: BuildDeployRefView[];
  skippedServices: SkippedBuildServiceView[];
  createdAt: string | null;
  updatedAt: string | null;
  cursor: string;
}

/**
 * Structured payload from `409 BUILD_SOURCE_PARTIALLY_LINKED` when creating a
 * Build Source with multiple `serviceIds` linked at least one service before a
 * subsequent link failed (`apps/build_sources.go:306-308`).
 */
export interface BuildSourcePartiallyLinked {
  buildSourceId: string;
  linkedServiceIds: string[];
  failedServiceId: string;
  message: string;
}

export function mapBuildSource(
  raw: (BuildSourceFieldsFragment & { cursor?: string | null }) | null | undefined,
): BuildSourceView | null {
  if (!raw?.id) return null;
  const git = raw.git
    ? {
        repoUrl: raw.git.repoUrl ?? "",
        branch: raw.git.branch ?? "",
        runtime: raw.git.runtime ?? null,
        region: raw.git.region ?? null,
        buildCommand: raw.git.buildCommand ?? null,
        rootDir: raw.git.rootDir ?? null,
        dockerfilePath: raw.git.dockerfilePath ?? null,
        dockerContext: raw.git.dockerContext ?? null,
        registryCredentialId: raw.git.registryCredentialId || null,
        buildFilter: raw.git.buildFilter
          ? {
              paths: (raw.git.buildFilter.paths ?? []).filter(nonNull),
              ignoredPaths: (raw.git.buildFilter.ignoredPaths ?? []).filter(
                nonNull,
              ),
            }
          : null,
      }
    : null;
  const image = raw.image
    ? {
        ownerId: raw.image.ownerId || null,
        imageUrl: raw.image.imageUrl ?? "",
        registryCredentialId: raw.image.registryCredentialId || null,
      }
    : null;
  const serviceLinks = (raw.serviceLinks ?? []).flatMap((link) =>
    link?.id ? [{ id: link.id, name: link.name ?? link.id }] : [],
  );

  return {
    id: raw.id,
    name: raw.name ?? raw.id,
    ownerId: raw.ownerId ?? "",
    projectId: raw.projectId || null,
    kind: git ? "git" : "image",
    git,
    image,
    serviceLinks,
    createdAt: raw.createdAt ?? null,
    updatedAt: raw.updatedAt ?? null,
    cursor: raw.cursor ?? raw.id,
  };
}

export function mapBuildSourceBuild(
  raw: BuildSourceBuildFieldsFragment | null | undefined,
  fallbackBuildSourceId = "",
): BuildSourceBuildView | null {
  if (!raw?.id) return null;
  const status = raw.buildRun?.status ?? (raw.image ? "succeeded" : "");
  return {
    id: raw.id,
    buildSourceId: raw.buildSourceId ?? fallbackBuildSourceId,
    status,
    succeeded: status === "succeeded",
    commitId: raw.buildRun?.commitId ?? null,
    commitUrl: raw.buildRun?.commitUrl ?? null,
    runtime: raw.buildRun?.runtime ?? null,
    buildStartedAt: raw.buildRun?.buildStartedAt ?? null,
    buildFinishedAt: raw.buildRun?.buildFinishedAt ?? null,
    imageRef: raw.image?.imageUrl ?? raw.image?.ref ?? null,
    imageSha: raw.image?.sha ?? null,
    registryCredentialId: raw.image?.registryCredentialId || null,
    deploys: (raw.deploys ?? []).flatMap((d) =>
      d?.serviceId && d?.deployId
        ? [{ serviceId: d.serviceId, deployId: d.deployId }]
        : [],
    ),
    skippedServices: (raw.skippedServices ?? []).flatMap((s) =>
      s?.serviceId && s?.reason
        ? [{ serviceId: s.serviceId, reason: s.reason }]
        : [],
    ),
    createdAt: raw.createdAt ?? null,
    updatedAt: raw.updatedAt ?? null,
    cursor: raw.cursor ?? raw.id,
  };
}
