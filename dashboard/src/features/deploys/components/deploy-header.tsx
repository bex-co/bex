import {
  CalendarDays,
  GitCommitHorizontal,
  Layers,
  Package,
  SquareStack,
  Terminal,
  Timer,
  Zap,
} from "lucide-react";
import { Link } from "@tanstack/react-router";
import { Fragment, useId, type ReactNode } from "react";
import { CopyButton } from "@/common/components/copy-button";
import { Skeleton } from "@/common/components/ui/skeleton";
import { useTranslations } from "@/common/hooks/use-translations";
import { useLocalDateTime } from "@/common/hooks/use-local-date";
import { cn } from "@/common/lib/utils/utils";
import {
  deployStatusKey,
  deployStatusKind,
  deployTriggerLabel,
  preDeployValueKey,
  type DeployStatusKind,
} from "@/features/deploys/lib/deploy-status";
import {
  deployDurationValue,
  deploySubject,
  deployTitle,
} from "@/features/deploys/lib/deploy-presentation";
import { repoCommitUrl } from "@/features/services/lib/repo";
import { CommitSha } from "./commit-sha";
import { DeployHeaderSkeleton } from "./deploy-page-skeletons";
import { DeployStatusIcon } from "./deploy-status-icon";
import {
  DEPLOY_FACT_CLASS,
  DEPLOY_FACT_ICON_CLASS,
  DEPLOY_FACT_FULL_CLASS,
  DEPLOY_FACTS_GRID_CLASS,
  DEPLOY_HEADER_CLASS,
  DEPLOY_ID_ROW_CLASS,
  DEPLOY_TITLE_ROW_CLASS,
} from "./deploy-page-layout";
import type { DeployView } from "../hooks/use-deploy";
import type { DeployScalingFact } from "../hooks/use-deploy-scaling";

/** "sha256:" plus the first 12 hex digits, Docker's short image id length. */
function shortDigest(digest: string): string {
  const [algorithm, hex] = digest.split(":");
  return hex ? `${algorithm}:${hex.slice(0, 12)}` : digest;
}

export interface DeployHeaderProps {
  /** Nullish while the header `deploy` query is still in flight (w9/m62 t002). */
  deploy?: DeployView | null;
  actions?: ReactNode;
  /** The service's repo, which links the commit to its diff; none for an
   *  image-backed service. */
  repo?: string | null;
  /** Linked Build Source id (`bds-…`) when the service builds from a Build Source. */
  buildSourceId?: string | null;
  /** Human-readable Build Source name when loaded. */
  buildSourceName?: string | null;
  /** Render's Scaling fact (w1/m661), from useDeployScaling: absent when it
   *  does not show, "loading" while its count is on its way. */
  scaling?: DeployScalingFact;
}

/**
 * The deploy page's header (w9/m1/t003), laid out as Render's deploy page
 * is: the commit's subject as the page's title with the deploy's actions
 * beside it, then a grid of facts — Status, Duration, Deployed, Trigger,
 * Source, Scaling (w1/m661) and Pre-deploy — each under a small label beside
 * an icon tile, and the image across the grid's width. The
 * status words and icon come from the mappings the Deploys list and Events
 * tab use, so the surfaces cannot drift on what a status looks like.
 */
export function DeployHeader({
  deploy,
  actions,
  repo,
  buildSourceId,
  buildSourceName,
  scaling,
}: DeployHeaderProps) {
  const { t } = useTranslations();
  const titleId = useId();
  // Absolute deploy times are the viewer's local timezone, deferred past
  // hydration (w6/m107). Read before the early `!deploy` return so the hook
  // order stays stable.
  const deployedAt = useLocalDateTime(deploy?.createdAt);
  const commitCreatedAt = useLocalDateTime(deploy?.commitCreatedAt);
  if (!deploy) return <DeployHeaderSkeleton />;

  const subject = deploySubject(deploy.commitMessage);
  const kind = deployStatusKind(deploy.status);
  const statusText = t(
    deployStatusKey(deploy.status) as Parameters<typeof t>[0],
  );
  const preDeploy = preDeployValueKey(deploy.preDeployStatus);
  // Duration is an elapsed span, not a wall-clock reading — timezone-neutral,
  // so it renders on the SSR pass too. A deploy that never started has
  // nothing to time and shows no fact.
  const duration = deployDurationValue(deploy, t);
  const commitUrl =
    repo && deploy.commitId ? repoCommitUrl(repo, deploy.commitId) : null;

  return (
    <section aria-labelledby={titleId} className={DEPLOY_HEADER_CLASS}>
      <div className={DEPLOY_TITLE_ROW_CLASS}>
        <div className="min-w-0 flex-1 space-y-1">
          {/* Render titles a deploy by its commit's subject. An image-backed
              deploy has none; its id is the only name it has. */}
          <h2
            id={titleId}
            className={cn(
              "text-lg leading-7 font-semibold tracking-tight break-words text-pretty",
              !subject && "font-mono",
            )}
          >
            {deployTitle(deploy.commitMessage, deploy.id)}
          </h2>
          {subject ? (
            <div className={DEPLOY_ID_ROW_CLASS}>
              <span className="truncate">{deploy.id}</span>
              <CopyButton
                value={deploy.id}
                label={t("common.copyField", { field: t("deploys.idLabel") })}
                successText={t("deploys.idCopied")}
                errorText={t("deploys.idCopyFailed")}
              />
            </div>
          ) : null}
        </div>
        {actions}
      </div>

      <dl className={DEPLOY_FACTS_GRID_CLASS}>
        {/* "Did it go live?" is the page's first question, so Status alone
            carries its state's colour: a tinted tile, the coloured word, and
            on a phone (no tiles) the state's glyph beside the word. */}
        <DeployFact
          icon={<DeployStatusIcon status={deploy.status} />}
          label={t("deploys.factStatus")}
          tileClassName={STATUS_TILE_CLASS[kind]}
        >
          <span
            className={cn(
              "flex items-center gap-1.5 font-medium",
              STATUS_TEXT_CLASS[kind],
            )}
          >
            <DeployStatusIcon
              status={deploy.status}
              className="size-4 @md/page:hidden"
            />
            {statusText}
          </span>
        </DeployFact>
        {duration ? (
          <DeployFact icon={<Timer />} label={t("deploys.duration")}>
            {duration}
          </DeployFact>
        ) : null}
        {deployedAt ? (
          <DeployFact icon={<CalendarDays />} label={t("deploys.factDeployed")}>
            {deployedAt}
          </DeployFact>
        ) : null}
        <DeployFact icon={<Zap />} label={t("deploys.columnTrigger")}>
          {deployTriggerLabel(deploy.trigger, deploy.rollbackOf, t)}
        </DeployFact>
        {/* The resolved commit this deploy ran (w9/001 + w2/m42 + w4/m108
            t004), linked to its diff when the repo is browsable. Absent
            (image-backed, or an unresolvable ref) => omitted, not faked. */}
        {deploy.commitId ? (
          <DeployFact
            icon={<GitCommitHorizontal />}
            label={t("deploys.factSource")}
          >
            <CommitSha commitId={deploy.commitId} url={commitUrl} />
            {commitCreatedAt ? (
              // A real text separator, not a margin (w6/m45 t004): a
              // screen reader and a copied line keep the two apart.
              <span className="text-muted-foreground">
                {" "}
                · {commitCreatedAt}
              </span>
            ) : null}
          </DeployFact>
        ) : null}
        {deploy.buildId ? (
          <DeployFact icon={<Layers />} label={t("deploys.factBuild")}>
            <span
              className="inline-flex min-w-0 flex-wrap items-center gap-1 font-mono"
              data-testid="deploy-build-id"
            >
              {buildSourceId ? (
                <Link
                  to="/build-sources/$buildSourceId/builds/$buildId"
                  params={{ buildSourceId, buildId: deploy.buildId }}
                  className="text-primary break-all hover:underline"
                >
                  {deploy.buildId}
                </Link>
              ) : (
                <span className="break-all">{deploy.buildId}</span>
              )}
              <CopyButton
                value={deploy.buildId}
                label={t("common.copyField", {
                  field: t("deploys.buildIdLabel"),
                })}
                successText={t("deploys.buildIdCopied")}
                errorText={t("deploys.buildIdCopyFailed")}
              />
            </span>
          </DeployFact>
        ) : null}
        {buildSourceId ? (
          <DeployFact icon={<Layers />} label={t("deploys.factBuildSource")}>
            <span
              className="inline-flex min-w-0 flex-wrap items-center gap-x-1.5 gap-y-0.5"
              data-testid="deploy-build-source"
            >
              <Link
                to="/build-sources/$buildSourceId"
                params={{ buildSourceId }}
                className="text-primary font-medium break-words hover:underline"
              >
                {buildSourceName ?? buildSourceId}
              </Link>
              <span className="inline-flex min-w-0 items-center gap-1 font-mono text-xs text-muted-foreground">
                {buildSourceName ? (
                  <span className="break-all">{buildSourceId}</span>
                ) : null}
                <CopyButton
                  value={buildSourceId}
                  label={t("common.copyField", {
                    field: t("deploys.buildSourceIdLabel"),
                  })}
                  successText={t("deploys.buildSourceIdCopied")}
                  errorText={t("deploys.buildSourceIdCopyFailed")}
                />
              </span>
            </span>
          </DeployFact>
        ) : null}
        {/* How many of the service's instances run this deploy (w1/m661):
            Render's fact after Source, for a service that autoscales or runs
            more than one instance. */}
        {scaling ? (
          <DeployFact icon={<SquareStack />} label={t("deploys.factScaling")}>
            {scaling === "loading" ? (
              <span className="flex h-5 items-center">
                <Skeleton className="h-3.5 w-24" />
              </span>
            ) : (
              <ScalingValue {...scaling} />
            )}
          </DeployFact>
        ) : null}
        {preDeploy ? (
          <DeployFact icon={<Terminal />} label={t("deploys.factPreDeploy")}>
            <span
              className={
                deploy.preDeployStatus === "failed"
                  ? "text-destructive"
                  : undefined
              }
            >
              {t(preDeploy as Parameters<typeof t>[0])}
            </span>
          </DeployFact>
        ) : null}
        {/* The image the deploy ran, across the grid's full width so the
            whole reference fits rather than being cut to its first letters.
            Render's image.sha (w8/m122): unknown => omitted, not faked. */}
        {deploy.image ? (
          <DeployFact
            icon={<Package />}
            label={t("services.sourceImageLabel")}
            className={DEPLOY_FACT_FULL_CLASS}
          >
            <span className="flex min-w-0 flex-wrap items-center gap-x-2 font-mono">
              <ImageReference image={deploy.image} />
              {deploy.imageSha ? (
                <span className="inline-flex shrink-0 items-center gap-1 text-muted-foreground">
                  <span title={deploy.imageSha}>
                    {shortDigest(deploy.imageSha)}
                  </span>
                  <CopyButton
                    value={deploy.imageSha}
                    label={t("deploys.imageDigestCopy")}
                    successText={t("deploys.imageDigestCopied")}
                    errorText={t("deploys.imageDigestCopyError")}
                  />
                </span>
              ) : null}
            </span>
          </DeployFact>
        ) : null}
      </dl>
    </section>
  );
}

// The Status fact's tint and word colour by where the deploy is in its life,
// from the classification its icon uses: the deploy that is live now speaks
// in green, one that went live before keeps a green tile under a plain word.
const STATUS_TILE_CLASS: Record<DeployStatusKind, string> = {
  live: "bg-primary/10",
  succeeded: "bg-primary/10",
  failed: "bg-destructive/10",
  canceled: "bg-muted",
  in_progress: "bg-muted",
  pending: "bg-muted",
};

const STATUS_TEXT_CLASS: Record<DeployStatusKind, string> = {
  live: "text-primary",
  succeeded: "text-foreground",
  failed: "text-destructive",
  canceled: "text-foreground",
  in_progress: "text-foreground",
  pending: "text-foreground",
};

// Stand-ins for the two numbers while translating, so the words around them
// come out in the language's own order and the numbers can be set apart.
// Private-use characters, which no translation contains.
const DEPLOYED_SENTINEL = "\uE000";
const TOTAL_SENTINEL = "\uE001";

/** "1 of 2 instances", the numbers in strong monospace as on Render. */
function ScalingValue({
  deployed,
  total,
}: {
  deployed: number;
  total: number;
}) {
  const { t } = useTranslations();
  const words = t("deploys.scalingInstances", {
    count: total,
    deployed: DEPLOYED_SENTINEL,
    total: TOTAL_SENTINEL,
  });
  return (
    <span className="text-muted-foreground" data-testid="deploy-scaling">
      {words.split(/([\uE000\uE001])/).map((part, i) =>
        part === DEPLOYED_SENTINEL || part === TOTAL_SENTINEL ? (
          <strong
            key={i}
            className="font-mono leading-none font-medium text-foreground"
          >
            {part === DEPLOYED_SENTINEL ? deployed : total}
          </strong>
        ) : (
          part
        ),
      )}
    </span>
  );
}

/**
 * An image reference, whole: where the line is too narrow it breaks after a
 * slash ("registry.example.com/" over "eden-cms-v2:a1b2c3d"), never inside a
 * name, and is never cut — the registry, name and tag each tell two images
 * apart.
 */
function ImageReference({ image }: { image: string }) {
  return (
    <span className="min-w-0 [overflow-wrap:anywhere]">
      {image.split("/").map((part, i) => (
        <Fragment key={i}>
          {i > 0 ? (
            <>
              /<wbr />
            </>
          ) : null}
          {part}
        </Fragment>
      ))}
    </span>
  );
}

/** One labelled fact; its icon tile sits in the term, decorative. */
function DeployFact({
  icon,
  label,
  className,
  tileClassName = "bg-muted",
  children,
}: {
  icon: ReactNode;
  label: string;
  className?: string;
  /** The icon tile's fill; Status tints it by state. */
  tileClassName?: string;
  children: ReactNode;
}) {
  return (
    <div className={cn(DEPLOY_FACT_CLASS, className)}>
      {/* Small tracked capitals carry a Latin label; CJK has no capitals to
          carry, so it reads at 12px without the tracking. */}
      <dt className="text-[11px] leading-4 font-medium tracking-wider text-muted-foreground uppercase [&:is(:lang(zh),:lang(ja),:lang(ko))]:text-xs [&:is(:lang(zh),:lang(ja),:lang(ko))]:tracking-normal">
        <span
          aria-hidden
          className={cn(
            DEPLOY_FACT_ICON_CLASS,
            "items-center justify-center text-muted-foreground [&_svg]:size-[18px]",
            tileClassName,
          )}
        >
          {icon}
        </span>
        {label}
      </dt>
      {/* Balanced, so a value that wraps breaks evenly ("July 11, 2026 /
          at 7:30 AM"), never leaving one word alone on its last line. */}
      <dd className="text-sm leading-5 text-balance break-words">{children}</dd>
    </div>
  );
}
