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
import { RelativeAge, RelativeUntil } from "@/common/components/relative-time";
import {
  useTranslations,
  type Translate,
} from "@/common/hooks/use-translations";
import { safeHttpHref } from "@/common/lib/external-url";
import { cn } from "@/common/lib/utils/utils";
import { useEnvironments } from "@/features/environments/hooks/use-environments";
import { useProjects } from "@/features/projects/hooks/use-projects";
import { AddSshKeyCta } from "@/features/ssh-keys/components/add-ssh-key-cta";
import { RequiresSshKey } from "@/features/ssh-keys/components/requires-ssh-key";
import { ResumeServiceButton } from "@/features/services/components/resume-service-button";
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
import { sshRemedy } from "@/features/services/lib/ssh-remedy";
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
 * dropdown (w2/m30) and Suspend/Resume moved to the Settings page. A
 * suspended service also gets Resume Service here, since every other action on
 * its pages is disabled until it resumes (w4/643).
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
                <Link
                  to="/services/$serviceId/compute"
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
          <div className="flex shrink-0 items-center gap-2 max-md:[&_button]:min-h-11 md:max-lg:col-start-2 md:max-lg:row-start-1">
            {/* A static_site has no running instance to SSH into — Render's
              static header carries only Manual Deploy (w5/m48/t001). */}
            {!isStaticSite(service) && (
              <ServiceConnectButton service={service} openPorts={openPorts} />
            )}
            {service.suspended && onRun ? (
              <ResumeServiceButton
                service={service}
                pending={pending}
                onRun={onRun}
                size="sm"
              />
            ) : null}
            <ManualDeployButton service={service} pending={pending !== null} />
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
            <>
              <div className="text-muted-foreground flex items-center gap-1.5">
                <span>{t("services.headerSchedule")}</span>
                <span className="text-foreground font-mono text-xs">
                  {service.schedule || "—"}
                </span>
              </div>
              {service.lastSuccessfulRunAt ? (
                <div className="text-muted-foreground flex items-center gap-1.5">
                  <span>{t("services.headerLastRun")}</span>
                  <RelativeAge
                    value={service.lastSuccessfulRunAt}
                    className="text-foreground text-xs"
                  />
                </div>
              ) : null}
              {service.nextRunAt ? (
                <div className="text-muted-foreground flex items-center gap-1.5">
                  <span>{t("services.headerNextRun")}</span>
                  <RelativeUntil
                    value={service.nextRunAt}
                    className="text-foreground text-xs"
                  />
                </div>
              ) : null}
            </>
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
          // Gate the doomed `ssh …` command behind key registration (w2/m66).
          // With no key it fails off-surface; swap it for an add-key CTA — and,
          // since a paid running service has the in-browser Web Shell, offer that
          // as a second, zero-setup door (session-auth, needs no SSH key).
          <RequiresSshKey
            surface="service-ssh"
            fallback={
              <AddSshKeyCta
                surface="service-ssh"
                returnTo={`/services/${service.id}`}
                secondaryAction={
                  <Button
                    asChild
                    variant="outline"
                    size="sm"
                    className="w-full"
                  >
                    <Link
                      to="/services/$serviceId/shell"
                      params={{ serviceId: service.id }}
                    >
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
        ) : (
          <SshUnavailable service={service} />
        )}
      </PopoverContent>
    </Popover>
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

/** Nothing cached says otherwise: a service built from a repository. */
const GIT_SOURCE: CachedServiceSource = {
  repository: true,
  registryCredential: false,
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
 * from the cache; without it the frame is a Git service's.
 */
export function ServiceDetailHeaderSkeleton({
  source,
}: {
  source?: CachedServiceSource | null;
}) {
  const { repository, registryCredential } = source ?? GIT_SOURCE;
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
            <Skeleton className="h-7 w-48 max-w-[50vw]" />
            <div className="contents sm:flex sm:items-center sm:gap-2 md:max-lg:col-span-2 md:max-lg:row-start-2">
              <Skeleton className="h-4 w-20" />
              <Skeleton className="h-[22px] w-16 rounded-full" />
              <Skeleton className="h-[22px] w-20 rounded-full" />
            </div>
          </div>
          <div className="flex items-center gap-2 md:max-lg:col-start-2 md:max-lg:row-start-1">
            <Skeleton data-touch-spacer="" className="h-8 w-24" />
            <Skeleton data-touch-spacer="" className="h-8 w-28" />
          </div>
        </div>
        {/* Placed as the ready row is: on a phone every item stacks in the
            first column and "Details" takes the second beside the first of
            them, at its touch height. */}
        <div className="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-x-3 max-sm:pt-1.5 max-sm:[&>*]:col-start-1 sm:flex sm:flex-wrap sm:gap-x-6 sm:gap-y-0.5">
          <div
            data-skeleton-item="service-id"
            className="flex items-center gap-1.5 max-lg:hidden"
          >
            <Skeleton className="h-4 w-64" />
            <Skeleton data-touch-spacer="" className="size-8" />
          </div>
          {repository ? (
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
          <div
            data-skeleton-item="service-url"
            className="flex items-center gap-1.5"
          >
            <Skeleton className="h-4 w-56" />
            <Skeleton data-touch-spacer="" className="size-8" />
          </div>
          <div
            data-skeleton-item="details-toggle"
            data-touch-spacer=""
            className="row-start-1 flex items-center max-sm:col-start-2! sm:ml-auto lg:hidden"
          >
            <Skeleton className="h-4 w-14" />
          </div>
        </div>
        <div className="hidden h-4 flex-wrap gap-x-4 gap-y-1 lg:flex">
          <Skeleton className="h-4 w-20" />
          <Skeleton className="h-4 w-24" />
          <Skeleton className="h-4 w-16" />
        </div>
      </div>
    </div>
  );
}
