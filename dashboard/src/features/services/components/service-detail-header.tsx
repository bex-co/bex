import { useId, useState, type ReactNode } from "react";
import { Link } from "@tanstack/react-router";
import {
  AlertTriangle,
  ChevronDown,
  KeyRound,
  Network,
  SquareTerminal,
  Terminal,
} from "lucide-react";
import { GithubIcon } from "@/common/components/github-icon";
import { Badge } from "@/common/components/ui/badge.tsx";
import {
  Alert,
  AlertDescription,
  AlertTitle,
} from "@/common/components/ui/alert.tsx";
import { Button } from "@/common/components/ui/button";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/common/components/ui/popover";
import { Skeleton } from "@/common/components/ui/skeleton.tsx";
import {
  Tooltip,
  TooltipTrigger,
  TooltipContent,
} from "@/common/components/ui/tooltip";
import { CopyButton } from "@/common/components/copy-button";
import {
  ReservedBadge,
  ReservedButton,
  ReservedText,
} from "@/common/components/reserved-skeleton";
import { LocalDateTimeWithZone } from "@/common/components/local-time";
import { RelativeAge } from "@/common/components/relative-time";
import { useIsHydrated } from "@/common/hooks/use-is-hydrated";
import {
  useTranslations,
  type Translate,
} from "@/common/hooks/use-translations";
import { formatInstantDetails } from "@/common/lib/format";
import { safeHttpHref } from "@/common/lib/external-url";
import { cn } from "@/common/lib/utils/utils";
import { useEnvironments } from "@/features/environments/hooks/use-environments";
import { useProjects } from "@/features/projects/hooks/use-projects";
import { AddSshKeyCta } from "@/features/ssh-keys/components/add-ssh-key-cta";
import { RequiresSshKey } from "@/features/ssh-keys/components/requires-ssh-key";
import { SLOT } from "@/common/lib/translation-slot";
import { CronTriggerRunButton } from "@/features/services/components/cron-header-actions";
import {
  SUSPENDED_NOTICE_CLASS,
  ServiceSuspendedNotice,
} from "@/features/services/components/service-suspended-notice";
import type { ProtectedActionResult } from "@/features/services/lib/protected-confirmation";
import { ManualDeployButton } from "@/features/services/components/manual-deploy-button";
import { ServiceStatusBadge } from "@/features/services/components/service-status-badge";
import { useInstanceTypes } from "@/features/services/hooks/use-instance-types";
import { formatRepoLabel, repoBrowseUrl } from "@/features/services/lib/repo";
import {
  deriveServiceType,
  isCron,
  isPrivateService,
  isStaticSite,
  SERVICE_TYPE_ICON,
  SERVICE_TYPE_LABEL,
} from "@/features/services/lib/service-type";
import { serviceBaseForType } from "@/features/services/lib/service-base";
import { describeCronRun } from "@/features/services/lib/cron-schedule-words";
import { isNativeRuntime } from "@/features/services/lib/runtime";
import { sshRemedy } from "@/features/services/lib/ssh-remedy";
import { computePlanLink } from "@/features/services/lib/compute-page";
import { autoSleepEligible } from "@/features/services/lib/idle-timeout";
import {
  openPortsDisplay,
  type OpenPortsDisplay,
} from "@/features/services/lib/port";
import {
  hasServed,
  isSleeping,
  runsInstances,
  isDeleting,
  undeployedChangesHintKey,
} from "@/features/services/lib/status";
import type { ServiceView, LifecycleAction } from "@/features/services/types";
import { useRegistryCredentials } from "@/features/registry-credentials/hooks/use-registry-credentials";
import { useShellGate } from "@/features/services/hooks/use-shell-gate";
import {
  deployStatusKey,
  deployStatusVariant,
} from "@/features/deploys/lib/deploy-status";
import type { LatestDeploySummary } from "@/features/deploys/hooks/use-latest-deploy";
import type { CachedServiceSource } from "@/features/services/hooks/use-cached-service-type";

export interface ServiceDetailHeaderProps {
  service: ServiceView;
  /** Newest control-plane deploy, named separately from operator/App phase. */
  latestDeploy?: LatestDeploySummary | null;
  /** The first read of it is still out: its place is held, so nothing below
   *  moves when it lands. */
  latestDeployLoading?: boolean;
  /** The lifecycle action in flight for this service, or null. */
  pending: LifecycleAction | null;
  /** Runs a lifecycle action; the header offers Resume while suspended. */
  onRun?: (
    action: LifecycleAction,
    service: ServiceView,
    confirmation?: string,
  ) => Promise<ProtectedActionResult>;
}

/**
 * The service-detail header, mirroring Render's service-page banner (captured
 * live from dashboard.render.com): an uppercase service-type eyebrow, the name
 * with its instance-type chip, Manual Deploy + the lifecycle actions, and a
 * metadata stack of Service ID / source repo / live URL. bex has no Overview tab
 * (neither does Render — the service root lands on Deploys), so this header is
 * where the identity facts live: it also carries the bex-native ones the retired
 * Overview panel showed — instances, revision, age.
 *
 * The action controls are Connect (SSH) and Manual Deploy (deploy/restart); the
 * header carries no "•••" lifecycle menu — restart lives in the Manual Deploy
 * dropdown (w2/m30) and Suspend/Resume moved to the Settings page. As on
 * Render, a header has actions only while the service runs: a suspended one
 * of any type has none, and Render's notice under its facts offers Resume
 * service instead (w3/m555, w3/107).
 */
export function ServiceDetailHeader({
  service,
  latestDeploy,
  latestDeployLoading = false,
  pending,
  onRun,
}: ServiceDetailHeaderProps) {
  const { t } = useTranslations();
  const { byID, loading: instanceTypesLoading } = useInstanceTypes();
  const typeKey = deriveServiceType(service.type);
  const TypeIcon = SERVICE_TYPE_ICON[typeKey];
  // Header links point at the service's canonical base (a static_site lives
  // under /static, Render parity w5/m57). The Plan chip below stays on
  // /services — plan has no /static route and renders for compute types only
  // (a static_site is gated out: its "Free" chip would link to a nonexistent
  // /static/<id>/plan and 404, and Render's static header has no plan concept).
  const base = serviceBaseForType(service.type);
  const instanceType = byID(service.plan);
  const repoUrl = service.repo
    ? repoBrowseUrl(service.repo, service.branch)
    : null;
  // Never place a non-http(s) scheme into an href (codex-security target #4).
  // A deleting service's route/certificate are withdrawn, so its URL is dead —
  // never render it as a live link (w3/m81). The backend already blanks the URL
  // in that state and by-id reads 404, so this is belt-and-suspenders for any
  // transient/stale render.
  const safeServiceUrl = isDeleting(service) ? null : safeHttpHref(service.url);
  const openPorts = openPortsDisplay(service);
  // Below the desktop layout the Service ID and the bex-native facts
  // (runtime, slug, instances, revision, age) start folded behind "Details",
  // and on a phone the source repository does too, so the tab's own content
  // is not pushed below the fold. From `lg` up everything always shows and
  // there is no toggle.
  const [factsOpen, setFactsOpen] = useState(false);
  const factsId = useId();
  const serviceIdRowId = useId();
  const repoRowId = useId();

  return (
    <div className="border-b px-4 py-3 max-sm:pb-2 sm:px-6">
      {/* The band is full-bleed; its content ends on the same right edge as
          the widest tab's content, so the actions sit above what they act
          on instead of in a far corner of a wide screen. */}
      <div className="mx-auto max-w-7xl space-y-2 max-sm:space-y-1.5">
        <div className="flex items-center justify-between gap-2">
          <div className="text-muted-foreground flex min-w-0 items-center gap-1.5 text-xs font-medium tracking-wide uppercase">
            <TypeIcon className="size-3.5 shrink-0" />
            <span className="shrink-0">{t(SERVICE_TYPE_LABEL[typeKey])}</span>
            <ServiceScopeLabel serviceId={service.id} />
          </div>
        </div>

        {/* Tablet widths: the name and the actions share the first row and the
          status items take the second, rather than the actions a third. */}
        <div className="flex flex-wrap items-center justify-between gap-3 md:max-lg:grid md:max-lg:grid-cols-[minmax(0,1fr)_auto] md:max-lg:gap-y-2 lg:flex-nowrap lg:items-start">
          <div className="flex min-w-0 flex-wrap items-center gap-2 md:max-lg:contents">
            <h1 className="truncate text-xl font-semibold">{service.name}</h1>
            {/* From `sm` the status items wrap under the name as one group
              rather than one at a time; a phone has no room for the group
              and lets each wrap on its own. */}
            <div className="contents sm:flex sm:min-w-0 sm:flex-wrap sm:items-center sm:gap-2 md:max-lg:col-span-2 md:max-lg:row-start-2">
              <span className="text-xs font-medium text-muted-foreground">
                {t("services.headerServicePhase")}
              </span>
              <ServiceStatusBadge service={service} />
              {latestDeploy ? (
                <Link
                  to={`${base}/$serviceId/deploys/$deployId`}
                  params={{ serviceId: service.id, deployId: latestDeploy.id }}
                  aria-label={`${t("services.headerLatestDeploy")}: ${t(
                    deployStatusKey(latestDeploy.status) as Parameters<
                      typeof t
                    >[0],
                  )}`}
                  // A 30px target from `md` (the badge is 22px), 46px on a
                  // phone where the rows have room.
                  className="relative flex items-center gap-1.5 rounded-md after:absolute after:inset-x-0 after:-inset-y-1 after:content-[''] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring max-md:after:-inset-y-3"
                >
                  <span className="text-xs font-medium text-muted-foreground">
                    {t("services.headerLatestDeploy")}
                  </span>
                  {/* The space keeps the visible words apart ("Latest deploy
                      Live"), so the link's name contains its label (WCAG
                      2.5.3); a flex container does not render it. */}{" "}
                  <Badge variant={deployStatusVariant(latestDeploy.status)}>
                    {t(
                      deployStatusKey(latestDeploy.status) as Parameters<
                        typeof t
                      >[0],
                    )}
                  </Badge>
                </Link>
              ) : latestDeployLoading ? (
                <span aria-hidden="true" className="flex items-center gap-1.5">
                  <span className="text-xs font-medium text-muted-foreground">
                    {t("services.headerLatestDeploy")}
                  </span>
                  <Skeleton className="h-[22px] w-10 rounded-full" />
                </span>
              ) : null}
              {isStaticSite(service) ? null : instanceType ? (
                // Render's plan badge opens where the plan is changed: a cron
                // job's Settings' Compute section, else the Compute page.
                <Link
                  {...computePlanLink(isCron(service))}
                  params={{ serviceId: service.id }}
                  className="relative flex after:absolute after:-inset-x-1 after:-inset-y-3 after:content-['']"
                >
                  <Badge variant="outline" className="hover:bg-accent">
                    {instanceType.name}
                  </Badge>
                </Link>
              ) : instanceTypesLoading && service.plan ? (
                <Skeleton
                  aria-hidden="true"
                  className="h-[22px] w-14 rounded-full"
                />
              ) : null}
            </div>
          </div>
          {/* Hidden while empty (a suspended service's): wrapped onto a line
              of its own, an empty slot still adds the row gap (w3/m555). */}
          <div className="flex shrink-0 items-center gap-2 empty:hidden max-md:[&_button]:min-h-11 md:max-lg:col-start-2 md:max-lg:row-start-1">
            {/* Render passes the header's actions only while the service
                runs; the notice below offers Resume (w3/107). */}
            {service.suspended ? null : isCron(service) ? (
              // Render's cron header: Trigger Run. Its build is Manual Build
              // on the Deploys page, and it has no SSH (w3/m555).
              <CronTriggerRunButton service={service} />
            ) : (
              <>
                {/* A static_site has no running instance to SSH into —
                  Render's static header carries only Manual Deploy
                  (w5/m48/t001). */}
                {!isStaticSite(service) && (
                  <ServiceConnectButton
                    service={service}
                    openPorts={openPorts}
                  />
                )}
                <ManualDeployButton
                  service={service}
                  pending={pending !== null}
                />
              </>
            )}
          </div>
        </div>

        {/* Service ID / source repo / live URL: stacked on a phone, where the
          links get touch-height rows and the "Details" toggle sits beside
          the first of them, and one wrapping row from `sm`. */}
        <div className="grid grid-cols-[minmax(0,1fr)_auto] gap-x-3 text-sm max-sm:pt-1.5 max-sm:[&>*]:col-start-1 sm:flex sm:flex-wrap sm:items-center sm:gap-x-6 sm:gap-y-0.5">
          <div
            id={serviceIdRowId}
            className={cn(
              "text-muted-foreground flex items-center gap-1.5",
              !factsOpen && "max-lg:hidden",
            )}
          >
            <span>{t("services.headerServiceId")}</span>
            <span className="text-foreground font-mono text-xs">
              {service.id}
            </span>
            <CopyButton
              value={service.id}
              label={t("services.headerCopyServiceId")}
              successText={t("services.headerCopied")}
              errorText={t("services.headerCopyError")}
            />
          </div>

          {service.repo ? (
            <div
              id={repoRowId}
              className={cn(
                "text-muted-foreground flex min-w-0 items-center gap-1.5",
                // A phone keeps the name, the statuses, the actions and the
                // live URL in view; the source is one tap away, right here.
                !factsOpen && "max-sm:hidden",
              )}
            >
              <GithubIcon className="size-3.5 shrink-0" />
              {repoUrl ? (
                <a
                  href={repoUrl}
                  target="_blank"
                  rel="noreferrer"
                  className={headerLinkClass("text-foreground", "up")}
                >
                  <span className="truncate">
                    <RepoLabel repo={service.repo} />
                  </span>
                </a>
              ) : (
                <span className="text-foreground truncate">
                  <RepoLabel repo={service.repo} />
                </span>
              )}
              {service.branch ? (
                <Badge variant="secondary" className="font-mono text-xs">
                  {service.branch}
                </Badge>
              ) : null}
            </div>
          ) : null}

          {service.registryCredentialId ? (
            <RegistryCredentialFact id={service.registryCredentialId} />
          ) : null}

          {isCron(service) ? (
            <CronRunFacts service={service} />
          ) : isPrivateService(service) && service.internalAddress ? (
            // A private service has no public URL — Render's header shows its
            // Service Address (the private-network `<slug>:<port>`) as copyable
            // text, never a link (the old cluster-internal href was dead from a
            // browser). ADR041 D4, w9/m58. An image-port service also lists the
            // other TCP ports its hostname serves (w6/m148).
            <ServiceAddressFact
              address={service.internalAddress}
              openPorts={openPorts}
            />
          ) : safeServiceUrl ? (
            <div className="flex min-w-0 items-center gap-1.5">
              <a
                href={safeServiceUrl}
                target="_blank"
                rel="noreferrer noopener"
                className={headerLinkClass("text-primary", "down")}
              >
                <span className="truncate">{service.url}</span>
              </a>
              <CopyButton
                value={service.url ?? ""}
                label={t("services.headerCopyUrl")}
                successText={t("services.headerCopied")}
                errorText={t("services.headerCopyError")}
              />
            </div>
          ) : service.url ? (
            // URL present but not a safe http(s) scheme: show it as inert,
            // copyable text rather than a live link (codex-security target #4).
            <div className="flex min-w-0 items-center gap-1.5">
              <span className="text-foreground truncate">{service.url}</span>
              <CopyButton
                value={service.url}
                label={t("services.headerCopyUrl")}
                successText={t("services.headerCopied")}
                errorText={t("services.headerCopyError")}
              />
            </div>
          ) : service.publicRoutingNotice ? (
            // No url, and the platform knows why (w7/m79). This slot used to
            // render nothing at all, which is how a service that would never be
            // publicly reachable looked identical to one still starting up.
            // The text is the operator's own diagnosis, rendered raw like a
            // deploy's failureReason — it names the missing piece and the fix.
            <p className="text-muted-foreground text-xs">
              {service.publicRoutingNotice}
            </p>
          ) : null}
          {/* Beside the block it opens, and what it opens appears in and
              under that block. */}
          <button
            type="button"
            aria-expanded={factsOpen}
            aria-controls={`${serviceIdRowId} ${repoRowId} ${factsId}`}
            onClick={() => setFactsOpen((open) => !open)}
            className="text-muted-foreground hover:text-foreground focus-visible:ring-ring relative row-start-1 flex items-center gap-1 self-center rounded-sm text-xs font-medium after:absolute after:-inset-y-3.5 after:-right-3 after:-left-1 after:content-[''] focus-visible:ring-2 focus-visible:outline-none max-sm:col-start-2! sm:ml-auto lg:hidden"
          >
            {t("services.headerMoreFacts")}
            <ChevronDown
              aria-hidden="true"
              className={cn(
                "size-3.5 transition-transform",
                factsOpen && "rotate-180",
              )}
            />
          </button>
        </div>

        <HeaderFacts service={service} id={factsId} open={factsOpen} />

        {service.suspended ? (
          <ServiceSuspendedNotice
            service={service}
            pending={pending}
            onRun={onRun}
          />
        ) : null}

        {service.undeployedChanges ? (
          // Save only, cancellation and rollback can leave saved settings different
          // from the runtime. Restart preserves it. Read the operator's distinction
          // instead of inferring it from the most recent deploy's status.
          <p className="text-muted-foreground text-sm" role="status">
            {t(undeployedChangesHintKey(service), {
              running: service.servingImage ?? "",
              saved: service.imagePath ?? "",
            })}
          </p>
        ) : null}

        {isSleeping(service) ? (
          <p className="text-muted-foreground text-sm">
            {t("services.statusSleepingHint")}
          </p>
        ) : null}

        {service.maintenanceMode?.enabled ? (
          <Alert variant="destructive">
            <AlertTriangle />
            <AlertTitle>{t("services.maintenanceModeBannerTitle")}</AlertTitle>
            <AlertDescription>
              {t("services.maintenanceModeBannerBody")}
            </AlertDescription>
          </Alert>
        ) : null}
      </div>
    </div>
  );
}

/**
 * Where the service lives — "storefront / Production" — beside its type, below
 * `lg`. From `lg` the top bar's breadcrumbs name the project and environment;
 * narrower bars keep only the service crumb, so without this a phone showed no
 * sign of which environment's secrets were on screen. Reads the breadcrumbs'
 * own cached queries; nothing renders until the project is known.
 */
function ServiceScopeLabel({ serviceId }: { serviceId: string }) {
  const { projects } = useProjects({ poll: false });
  const project = projects.find((item) => item.serviceIds.includes(serviceId));
  const { environments } = useEnvironments(project?.id ?? null, {
    poll: false,
  });
  const environment = environments.find((item) =>
    item.serviceIds.includes(serviceId),
  );
  if (!project) return null;
  return (
    <span className="min-w-0 truncate tracking-normal normal-case lg:hidden">
      <span aria-hidden="true">· </span>
      {project.name}
      {environment ? ` / ${environment.name}` : null}
    </span>
  );
}

/**
 * A header fact that is a link. On a phone the rows stay close together and
 * the link's touch area reaches past its text instead (the truncation lives
 * on the inner span, since `overflow: hidden` would clip that area).
 */
function headerLinkClass(tone: string, reach: "up" | "down") {
  return cn(
    tone,
    "relative flex min-w-0 hover:underline max-sm:py-1.5 max-sm:after:absolute max-sm:after:inset-x-0 max-sm:after:content-['']",
    // From `sm` the 20px line takes a 24px target (WCAG 2.5.8).
    "sm:after:absolute sm:after:inset-x-0 sm:after:-inset-y-0.5 sm:after:content-['']",
    // The repository and URL rows touch, so one area reaches up and the
    // other down, and neither takes from the other.
    reach === "up"
      ? "max-sm:after:-top-3 max-sm:after:bottom-0"
      : "max-sm:after:top-0 max-sm:after:-bottom-3",
  );
}

/**
 * `host · owner / repo`. A phone drops the host, which the forge icon beside
 * it already says, so the repository's own name is what survives truncation.
 */
function RepoLabel({ repo }: { repo: string }) {
  const label = formatRepoLabel(repo);
  const split = label.indexOf(" · ");
  if (split < 0) return <>{label}</>;
  return (
    <>
      <span className="max-sm:hidden">{label.slice(0, split + 2)}</span>
      {label.slice(split + 2)}
    </>
  );
}

/** The Dockerfile instruction a container cron job runs when nothing
 *  overrides it: a keyword, not words. */
const DOCKER_CMD = "CMD";
const DEFAULT_DOCKERFILE = "Dockerfile";

/** What a cron job runs, in its facts line: laid out from no width, it grows
 *  to its own and truncates to fit, down to four characters, so the schedule's
 *  words beside it keep their line (w3/m555). */
const COMMAND_CLASS = "min-w-[4ch] max-w-max grow basis-0 truncate";

/**
 * A cron job's two fact lines, as Render's header has them (w3/m555): "Runs
 * <command> <schedule in words>", then "Last successful run <date>" or "No
 * successful runs yet.". They take the live URL's place; from `lg` they are
 * their own lines under the Service ID and the repository, as on Render.
 *
 * What runs is the command override, else a native runtime's start command,
 * else (a Dockerfile or image cron job) the image's own CMD, which Render
 * words "CMD in Dockerfile" for an image too. A schedule Render reads nothing
 * for reads nothing here either (w3/m643).
 */
function CronRunFacts({ service }: { service: ServiceView }) {
  const { t, i18n } = useTranslations();
  const words = service.schedule
    ? describeCronRun(service.schedule, i18n.language)
    : null;
  const command = service.command?.trim()
    ? service.command
    : isNativeRuntime(service.runtime) && service.startCommand?.trim()
      ? service.startCommand
      : null;
  const codeClass = "text-foreground font-mono text-xs";

  let what: ReactNode;
  if (command) {
    what = (
      <code title={command} className={cn(codeClass, COMMAND_CLASS)}>
        {command}
      </code>
    );
  } else {
    const dockerfile = service.dockerfilePath?.trim() || DEFAULT_DOCKERFILE;
    const parts = t("services.headerRunsDockerCmd", {
      cmd: `${SLOT}cmd${SLOT}`,
      dockerfile: `${SLOT}dockerfile${SLOT}`,
    }).split(SLOT);
    what = (
      <span className={COMMAND_CLASS}>
        {parts.map((part, i) =>
          i % 2 === 0 ? (
            part
          ) : (
            <code key={part} className={codeClass}>
              {part === "cmd" ? DOCKER_CMD : dockerfile}
            </code>
          ),
        )}
      </span>
    );
  }

  // Two items of the row, so on a phone "Details" sits beside the first and
  // the second takes the whole width; from `lg` each is its own line.
  return (
    <>
      {/* The spaces between the items are the sentence's, for whoever reads
          its text; the flex gap is what draws them. A long command truncates
          before the schedule's words wrap: they never shrink, and move to a
          line of their own only when even the truncated command leaves them
          no room (w3/m555). */}
      <p
        data-header-item="cron-runs"
        className="text-muted-foreground flex min-w-0 flex-wrap items-baseline gap-x-1.5 max-sm:self-center sm:basis-full"
      >
        <span className="shrink-0">{t("services.headerRuns")}</span> {what}{" "}
        {words ? <span className="max-w-full shrink-0">{words}</span> : null}
      </p>
      <p
        data-header-item="cron-last-run"
        className="text-muted-foreground min-w-0 max-sm:col-span-2 lg:basis-full"
      >
        {service.lastSuccessfulRunAt ? (
          <>
            {t("services.headerLastRun")}{" "}
            <LastRunDate value={service.lastSuccessfulRunAt} />
          </>
        ) : (
          t("services.headerNoSuccessfulRuns")
        )}
      </p>
    </>
  );
}

/**
 * The last successful run's date as Render's header has it
 * (`CronJobRunMetadata`, w3/m643): in the viewer's zone with the zone's name,
 * "November 21, 2026 at 9:00 AM PST". Off UTC the date is a term underlined
 * on hover whose tooltip reads the instant in UTC, "UTC: Nov 21 05:00:00 PM";
 * a viewer on UTC gets no tooltip. Both read the viewer's zone, so both wait
 * for hydration: until then the date is its placeholder, with no tooltip.
 */
function LastRunDate({ value }: { value: string }) {
  const { t, i18n } = useTranslations();
  const hydrated = useIsHydrated();
  const utc = hydrated ? formatInstantDetails(value, i18n.language)?.utc : null;
  if (!utc) {
    return <LocalDateTimeWithZone value={value} className="text-foreground" />;
  }
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <time
          dateTime={value}
          // eslint-disable-next-line jsx-a11y-x/no-noninteractive-tabindex -- the tooltip's trigger is focusable so keyboard users can reach its text
          tabIndex={0}
          className="text-foreground cursor-default rounded-sm underline-offset-2 hover:underline focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
        >
          <LocalDateTimeWithZone value={value} as="span" />
        </time>
      </TooltipTrigger>
      <TooltipContent>
        <time dateTime={value}>
          {t("services.headerLastRunUtc", { time: utc })}
        </time>
      </TooltipContent>
    </Tooltip>
  );
}

function RegistryCredentialFact({ id }: { id: string }) {
  const { t } = useTranslations();
  const { credentials } = useRegistryCredentials();
  const credential = credentials.find((item) => item.id === id);

  return (
    <div className="text-muted-foreground flex min-w-0 items-center gap-1.5">
      <KeyRound className="size-3.5 shrink-0" />
      <span>{t("services.headerRegistryCredential")}</span>
      <Link
        to="/workspace/settings"
        hash="registry-credentials"
        className="text-foreground truncate hover:underline"
      >
        {credential?.name || id}
      </Link>
    </div>
  );
}

function ServiceConnectButton({
  service,
  openPorts,
}: {
  service: ServiceView;
  openPorts: OpenPortsDisplay;
}) {
  const { t } = useTranslations();
  const command = service.sshAddress ? `ssh ${service.sshAddress}` : "";
  // A link inside the panel (a remedy, the add-key CTA, the browser terminal)
  // navigates without unmounting this header, so the panel stayed open over
  // the destination (w4/m183). It is a Popover, not a menu: its Copy buttons
  // and links are ordinary controls, so Tab walks them and Escape returns
  // focus to Connect (w4/512). A dropdown menu with no menu items kept focus
  // on its container.
  const [open, setOpen] = useState(false);

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button variant="outline" size="sm">
          {t("services.connect")}
          <ChevronDown className="size-3.5" />
        </Button>
      </PopoverTrigger>
      <PopoverContent
        align="end"
        aria-label={t("services.connect")}
        className="w-80 p-3"
        onClickCapture={(e) => {
          if ((e.target as Element).closest("a[href]")) setOpen(false);
        }}
      >
        {service.internalAddress ? (
          // Render's Connect → Internal tab: the private-network
          // `<slug>:<port>` sibling services dial (ADR041 D4, w9/m58), plus
          // an image-port service's other open ports (w6/m148).
          <>
            <div className="flex items-center gap-2 pb-1.5 text-sm font-medium">
              <Network className="size-4" />
              {t("services.connectInternal")}
            </div>
            <ConnectCodeRow
              className="mb-2"
              value={service.internalAddress}
              copyLabel={t("services.internalCopy")}
              copiedText={t("services.internalCopied")}
              errorText={t("services.internalCopyError")}
            />
            <OpenPortsFact className="mb-2" openPorts={openPorts} />
            {/* Render's Free plan does not support receiving private network
                traffic, and the platform drops tenant traffic to a free
                web service's pod (w8/m130). The address stays readable, as
                Render derives it from the slug. */}
            {service.url && autoSleepEligible(service.type, service.plan) ? (
              <p className="text-muted-foreground mb-2 text-xs">
                {t("services.connectInternalFreeNote")}
              </p>
            ) : null}
          </>
        ) : null}
        <div className="flex items-center gap-2 pb-1.5 text-sm font-medium">
          <Terminal className="size-4" />
          {t("services.connectSSH")}
        </div>
        {command ? (
          <ConnectSshCommand serviceId={service.id} command={command} />
        ) : (
          <SshUnavailable service={service} />
        )}
      </PopoverContent>
    </Popover>
  );
}

/**
 * The Connect menu's SSH command. Mounted only while the menu is open, it
 * asks the service's shell decision then (w4/726): a caller it refuses, such
 * as a Developer in a protected environment, reads why instead of a command
 * and a browser terminal the gateway would refuse.
 */
function ConnectSshCommand({
  serviceId,
  command,
}: {
  serviceId: string;
  command: string;
}) {
  const { t } = useTranslations();
  const shell = useShellGate(serviceId);
  if (shell.gate.kind === "denied") {
    return <p className="text-muted-foreground text-xs">{shell.reason}</p>;
  }
  return (
    // Gate the doomed `ssh …` command behind key registration (w2/m66).
    // With no key it fails off-surface; swap it for an add-key CTA — and,
    // since a paid running service has the in-browser Web Shell, offer that
    // as a second, zero-setup door (session-auth, needs no SSH key).
    <RequiresSshKey
      surface="service-ssh"
      fallback={
        <AddSshKeyCta
          surface="service-ssh"
          returnTo={`/services/${serviceId}`}
          secondaryAction={
            <Button asChild variant="outline" size="sm" className="w-full">
              <Link to="/services/$serviceId/shell" params={{ serviceId }}>
                <SquareTerminal className="size-4" />
                {t("services.openBrowserTerminal")}
              </Link>
            </Button>
          }
        />
      }
    >
      <ConnectCodeRow
        value={command}
        copyLabel={t("services.sshCopy")}
        copiedText={t("services.sshCopied")}
        errorText={t("services.sshCopyError")}
      />
    </RequiresSshKey>
  );
}

/**
 * The Connect menu's SSH section when the service has no SSH address: the
 * reason as visible text (it used to live in a hover-only `title`), and a link
 * to the remedy when the user holds one (w4/143).
 */
function SshUnavailable({ service }: { service: ServiceView }) {
  const { t } = useTranslations();
  const remedy = sshRemedy(service);
  return (
    <div className="space-y-1 text-xs">
      <p className="font-medium">{t("services.sshUnavailable")}</p>
      <p className="text-muted-foreground">{t(remedy.reason)}</p>
      {remedy.action ? (
        <Link
          to={
            remedy.action.path === "plan"
              ? "/services/$serviceId/compute"
              : "/services/$serviceId/settings"
          }
          params={{ serviceId: service.id }}
          hash={remedy.action.path === "settings" ? "suspend" : undefined}
          className="text-foreground inline-block underline underline-offset-2"
        >
          {t(remedy.action.label)}
        </Link>
      ) : null}
    </div>
  );
}

/**
 * A private service's Service Address fact: the copyable `<slug>:<port>`, and
 * for an image-port service its open ports beside it (wrapping under it on a
 * narrow screen). The row keeps the copy button's height in every state so it
 * lines up with the header skeleton's address row.
 */
function ServiceAddressFact({
  address,
  openPorts,
}: {
  address: string;
  openPorts: OpenPortsDisplay;
}) {
  const { t } = useTranslations();
  return (
    <div className="flex min-h-8 min-w-0 flex-wrap items-center gap-x-3 gap-y-0.5">
      <div className="flex min-w-0 items-center gap-1.5">
        <span className="text-muted-foreground">
          {t("services.headerServiceAddress")}
        </span>
        <span className="text-foreground truncate font-mono text-xs">
          {address}
        </span>
        <CopyButton
          value={address}
          label={t("services.internalCopy")}
          successText={t("services.internalCopied")}
          errorText={t("services.internalCopyError")}
        />
      </div>
      <OpenPortsFact openPorts={openPorts} />
    </div>
  );
}

/**
 * The open-ports line beside a Service Address: the port list; nothing for a
 * single-port service.
 */
function OpenPortsFact({
  openPorts,
  className,
}: {
  openPorts: OpenPortsDisplay;
  className?: string;
}) {
  if (openPorts.kind === "list") {
    return <OpenPortsList className={className} ports={openPorts.ports} />;
  }
  return null;
}

/**
 * The TCP ports a private service's hostname serves, dot-separated
 * (`8123 · 9000 · 9009`), the primary — the `<slug>:<port>` suffix — marked.
 */
function OpenPortsList({
  ports,
  className,
}: {
  ports: { port: number; primary: boolean }[];
  className?: string;
}) {
  const { t } = useTranslations();
  const labelId = useId();
  return (
    <div
      className={cn(
        "text-muted-foreground flex min-w-0 flex-wrap items-center gap-x-1.5 gap-y-0.5 text-xs",
        className,
      )}
    >
      <span id={labelId}>{t("services.openPorts")}</span>
      <ul
        aria-labelledby={labelId}
        className="text-foreground flex flex-wrap items-center gap-x-1.5 gap-y-0.5 font-mono tabular-nums"
      >
        {ports.map(({ port, primary }, i) => (
          <li key={port} className="flex items-center gap-1.5">
            {i > 0 ? (
              <span aria-hidden="true" className="text-muted-foreground">
                ·
              </span>
            ) : null}
            {port}
            {primary ? (
              <Badge variant="secondary" className="px-1.5 py-0 font-sans">
                {t("services.openPortsPrimary")}
              </Badge>
            ) : null}
          </li>
        ))}
      </ul>
    </div>
  );
}

/** One copyable code row in the Connect menu — shared by Internal and SSH. */
function ConnectCodeRow({
  value,
  copyLabel,
  copiedText,
  errorText,
  className,
}: {
  value: string;
  copyLabel: string;
  copiedText: string;
  errorText: string;
  className?: string;
}) {
  return (
    <div
      className={`bg-muted flex min-w-0 items-center gap-1 rounded-md py-1 pr-1 pl-2 ${className ?? ""}`}
    >
      <code className="min-w-0 flex-1 truncate text-xs" title={value}>
        {value}
      </code>
      <CopyButton
        value={value}
        label={copyLabel}
        successText={copiedText}
        errorText={errorText}
      />
    </div>
  );
}

/**
 * What runs now, as the instances API and Metrics count it: none before a
 * first release goes live, and none while suspended or asleep, with the count
 * it returns to (w4/547).
 */
function instancesFact(service: ServiceView, t: Translate): string {
  if (service.replicas == null) return "—";
  if (!hasServed(service)) return "0";
  if (runsInstances(service)) return String(service.replicas);
  const n = String(service.replicas);
  if (service.suspended) return t("services.instancesSuspended", { n });
  return isSleeping(service) ? t("services.instancesSleeping", { n }) : "0";
}

/**
 * The bex-native facts the retired Overview tab used to list — instances, active
 * revision, age — as one muted, dot-separated line under Render's metadata
 * stack. Phase and suspension aren't repeated: the status badge beside the name
 * already encodes both. A static_site has no runtime instances (it serves from
 * the object store, docs/ADR029-static-sites.md), so its line omits Instances
 * (w5/m48/t001 — Render shows no instance concept for static sites). Nor does
 * a cron job, which runs nothing between runs: its schedule and runs describe
 * it, and Render's cron job has no instance count (w4/606).
 */
function HeaderFacts({
  service,
  id,
  open,
}: {
  service: ServiceView;
  id: string;
  /** Below `lg` the list is folded until asked for. */
  open: boolean;
}) {
  const { t } = useTranslations();

  const facts: { label: string; value: ReactNode }[] = [
    // The runtime is a fact about the service, not a status: it reads here
    // with the others instead of as a third chip beside the name.
    ...(service.runtime?.trim()
      ? [
          {
            label: t("services.headerRuntime"),
            value: <span className="capitalize">{service.runtime.trim()}</span>,
          },
        ]
      : []),
    { label: t("services.colSlug"), value: service.slug || "—" },
    ...(service.immutableName && service.immutableName !== service.name
      ? [
          {
            label: t("services.colBlueprintName"),
            value: (
              <Tooltip>
                <TooltipTrigger asChild>
                  {/* eslint-disable-next-line jsx-a11y-x/no-noninteractive-tabindex -- the tooltip's trigger is focusable so keyboard users can reach its text */}
                  <code tabIndex={0} className="cursor-help">
                    {service.immutableName}
                  </code>
                </TooltipTrigger>
                <TooltipContent>
                  {t("services.blueprintNameHelp")}
                </TooltipContent>
              </Tooltip>
            ),
          },
        ]
      : []),
    ...(isStaticSite(service) || isCron(service)
      ? []
      : [
          {
            label: t("services.colInstances"),
            value: instancesFact(service, t),
          },
        ]),
    { label: t("services.colRevision"), value: service.revision || "—" },
    {
      label: t("services.colCreated"),
      value: <RelativeAge value={service.createdAt} />,
    },
  ];

  return (
    <dl
      id={id}
      className={cn(
        "text-muted-foreground flex-wrap items-center gap-x-4 gap-y-1 text-xs sm:max-xl:gap-x-3 lg:flex",
        // Opened on a phone: labels in one column, values in the next.
        open
          ? "max-sm:grid max-sm:grid-cols-[auto_minmax(0,1fr)] max-sm:gap-y-1.5 max-sm:pt-1 sm:flex"
          : "hidden",
      )}
    >
      {facts.map((fact) => (
        <div
          key={fact.label}
          className="flex items-center gap-1.5 max-sm:contents"
        >
          <dt>{fact.label}</dt>
          <dd className="text-foreground font-medium tabular-nums [&_code]:font-sans">
            {fact.value}
          </dd>
        </div>
      ))}
    </dl>
  );
}

/** The date the frame's last-run line is laid out with: the frame has none
 *  of its own, and any date reads about as wide. */
const SAMPLE_LAST_RUN = "2026-11-21T17:00:00Z";

/** Nothing cached says otherwise: a service built from a repository. */
const GIT_SOURCE: CachedServiceSource = {
  repository: true,
  registryCredential: false,
  cron: false,
  suspendedNotice: null,
};

/**
 * Placeholder header shown while `server(id)` is still loading. Each slot is
 * the ready element's height: the name is `text-xl` (28px), a badge 22px, and
 * a control (an action button, a copy button) 32px, growing to 44px where the
 * page's touch rule grows the ready one (`data-touch-spacer`, style.css), so
 * the band ends on the same line in both states (w3/085).
 *
 * The second row wraps, so it draws the source items the ready one will: an
 * image-backed service has no repository item, and at 1024 its Service ID and
 * URL fit on one line where a Git service's take two (w3/094). `source` comes
 * from the cache; without it the frame is a Git service's. Each item is laid
 * out in the ready one's words where they are known (the labels always, the
 * id, repository, branch and URL from the cache), so the row wraps where the
 * ready one does at every width: at 640 fixed bars fit the URL beside the
 * repository where the ready URL wraps under it (w3/112).
 *
 * A cron job's frame is its own (w3/m555): one action, Trigger Run, and its
 * two "Runs …" items in the URL's place. A suspended service of any type has
 * no action and its notice under the facts, its words laid out invisibly so
 * it wraps where the ready notice does (w3/107).
 */
export function ServiceDetailHeaderSkeleton({
  source,
  name,
}: {
  source?: CachedServiceSource | null;
  /** The cached name, laid out invisibly at the ready heading's size. */
  name?: string | null;
}) {
  const { t } = useTranslations();
  const {
    repository,
    registryCredential,
    cron,
    suspendedNotice,
    id,
    repo,
    branch,
    url,
  } = source ?? GIT_SOURCE;
  return (
    <div
      aria-hidden="true"
      className="border-b px-4 py-3 max-sm:pb-2 sm:px-6"
      data-skeleton-region="service-header"
    >
      <div className="mx-auto max-w-7xl space-y-2 max-sm:space-y-1.5">
        <Skeleton className="h-4 w-24" />
        <div className="flex flex-wrap items-center justify-between gap-3 md:max-lg:grid md:max-lg:grid-cols-[minmax(0,1fr)_auto] md:max-lg:gap-y-2 lg:flex-nowrap lg:items-start">
          <div className="flex min-w-0 flex-wrap items-center gap-2 md:max-lg:contents">
            {/* The ready row's items in their own words where the frame
                knows them, the name and the latest deploy's included (a
                service has one from its first deploy), so the row wraps
                where the ready one does (w3/m555). */}
            {name ? (
              <span className="relative min-w-0 truncate text-xl font-semibold">
                <ReservedText>{name}</ReservedText>
              </span>
            ) : (
              <Skeleton className="h-7 w-48 max-w-[50vw] max-sm:w-36" />
            )}
            <div className="contents sm:flex sm:min-w-0 sm:flex-wrap sm:items-center sm:gap-2 md:max-lg:col-span-2 md:max-lg:row-start-2">
              <span className="relative text-xs font-medium">
                <ReservedText>{t("services.headerServicePhase")}</ReservedText>
              </span>
              <Skeleton className="h-[22px] w-16 rounded-full" />
              <span
                data-skeleton-item="latest-deploy"
                className="flex items-center gap-1.5"
              >
                <span className="relative text-xs font-medium">
                  <ReservedText>
                    {t("services.headerLatestDeploy")}
                  </ReservedText>
                </span>
                <Skeleton className="h-[22px] w-10 rounded-full" />
              </span>
              <Skeleton className="h-[22px] w-14 rounded-full" />
            </div>
          </div>
          {/* The actions' own boxes, so the row wraps where the ready one
              does in every language (w3/112). */}
          <div className="flex items-center gap-2 empty:hidden md:max-lg:col-start-2 md:max-lg:row-start-1">
            {suspendedNotice ? null : cron ? (
              <ReservedButton
                variant="default"
                size="sm"
                data-touch-spacer=""
                data-skeleton-item="trigger-run"
              >
                {t("services.cronTriggerRun")}
              </ReservedButton>
            ) : (
              <>
                <ReservedButton
                  variant="outline"
                  size="sm"
                  data-touch-spacer=""
                  data-skeleton-item="connect"
                >
                  {t("services.connect")}
                  <ChevronDown className="size-3.5" />
                </ReservedButton>
                <ReservedButton
                  variant="default"
                  size="sm"
                  data-touch-spacer=""
                  data-skeleton-item="manual-deploy"
                >
                  {t("services.eventsManualDeploy")}
                  <ChevronDown className="size-3.5" />
                </ReservedButton>
              </>
            )}
          </div>
        </div>
        {/* Placed as the ready row is: on a phone every item stacks in the
            first column and "Details" takes the second beside the first of
            them, at its touch height. */}
        <div className="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-x-3 text-sm max-sm:pt-1.5 max-sm:[&>*]:col-start-1 sm:flex sm:flex-wrap sm:gap-x-6 sm:gap-y-0.5">
          <div
            data-skeleton-item="service-id"
            className="flex items-center gap-1.5 max-lg:hidden"
          >
            <span className="relative">
              <ReservedText>{t("services.headerServiceId")}</ReservedText>
            </span>
            {id ? (
              <span className="relative font-mono text-xs">
                <ReservedText>{id}</ReservedText>
              </span>
            ) : (
              <Skeleton className="h-4 w-44" />
            )}
            <Skeleton data-touch-spacer="" className="size-8" />
          </div>
          {repository && repo ? (
            <div
              data-skeleton-item="repository"
              className="flex min-w-0 items-center gap-1.5 max-sm:hidden"
            >
              <Skeleton className="size-3.5 shrink-0 rounded-full" />
              <span className="relative min-w-0 truncate">
                <ReservedText>
                  <RepoLabel repo={repo} />
                </ReservedText>
              </span>
              {branch ? (
                <ReservedBadge variant="secondary">
                  <span className="font-mono">{branch}</span>
                </ReservedBadge>
              ) : null}
            </div>
          ) : repository ? (
            <div
              data-skeleton-item="repository"
              className="flex h-[22px] items-center gap-1.5 max-sm:hidden"
            >
              <Skeleton className="h-4 w-48" />
              <Skeleton className="h-[22px] w-20 rounded-full" />
            </div>
          ) : null}
          {registryCredential ? (
            <div
              data-skeleton-item="registry-credential"
              className="flex h-5 items-center"
            >
              <Skeleton className="h-4 w-60" />
            </div>
          ) : null}
          {cron ? (
            <>
              <div
                data-skeleton-item="cron-runs"
                className="flex h-5 min-w-0 items-center sm:basis-full"
              >
                <Skeleton className="h-4 w-64 max-w-full" />
              </div>
              {/* The ready line's words, a sample date in the viewer's zone
                  included, laid out invisibly: with its zone the line wraps
                  at 390 in English and other languages, and the frame wraps
                  where it does (w3/m643). */}
              <p
                data-skeleton-item="cron-last-run"
                className="min-w-0 text-sm max-sm:col-span-2 lg:basis-full"
              >
                <span className="relative inline-block max-w-full">
                  <ReservedText>
                    {t("services.headerLastRun")}{" "}
                    <LocalDateTimeWithZone value={SAMPLE_LAST_RUN} />
                  </ReservedText>
                </span>
              </p>
            </>
          ) : (
            <div
              data-skeleton-item="service-url"
              className="flex min-w-0 items-center gap-1.5"
            >
              {url ? (
                <span className="relative min-w-0 truncate">
                  <ReservedText>{url}</ReservedText>
                </span>
              ) : (
                <Skeleton className="h-4 w-56" />
              )}
              <Skeleton data-touch-spacer="" className="size-8" />
            </div>
          )}
          <div
            data-skeleton-item="details-toggle"
            data-touch-spacer=""
            className="row-start-1 flex items-center max-sm:col-start-2! sm:ml-auto lg:hidden"
          >
            {/* The toggle's words and its chevron's room. */}
            <span className="relative flex items-center gap-1 text-xs font-medium">
              <ReservedText>{t("services.headerMoreFacts")}</ReservedText>
              <span className="size-3.5" />
            </span>
          </div>
        </div>
        <div className="hidden h-4 flex-wrap gap-x-4 gap-y-1 lg:flex">
          <Skeleton className="h-4 w-20" />
          <Skeleton className="h-4 w-24" />
          <Skeleton className="h-4 w-16" />
        </div>
        {suspendedNotice ? (
          <div
            data-skeleton-item="suspended-notice"
            className={SUSPENDED_NOTICE_CLASS}
          >
            <Skeleton className="size-4 shrink-0 rounded-full" />
            <span className="relative font-medium">
              <ReservedText>{t(suspendedNotice)}</ReservedText>
            </span>{" "}
            <span className="relative">
              <ReservedText>{t("services.suspendedNoticeBody")}</ReservedText>
            </span>{" "}
            <span
              data-touch-spacer=""
              className="relative inline-flex h-8 items-center"
            >
              <ReservedText>{t("services.suspendedNoticeResume")}</ReservedText>
            </span>
          </div>
        ) : null}
      </div>
    </div>
  );
}
