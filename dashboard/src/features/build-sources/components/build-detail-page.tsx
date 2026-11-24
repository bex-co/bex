import { useMemo } from "react";
import { Link } from "@tanstack/react-router";
import {
  AlertTriangle,
  CalendarDays,
  ChevronLeft,
  Cpu,
  ExternalLink,
  GitCommitHorizontal,
  Package,
  Timer,
} from "lucide-react";
import { isUnauthenticatedError } from "@/common/apollo/auth-error-link";
import { CopyButton } from "@/common/components/copy-button";
import { LocalDateTime } from "@/common/components/local-time";
import { ResourceLoadError } from "@/common/components/resource-load-error";
import {
  Alert,
  AlertDescription,
  AlertTitle,
} from "@/common/components/ui/alert";
import { Badge } from "@/common/components/ui/badge";
import { Button } from "@/common/components/ui/button";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import {
  resourceNotFound,
  useNotFoundRedirect,
} from "@/common/hooks/use-not-found-redirect";
import { useTranslations } from "@/common/hooks/use-translations";
import { CommitSha } from "@/features/deploys/components/commit-sha";
import type { EventSourceFactory } from "@/features/logs/hooks/use-live-logs";
import { useServices } from "@/features/services/hooks/use-services";
import { repoCommitUrl } from "@/features/services/lib/repo";
import { BuildLogPanel } from "@/features/build-sources/components/build-log-panel";
import { BuildStatusBadge } from "@/features/build-sources/components/build-source-builds-tab";
import { useBuildSourceDetailContext } from "@/features/build-sources/components/build-source-detail-context";
import { BuildDetailRouteSkeleton } from "@/features/build-sources/components/build-source-skeletons";
import { useBuildSourceBuild } from "@/features/build-sources/hooks/use-build-source-builds";
import {
  buildDurationValue,
  isActiveBuildStatus,
  shortBuildDigest,
  skippedReasonLabel,
} from "@/features/build-sources/lib/build-status";

export interface BuildDetailPageProps {
  buildSourceId: string;
  buildId: string;
  createEventSource?: EventSourceFactory;
}

/**
 * Build Detail & Live Build Logs page (`/build-sources/$buildSourceId/builds/$buildId`,
 * `w2/m614/t004`): renders build metadata, fan-out deploys / skippedServices,
 * and historical + live-streaming SSE build logs (`resource=bds-…`, `build=[bld-…]`).
 */
export function BuildDetailPage({
  buildSourceId,
  buildId,
  createEventSource,
}: BuildDetailPageProps) {
  const { t } = useTranslations();
  const { buildSource } = useBuildSourceDetailContext();
  const { build, loading, error, notFound, refetch } = useBuildSourceBuild(
    buildSourceId,
    buildId,
  );
  const { services } = useServices({ poll: false });

  const serviceNameById = useMemo(() => {
    const map = new Map<string, string>();
    for (const s of services) map.set(s.id, s.name);
    for (const s of buildSource.serviceLinks) map.set(s.id, s.name);
    return map;
  }, [buildSource.serviceLinks, services]);

  const buildsTab = useMemo(
    () => ({
      to: "/build-sources/$buildSourceId" as const,
      params: { buildSourceId },
      replace: true,
    }),
    [buildSourceId],
  );
  useNotFoundRedirect(
    notFound || resourceNotFound(build, loading, error),
    buildsTab,
  );

  const backLink = (
    <Link
      to="/build-sources/$buildSourceId"
      params={{ buildSourceId }}
      className="text-muted-foreground hover:text-foreground inline-flex h-5 items-center gap-1 text-xs"
    >
      <ChevronLeft className="size-3.5" aria-hidden="true" />
      {t("buildSources.backToBuilds")}
    </Link>
  );

  if (loading) {
    return <BuildDetailRouteSkeleton />;
  }

  if (error && !build && !notFound) {
    return (
      <div className="space-y-4">
        {backLink}
        <ResourceLoadError
          variant={isUnauthenticatedError(error) ? "unauthenticated" : "error"}
          onRetry={() => void refetch()}
        />
      </div>
    );
  }

  if (!build) {
    return <BuildDetailRouteSkeleton />;
  }

  const isGit = buildSource.kind === "git";
  const active = isActiveBuildStatus(build.status);
  const duration = buildDurationValue(build);
  const startedAt = build.buildStartedAt ?? build.createdAt ?? undefined;
  const endTime = active ? undefined : (build.buildFinishedAt ?? undefined);
  const commitUrl =
    build.commitUrl ??
    (buildSource.git?.repoUrl && build.commitId
      ? repoCommitUrl(buildSource.git.repoUrl, build.commitId)
      : null);

  return (
    <div className="space-y-6">
      {error ? (
        <Alert variant="destructive">
          <AlertTriangle className="size-4" aria-hidden="true" />
          <AlertDescription className="flex flex-wrap items-center justify-between gap-2">
            <span>{t("buildSources.buildDetailRefreshError")}</span>
            <Button size="sm" variant="outline" onClick={() => void refetch()}>
              {t("common.tryAgain")}
            </Button>
          </AlertDescription>
        </Alert>
      ) : null}

      <div data-region="build-header" className="space-y-4">
        {backLink}

        <div className="flex flex-wrap items-start justify-between gap-3">
          <div className="min-w-0 space-y-1">
            <div className="flex flex-wrap items-center gap-1.5">
              <h2 className="font-mono text-lg font-semibold tracking-tight break-all">
                {build.id}
              </h2>
              <CopyButton
                value={build.id}
                label={t("buildSources.copyBuildId")}
                successText={t("buildSources.copied")}
                errorText={t("buildSources.copyError")}
              />
            </div>
            <p className="text-muted-foreground text-xs">
              {t("buildSources.buildDetailSubtitle", {
                name: buildSource.name,
              })}
            </p>
          </div>
          <BuildStatusBadge status={build.status} iconClassName="size-3.5" />
        </div>

        <dl className="grid grid-cols-1 gap-4 rounded-lg border p-4 text-sm sm:grid-cols-2 lg:grid-cols-4">
          <div className="space-y-1">
            <dt className="text-muted-foreground flex items-center gap-1.5 text-xs">
              <CalendarDays className="size-3.5" aria-hidden="true" />
              {t("buildSources.factStarted")}
            </dt>
            <dd className="font-medium">
              {startedAt ? <LocalDateTime value={startedAt} /> : "—"}
            </dd>
          </div>

          <div className="space-y-1">
            <dt className="text-muted-foreground flex items-center gap-1.5 text-xs">
              <Timer className="size-3.5" aria-hidden="true" />
              {t("buildSources.colDuration")}
            </dt>
            <dd className="font-medium tabular-nums">{duration ?? "—"}</dd>
          </div>

          {build.runtime ? (
            <div className="space-y-1">
              <dt className="text-muted-foreground flex items-center gap-1.5 text-xs">
                <Cpu className="size-3.5" aria-hidden="true" />
                {t("buildSources.fieldRuntime")}
              </dt>
              <dd className="font-mono font-medium">{build.runtime}</dd>
            </div>
          ) : null}

          {build.commitId ? (
            <div className="space-y-1">
              <dt className="text-muted-foreground flex items-center gap-1.5 text-xs">
                <GitCommitHorizontal className="size-3.5" aria-hidden="true" />
                {t("buildSources.factCommit")}
              </dt>
              <dd>
                <CommitSha commitId={build.commitId} url={commitUrl} />
              </dd>
            </div>
          ) : null}

          {build.imageRef || build.imageSha ? (
            <div className="space-y-1 sm:col-span-2 lg:col-span-4">
              <dt className="text-muted-foreground flex items-center gap-1.5 text-xs">
                <Package className="size-3.5" aria-hidden="true" />
                {t("buildSources.factArtifact")}
              </dt>
              <dd className="flex flex-wrap items-center gap-2 font-mono text-xs">
                {build.imageRef ? (
                  <span className="break-all">{build.imageRef}</span>
                ) : null}
                {build.imageSha ? (
                  <span className="text-muted-foreground inline-flex items-center gap-1">
                    <span title={build.imageSha}>
                      {shortBuildDigest(build.imageSha)}
                    </span>
                    <CopyButton
                      value={build.imageSha}
                      label={t("deploys.imageDigestCopy")}
                      successText={t("deploys.imageDigestCopied")}
                      errorText={t("deploys.imageDigestCopyError")}
                    />
                  </span>
                ) : null}
              </dd>
            </div>
          ) : null}
        </dl>
      </div>

      {/* Fan-out deploys & skipped services */}
      <Card data-region="build-fanout" className="gap-3 py-4">
        <CardHeader className="px-4 py-0">
          <CardTitle as="h3" className="text-sm font-medium">
            {t("buildSources.fanoutTitle")}
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-3 px-4">
          {build.deploys.length > 0 ? (
            <ul className="divide-y rounded-md border">
              {build.deploys.map((dep) => (
                <li
                  key={`${dep.serviceId}:${dep.deployId}`}
                  className="flex flex-wrap items-center justify-between gap-2 px-3 py-2 text-sm"
                >
                  <Link
                    to="/services/$serviceId"
                    params={{ serviceId: dep.serviceId }}
                    className="font-medium hover:underline"
                  >
                    {serviceNameById.get(dep.serviceId) ?? dep.serviceId}
                  </Link>
                  <Link
                    to="/services/$serviceId/deploys/$deployId"
                    params={{
                      serviceId: dep.serviceId,
                      deployId: dep.deployId,
                    }}
                    className="text-primary inline-flex items-center gap-1 font-mono text-xs hover:underline"
                  >
                    {dep.deployId}
                    <ExternalLink className="size-3" aria-hidden="true" />
                  </Link>
                </li>
              ))}
            </ul>
          ) : (
            <p className="text-muted-foreground text-sm">
              {t("buildSources.fanoutEmpty")}
            </p>
          )}

          {build.skippedServices.length > 0 ? (
            <Alert variant="destructive" data-testid="build-skipped-services">
              <AlertTriangle className="size-4" aria-hidden="true" />
              <AlertTitle>
                {t("buildSources.triggerResultSkippedTitle", {
                  count: build.skippedServices.length,
                })}
              </AlertTitle>
              <AlertDescription>
                <ul className="mt-1 space-y-1 text-xs">
                  {build.skippedServices.map((item) => (
                    <li
                      key={item.serviceId}
                      className="flex flex-wrap items-center justify-between gap-2"
                    >
                      <Link
                        to="/services/$serviceId"
                        params={{ serviceId: item.serviceId }}
                        className="font-medium hover:underline"
                      >
                        {serviceNameById.get(item.serviceId) ?? item.serviceId}
                      </Link>
                      <Badge variant="outline" className="text-xs">
                        {skippedReasonLabel(item.reason, t)}
                      </Badge>
                    </li>
                  ))}
                </ul>
              </AlertDescription>
            </Alert>
          ) : null}
        </CardContent>
      </Card>

      {/* Historical + live SSE build logs */}
      {isGit ? (
        <BuildLogPanel
          key={build.id}
          buildSourceId={buildSourceId}
          buildId={build.id}
          title={`${buildSource.name} · ${build.id}`}
          startTime={startedAt}
          endTime={endTime}
          startedAt={startedAt}
          followBuild={active}
          createEventSource={createEventSource}
        />
      ) : (
        <Card data-region="build-logs" className="py-6">
          <CardContent className="px-4 text-center">
            <p className="text-muted-foreground text-sm">
              {t("buildSources.imageBuildLogsNotice")}
            </p>
          </CardContent>
        </Card>
      )}
    </div>
  );
}
