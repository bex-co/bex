import { Link } from "@tanstack/react-router";
import {
  AlertTriangle,
  CheckCircle2,
  Clock,
  GitCommitHorizontal,
  Layers,
  Loader2,
  Package,
  Play,
  XCircle,
} from "lucide-react";
import { LocalDateTime } from "@/common/components/local-time";
import { ResourceLoadError } from "@/common/components/resource-load-error";
import { Alert, AlertDescription } from "@/common/components/ui/alert";
import { Badge } from "@/common/components/ui/badge";
import { Button } from "@/common/components/ui/button";
import { Card, CardContent } from "@/common/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/common/components/ui/table";
import { isUnauthenticatedError } from "@/common/apollo/auth-error-link";
import { useTranslations } from "@/common/hooks/use-translations";
import { PermissionTooltip } from "@/features/capabilities/components/permission-tooltip";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { CommitSha } from "@/features/deploys/components/commit-sha";
import { repoCommitUrl } from "@/features/services/lib/repo";
import { useBuildSourceDetailContext } from "@/features/build-sources/components/build-source-detail-context";
import { BuildSourceBuildsTabSkeleton } from "@/features/build-sources/components/build-source-skeletons";
import { useBuildSourceBuilds } from "@/features/build-sources/hooks/use-build-source-builds";
import {
  buildDurationValue,
  buildStatusLabelKey,
  buildStatusVariant,
  isActiveBuildStatus,
  shortBuildDigest,
} from "@/features/build-sources/lib/build-status";
import type { BuildSourceBuildView } from "@/features/build-sources/types";

export function BuildStatusBadge({
  status,
  iconClassName = "size-3",
}: {
  status: string;
  iconClassName?: string;
}) {
  const { t } = useTranslations();
  const variant = buildStatusVariant(status);
  const label = t(buildStatusLabelKey(status) as Parameters<typeof t>[0]);
  const active = isActiveBuildStatus(status);

  return (
    <Badge variant={variant} className="gap-1">
      {active ? (
        <Loader2
          className={`${iconClassName} animate-spin`}
          aria-hidden="true"
        />
      ) : variant === "success" ? (
        <CheckCircle2 className={iconClassName} aria-hidden="true" />
      ) : variant === "destructive" ? (
        <XCircle className={iconClassName} aria-hidden="true" />
      ) : (
        <Clock className={iconClassName} aria-hidden="true" />
      )}
      {label}
    </Badge>
  );
}

/**
 * Builds tab (`/build-sources/$buildSourceId` index, `w2/m614/t003`):
 * lists builds (`bld-…`) with status badges, commit/image digest, duration,
 * fan-out deploy counts, cursor pagination, and bounded polling while active.
 */
export function BuildSourceBuildsTab() {
  const { t } = useTranslations();
  const { buildSource, openTriggerBuild } = useBuildSourceDetailContext();
  const capabilities = useCapabilities();
  const { canCreate } = capabilities;
  const createReasonKey = capabilities.reasonKey("can_create");
  const createReason = createReasonKey ? t(createReasonKey) : undefined;

  const { builds, loading, loadingMore, hasMore, error, loadMore, refetch } =
    useBuildSourceBuilds(buildSource.id, true, 20, { poll: true });

  const isGit = buildSource.kind === "git";
  const repoUrl = buildSource.git?.repoUrl ?? null;

  if (loading) {
    return <BuildSourceBuildsTabSkeleton />;
  }

  if (error && builds.length === 0) {
    return (
      <ResourceLoadError
        variant={isUnauthenticatedError(error) ? "unauthenticated" : "error"}
        onRetry={() => void refetch()}
      />
    );
  }

  if (builds.length === 0) {
    return (
      <div
        data-region="builds-list"
        className="flex flex-col items-center gap-2 rounded-lg border border-dashed px-4 py-12 text-center"
      >
        <Layers className="text-muted-foreground size-8" aria-hidden="true" />
        <p className="text-sm font-medium">
          {t("buildSources.buildsEmptyTitle")}
        </p>
        <p className="text-muted-foreground max-w-md text-xs">
          {t(
            isGit
              ? "buildSources.buildsEmptyGitDescription"
              : "buildSources.buildsEmptyImageDescription",
          )}
        </p>
        {isGit ? (
          <PermissionTooltip reason={createReason}>
            <Button
              size="sm"
              className="mt-2"
              disabled={!canCreate}
              onClick={openTriggerBuild}
            >
              <Play className="size-4" aria-hidden="true" />
              {t("buildSources.triggerBuildButton")}
            </Button>
          </PermissionTooltip>
        ) : null}
      </div>
    );
  }

  return (
    <div data-region="builds-list" className="space-y-4">
      {error ? (
        <Alert variant="destructive">
          <AlertTriangle className="size-4" aria-hidden="true" />
          <AlertDescription className="flex flex-wrap items-center justify-between gap-2">
            <span>{t("buildSources.buildsRefreshError")}</span>
            <Button size="sm" variant="outline" onClick={() => void refetch()}>
              {t("common.tryAgain")}
            </Button>
          </AlertDescription>
        </Alert>
      ) : null}

      {/* Desktop table (md+) */}
      <div className="hidden overflow-hidden rounded-lg border md:block">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="w-[24%]">
                {t("buildSources.colBuildId")}
              </TableHead>
              <TableHead className="w-[16%]">
                {t("buildSources.colStatus")}
              </TableHead>
              <TableHead className="w-[24%]">
                {t("buildSources.colArtifact")}
              </TableHead>
              <TableHead className="w-[12%]">
                {t("buildSources.colDuration")}
              </TableHead>
              <TableHead className="w-[12%]">
                {t("buildSources.colDeploys")}
              </TableHead>
              <TableHead className="w-[12%] text-right">
                {t("buildSources.colStarted")}
              </TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {builds.map((build) => (
              <BuildDesktopRow
                key={build.id}
                buildSourceId={buildSource.id}
                repoUrl={repoUrl}
                build={build}
              />
            ))}
          </TableBody>
        </Table>
      </div>

      {/* Narrow-mobile stacked cards (< md, 390px) */}
      <div className="space-y-3 md:hidden">
        {builds.map((build) => (
          <BuildMobileCard
            key={build.id}
            buildSourceId={buildSource.id}
            repoUrl={repoUrl}
            build={build}
          />
        ))}
      </div>

      {hasMore ? (
        <div className="flex justify-center pt-1">
          <Button
            type="button"
            variant="outline"
            size="sm"
            disabled={loadingMore}
            onClick={() => void loadMore()}
          >
            {loadingMore ? (
              <Loader2 className="size-4 animate-spin" aria-hidden="true" />
            ) : null}
            {t("buildSources.buildsLoadMore")}
          </Button>
        </div>
      ) : null}
    </div>
  );
}

function BuildArtifactCell({
  build,
  repoUrl,
}: {
  build: BuildSourceBuildView;
  repoUrl: string | null;
}) {
  const commitUrl =
    build.commitUrl ??
    (repoUrl && build.commitId ? repoCommitUrl(repoUrl, build.commitId) : null);

  if (build.commitId) {
    return (
      <span className="inline-flex items-center gap-1.5 text-xs">
        <GitCommitHorizontal
          className="text-muted-foreground size-3.5 shrink-0"
          aria-hidden="true"
        />
        <CommitSha commitId={build.commitId} url={commitUrl} />
      </span>
    );
  }

  if (build.imageSha || build.imageRef) {
    return (
      <span
        className="text-muted-foreground inline-flex max-w-full items-center gap-1.5 font-mono text-xs"
        title={build.imageSha ?? build.imageRef ?? undefined}
      >
        <Package className="size-3.5 shrink-0" aria-hidden="true" />
        <span className="truncate">
          {build.imageSha ? shortBuildDigest(build.imageSha) : build.imageRef}
        </span>
      </span>
    );
  }

  return <span className="text-muted-foreground text-xs">—</span>;
}

function BuildDesktopRow({
  buildSourceId,
  repoUrl,
  build,
}: {
  buildSourceId: string;
  repoUrl: string | null;
  build: BuildSourceBuildView;
}) {
  const { t } = useTranslations();
  const duration = buildDurationValue(build);
  const stamp = build.buildStartedAt ?? build.createdAt;

  return (
    <TableRow>
      <TableCell className="py-3.5 font-mono text-xs">
        <Link
          to="/build-sources/$buildSourceId/builds/$buildId"
          params={{ buildSourceId, buildId: build.id }}
          className="text-foreground font-medium hover:underline"
        >
          {build.id}
        </Link>
      </TableCell>
      <TableCell className="py-3.5">
        <BuildStatusBadge status={build.status} />
      </TableCell>
      <TableCell className="max-w-xs py-3.5">
        <BuildArtifactCell build={build} repoUrl={repoUrl} />
      </TableCell>
      <TableCell className="text-muted-foreground py-3.5 text-sm tabular-nums">
        {duration ?? "—"}
      </TableCell>
      <TableCell className="py-3.5">
        <div className="flex flex-wrap items-center gap-1">
          <Badge variant="outline" className="tabular-nums">
            {t("buildSources.buildDeploysCount", {
              count: build.deploys.length,
            })}
          </Badge>
          {build.skippedServices.length > 0 ? (
            <Badge variant="secondary" className="tabular-nums">
              {t("buildSources.buildSkippedCount", {
                count: build.skippedServices.length,
              })}
            </Badge>
          ) : null}
        </div>
      </TableCell>
      <TableCell className="text-muted-foreground py-3.5 text-right text-sm whitespace-nowrap">
        {stamp ? <LocalDateTime value={stamp} /> : "—"}
      </TableCell>
    </TableRow>
  );
}

function BuildMobileCard({
  buildSourceId,
  repoUrl,
  build,
}: {
  buildSourceId: string;
  repoUrl: string | null;
  build: BuildSourceBuildView;
}) {
  const { t } = useTranslations();
  const duration = buildDurationValue(build);
  const stamp = build.buildStartedAt ?? build.createdAt;

  return (
    <Card className="gap-2 py-4">
      <CardContent className="space-y-2.5 px-4">
        <div className="flex items-center justify-between gap-2">
          <Link
            to="/build-sources/$buildSourceId/builds/$buildId"
            params={{ buildSourceId, buildId: build.id }}
            className="truncate font-mono text-xs font-medium hover:underline"
          >
            {build.id}
          </Link>
          <BuildStatusBadge status={build.status} />
        </div>

        <BuildArtifactCell build={build} repoUrl={repoUrl} />

        <div className="text-muted-foreground flex flex-wrap items-center justify-between gap-2 border-t pt-2 text-xs">
          <span className="tabular-nums">
            {t("buildSources.buildDeploysCount", {
              count: build.deploys.length,
            })}
            {duration ? ` · ${duration}` : ""}
          </span>
          {stamp ? <LocalDateTime value={stamp} /> : "—"}
        </div>
      </CardContent>
    </Card>
  );
}
