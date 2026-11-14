import { useState, type ReactNode } from "react";
import { Hammer, Loader2 } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/common/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/common/components/ui/dialog";
import { Badge } from "@/common/components/ui/badge";
import { Skeleton } from "@/common/components/ui/skeleton";
import { useIsHydrated } from "@/common/hooks/use-is-hydrated";
import { useLocalDateTime } from "@/common/hooks/use-local-date";
import { useTranslations } from "@/common/hooks/use-translations";
import { hasGraphQLErrorCode } from "@/common/lib/graphql-error";
import { ownValue } from "@/common/lib/own-value";
import { formatBytes, formatMediumDateTime } from "@/common/lib/format";
import { cn } from "@/common/lib/utils/utils";
import {
  useAgentRepoEnvironmentBuild,
  useAgentRepoEnvironmentBuilds,
  type RepoEnvironmentBuildDetail,
} from "@/features/agent-environment/hooks/use-agent-repo-environment-builds";
import { RepoEnvironmentBuildBadge } from "@/features/agent-environment/components/repo-environment-build-badge";
import {
  AgentRepoEnvironmentBuildPanelHeader,
  AgentRepoEnvironmentBuildsFrame,
  AgentRepoEnvironmentBuildsSkeleton,
  BUILD_ROW_CLASS,
} from "@/features/agent-environment/components/agent-repo-environment-builds-skeleton";
import {
  buildDurationMs,
  buildFailureKey,
  buildInstant,
  buildNowRefusal,
  buildTriggerKey,
  formatBuildDuration,
  type BuildNowRefusal,
  type RepoEnvironmentBuildStepView,
  type RepoEnvironmentBuildView,
} from "@/features/agent-environment/lib/repo-environment-builds";
import type { RepoEnvironmentView } from "@/features/agent-environment/lib/repo-environments";

interface AgentRepoEnvironmentBuildsDialogProps {
  ownerId: string;
  environment: RepoEnvironmentView;
  /** Admins only (can_manage): whether Build now is enabled (ADR111 D10). */
  canManage: boolean;
  onOpenChange: (open: boolean) => void;
}

/**
 * A repository's environment builds (ADR111 D10): Build now with Devin's
 * warning that files the setup steps write stay in the snapshot, the last 20
 * builds, and the selected build's log, step by step with redacted output.
 * Members read it; only admins start a build.
 */
export function AgentRepoEnvironmentBuildsDialog({
  ownerId,
  environment,
  canManage,
  onOpenChange,
}: AgentRepoEnvironmentBuildsDialogProps) {
  const { t } = useTranslations();
  const { repo } = environment;
  const list = useAgentRepoEnvironmentBuilds(ownerId, repo);
  const [picked, setPicked] = useState<string | null>(null);
  const selected =
    picked ?? list.builds[0]?.id ?? environment.latestBuild?.id ?? null;
  const detail = useAgentRepoEnvironmentBuild(ownerId, selected);
  const [refusal, setRefusal] = useState<BuildNowRefusal | null>(null);
  const canBuild = environment.setup.length > 0;

  async function buildNow() {
    setRefusal(null);
    try {
      const build = await list.buildNow();
      setPicked(build.id);
      toast.success(t("agentEnvironment.repoEnvs.builds.queued", { repo }));
    } catch (err) {
      setRefusal(buildNowRefusal(err));
    }
  }

  // The panel names the freshest copy: the log read polls on its own.
  const shown: RepoEnvironmentBuildView | null =
    detail.build ??
    list.builds.find((b) => b.id === selected) ??
    (environment.latestBuild?.id === selected ? environment.latestBuild : null);

  let body: ReactNode;
  if (list.loading) {
    body = <AgentRepoEnvironmentBuildsSkeleton />;
  } else if (list.error && list.builds.length === 0) {
    body = (
      <div
        className="flex h-40 flex-col items-start justify-center gap-3 md:h-96"
        role="alert"
      >
        <p className="text-destructive text-sm">
          {hasGraphQLErrorCode(
            list.error,
            "AGENT_REPO_ENVIRONMENT_BUILDS_UNAVAILABLE",
          )
            ? t("agentEnvironment.repoEnvs.builds.unavailable")
            : t("agentEnvironment.repoEnvs.builds.loadFailed")}
        </p>
        <Button size="sm" variant="outline" onClick={list.retry}>
          {t("agentEnvironment.retry")}
        </Button>
      </div>
    );
  } else {
    body = (
      <AgentRepoEnvironmentBuildsFrame
        list={
          list.builds.length === 0 ? (
            <p className="text-muted-foreground p-3 text-sm">
              {t("agentEnvironment.repoEnvs.builds.none")}
            </p>
          ) : (
            <ul
              data-testid="repo-environment-build-history"
              aria-label={t("agentEnvironment.repoEnvs.builds.historyLabel")}
            >
              {list.builds.map((build) => (
                <li key={build.id} className="border-b">
                  <button
                    type="button"
                    aria-current={build.id === selected}
                    data-testid={`build-${build.id}`}
                    onClick={() => setPicked(build.id)}
                    className={cn(
                      BUILD_ROW_CLASS,
                      "hover:bg-muted/60 gap-1 px-3 text-left",
                      build.id === selected && "bg-muted",
                    )}
                  >
                    <span className="flex min-w-0 items-center gap-2">
                      <RepoEnvironmentBuildBadge status={build.status} />
                      <span className="truncate text-xs">
                        <BuildTrigger trigger={build.trigger} />
                      </span>
                    </span>
                    <span className="text-muted-foreground truncate text-xs">
                      <BuildMeta build={build} />
                    </span>
                  </button>
                </li>
              ))}
            </ul>
          )
        }
        panel={
          <>
            <AgentRepoEnvironmentBuildPanelHeader>
              {shown ? (
                <>
                  <span className="flex min-w-0 items-center gap-2">
                    <RepoEnvironmentBuildBadge status={shown.status} />
                    <span className="truncate text-sm">
                      <BuildTrigger trigger={shown.trigger} />
                    </span>
                  </span>
                  <span
                    className="text-muted-foreground truncate text-xs"
                    data-testid="build-details"
                  >
                    <BuildMeta build={shown} details />
                  </span>
                </>
              ) : (
                <span className="text-muted-foreground text-sm">
                  {t("agentEnvironment.repoEnvs.builds.logTitle")}
                </span>
              )}
            </AgentRepoEnvironmentBuildPanelHeader>
            <div
              className="min-h-0 flex-1 overflow-auto"
              data-testid="build-log"
            >
              <BuildLog
                selected={selected}
                shown={shown}
                build={detail.build}
                loading={detail.loading}
                error={detail.error}
                onRetry={detail.retry}
              />
            </div>
          </>
        }
      />
    );
  }

  return (
    <Dialog open onOpenChange={onOpenChange}>
      <DialogContent
        className="max-h-[90vh] grid-cols-[minmax(0,1fr)] overflow-y-auto sm:max-w-3xl"
        data-testid="agent-repo-environment-builds-dialog"
      >
        <DialogHeader>
          <DialogTitle>
            {t("agentEnvironment.repoEnvs.builds.title", { repo })}
          </DialogTitle>
          <DialogDescription>
            {t("agentEnvironment.repoEnvs.builds.description")}
          </DialogDescription>
        </DialogHeader>
        <div className="min-w-0 space-y-4">
          <div className="space-y-2" data-testid="build-now-bar">
            <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:gap-3">
              <Button
                type="button"
                className="self-start sm:self-auto"
                disabled={!canManage || !canBuild || list.starting}
                onClick={() => void buildNow()}
              >
                {list.starting ? (
                  <Loader2 className="animate-spin" />
                ) : (
                  <Hammer />
                )}
                {t("agentEnvironment.repoEnvs.builds.buildNow")}
              </Button>
              <p
                className="text-muted-foreground text-xs"
                data-testid="build-now-note"
              >
                {t("agentEnvironment.repoEnvs.builds.snapshotNote")}
              </p>
            </div>
            {!canManage ? (
              <p className="text-muted-foreground text-xs">
                {t("agentEnvironment.repoEnvs.builds.readOnly")}
              </p>
            ) : !canBuild ? (
              <p className="text-muted-foreground text-xs">
                {t("agentEnvironment.repoEnvs.builds.nothingToBuild")}
              </p>
            ) : null}
            {refusal ? <BuildNowRefusalMessage refusal={refusal} /> : null}
          </div>
          {body}
        </div>
        <DialogFooter>
          <Button
            type="button"
            variant="outline"
            onClick={() => onOpenChange(false)}
          >
            {t("agentEnvironment.repoEnvs.close")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

/** A refused Build now; the rate limit names when the next build may start. */
function BuildNowRefusalMessage({ refusal }: { refusal: BuildNowRefusal }) {
  const { t } = useTranslations();
  const time = useLocalDateTime(refusal.nextBuildAt);
  return (
    <p className="text-destructive text-sm" role="alert">
      {refusal.nextBuildAt && time
        ? t(refusal.key, { time })
        : t(
            refusal.nextBuildAt
              ? "agentEnvironment.repoEnvs.builds.rateLimited"
              : refusal.key,
          )}
    </p>
  );
}

/** What queued a build, in words (ADR111 D3). */
function BuildTrigger({ trigger }: { trigger: string }) {
  const { t } = useTranslations();
  const key = buildTriggerKey(trigger);
  return <>{key ? t(key) : trigger}</>;
}

/**
 * A build's time and how long it took; with `details`, also the commit it
 * checked out, the snapshot's size and whether sessions can still boot it.
 */
function BuildMeta({
  build,
  details = false,
}: {
  build: RepoEnvironmentBuildView;
  details?: boolean;
}) {
  const { t, i18n } = useTranslations();
  // The viewer's clock: absent from the SSR pass (w6/m107).
  const hydrated = useIsHydrated();
  const instant = buildInstant(build);
  const duration = buildDurationMs(build);
  const parts = [
    hydrated && instant ? formatMediumDateTime(instant, i18n.language) : "",
    duration != null
      ? t("agentEnvironment.repoEnvs.builds.took", {
          duration: formatBuildDuration(duration),
        })
      : "",
  ];
  if (details) {
    parts.push(build.headSha ? build.headSha.slice(0, 7) : "");
    parts.push(build.sizeBytes != null ? formatBytes(build.sizeBytes, 1) : "");
    if (build.status === "success" && !build.imageAvailable)
      parts.push(t("agentEnvironment.repoEnvs.builds.imageRemoved"));
  }
  return <>{parts.filter((part) => part !== "").join(" · ")}</>;
}

/** Why a build has no step to show yet, by its status. */
const NO_STEPS_KEYS: Record<string, string> = {
  queued: "agentEnvironment.repoEnvs.builds.noSteps.queued",
  building: "agentEnvironment.repoEnvs.builds.noSteps.building",
  cancelled: "agentEnvironment.repoEnvs.builds.noSteps.cancelled",
  failed: "agentEnvironment.repoEnvs.builds.noSteps.failed",
};

/** The selected build's log: its failure, then each setup step. */
function BuildLog({
  selected,
  shown,
  build,
  loading,
  error,
  onRetry,
}: {
  selected: string | null;
  shown: RepoEnvironmentBuildView | null;
  build: RepoEnvironmentBuildDetail | null;
  loading: boolean;
  error: Error | undefined;
  onRetry: () => void;
}) {
  const { t } = useTranslations();
  if (selected == null) {
    return (
      <p className="text-muted-foreground p-3 text-sm">
        {t("agentEnvironment.repoEnvs.builds.noneLog")}
      </p>
    );
  }
  if (error && !build) {
    return (
      <div className="flex flex-col items-start gap-3 p-3" role="alert">
        <p className="text-destructive text-sm">
          {t("agentEnvironment.repoEnvs.builds.logFailed")}
        </p>
        <Button size="sm" variant="outline" onClick={onRetry}>
          {t("agentEnvironment.retry")}
        </Button>
      </div>
    );
  }
  if (loading || !build) {
    return (
      <div className="space-y-2 p-3" data-testid="build-log-loading">
        <Skeleton className="h-4 w-1/4" />
        <Skeleton className="h-16 w-full" />
      </div>
    );
  }
  const status = shown?.status ?? build.status;
  const steps: RepoEnvironmentBuildStepView[] = build.steps ?? [];
  return (
    <div className="space-y-3 p-3">
      {status === "failed" ? (
        <div
          className="border-destructive/50 rounded-md border p-3"
          data-testid="build-failure"
        >
          <p className="text-destructive text-sm font-medium">
            {t(buildFailureKey(build.failureCode))}
          </p>
          {build.failureReason ? (
            <p className="text-muted-foreground mt-1 font-mono text-xs break-words">
              {build.failureReason}
            </p>
          ) : null}
        </div>
      ) : null}
      {steps.length === 0 ? (
        <p className="text-muted-foreground text-sm">
          {t(
            ownValue(NO_STEPS_KEYS, status) ??
              "agentEnvironment.repoEnvs.builds.noSteps.other",
          )}
        </p>
      ) : (
        <ol className="space-y-3" data-testid="build-steps">
          {steps.map((step, i) => (
            <BuildStep key={`${i}-${step.name}`} step={step} />
          ))}
        </ol>
      )}
    </div>
  );
}

/** One setup step: its name, exit and duration, then its redacted output. */
function BuildStep({ step }: { step: RepoEnvironmentBuildStepView }) {
  const { t } = useTranslations();
  return (
    <li className="space-y-1" data-testid="build-step">
      <div className="flex min-w-0 items-center gap-2">
        <span className="truncate font-mono text-sm font-medium">
          {step.name}
        </span>
        <Badge
          variant={step.exitCode === 0 ? "success" : "destructive"}
          data-testid="build-step-exit"
        >
          {step.exitCode == null
            ? t("agentEnvironment.repoEnvs.builds.stepKilled")
            : t("agentEnvironment.repoEnvs.builds.stepExit", {
                code: step.exitCode,
              })}
        </Badge>
        <span className="text-muted-foreground ml-auto shrink-0 text-xs">
          {formatBuildDuration(step.durationMs)}
        </span>
      </div>
      {step.output ? (
        <pre className="bg-muted max-h-64 overflow-auto rounded-md p-2 font-mono text-xs break-all whitespace-pre-wrap">
          {step.output}
        </pre>
      ) : (
        <p className="text-muted-foreground text-xs">
          {t("agentEnvironment.repoEnvs.builds.noOutput")}
        </p>
      )}
    </li>
  );
}
